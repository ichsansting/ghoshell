// Package vault is the crypto core (Seam 1): it seals a set of path-keyed files
// into a single blob and unseals them back byte-identical. A blob is a plaintext
// header (magic + Argon2id params + salt + nonce) followed by the
// XChaCha20-Poly1305 ciphertext of a tar of the files. The header is bound as
// AEAD additional data, so a wrong passphrase — or any tampering — fails loud
// rather than returning garbage or a partial extract.
package vault

import (
	"archive/tar"
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// File is one path-keyed entry in the vault. Mode carries POSIX perms (0600 for
// secrets); Secret marks credential files the launcher must place at 0600.
type File struct {
	Path   string
	Mode   fs.FileMode
	Secret bool
	Data   []byte
}

// secretPAX is the tar PAX record key that carries the Secret flag through the
// archive. Non-standard keys are namespaced by convention.
const secretPAX = "GHOSHELL.secret"

// magic tags the format and version so a future layout change fails loud on old
// blobs instead of misparsing them.
const magic = "GHOVLT1\x00"

// Argon2id cost used when sealing. Memory-hard by design: the sealed blob is
// offline-crackable, so these are the defense. They are written into each blob's
// header and read back at unseal time (not hardcoded there), so raising them
// later still decrypts older blobs at their original cost.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB → 64 MiB
	argonThreads = 4
	keyLen       = chacha20poly1305.KeySize
	saltLen      = 16
)

// Ceilings on the cost read from an untrusted blob's header. The KDF runs before
// AEAD authentication, so a corrupt or hostile header could otherwise drive
// Argon2id into a multi-hour / multi-GiB derive (a DoS) before Open ever fails.
// Generous enough to raise the seal-time cost later; tight enough to fail fast.
const (
	maxArgonTime   = 64
	maxArgonMemory = 2 * 1024 * 1024 // KiB → 2 GiB
)

// Header byte layout: magic ‖ time(u32) ‖ memory(u32) ‖ threads(u8) ‖ salt ‖ nonce.
const (
	offTime    = len(magic)
	offMemory  = offTime + 4
	offThreads = offMemory + 4
	offSalt    = offThreads + 1
	offNonce   = offSalt + saltLen
	headerLen  = offNonce + chacha20poly1305.NonceSizeX
)

// Seal packs files into a tar, then encrypts it under a key derived from
// passphrase. Salt and nonce are freshly random every call, so sealing the same
// files twice yields distinct blobs.
func Seal(passphrase string, files []File) ([]byte, error) {
	plaintext, err := tarFiles(files)
	if err != nil {
		return nil, err
	}

	salt := make([]byte, saltLen)
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("vault: read salt: %w", err)
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("vault: read nonce: %w", err)
	}

	header := buildHeader(salt, nonce)
	aead, err := newAEAD(passphrase, salt, argonTime, argonMemory, argonThreads)
	if err != nil {
		return nil, err
	}
	// Seal appends the ciphertext to header, with header as additional data so
	// the params are authenticated alongside the payload.
	return aead.Seal(header, nonce, plaintext, header), nil
}

// Unseal reverses Seal. Any failure — short blob, bad magic, wrong passphrase,
// tampered header — returns a nil slice and a non-nil error. Never partial.
func Unseal(passphrase string, blob []byte) ([]File, error) {
	if len(blob) < headerLen {
		return nil, fmt.Errorf("vault: blob too short (%d bytes)", len(blob))
	}
	header := blob[:headerLen]
	if string(header[:len(magic)]) != magic {
		return nil, fmt.Errorf("vault: bad magic — not a ghoshell vault")
	}
	time := binary.BigEndian.Uint32(header[offTime:offMemory])
	memory := binary.BigEndian.Uint32(header[offMemory:offThreads])
	threads := header[offThreads]
	if time == 0 || threads == 0 || time > maxArgonTime || memory > maxArgonMemory {
		return nil, fmt.Errorf("vault: header cost out of range (t=%d m=%d p=%d) — corrupt vault", time, memory, threads)
	}
	salt := header[offSalt:offNonce]
	nonce := header[offNonce:headerLen]
	ciphertext := blob[headerLen:]

	aead, err := newAEAD(passphrase, salt, time, memory, threads)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, header)
	if err != nil {
		return nil, fmt.Errorf("vault: unseal failed (wrong passphrase or corrupt vault): %w", err)
	}
	return untarFiles(plaintext)
}

// newAEAD derives the key with Argon2id at the given cost and builds the
// XChaCha20-Poly1305 cipher. Cost is passed (not read from constants) so Unseal
// can reproduce the exact cost recorded in the blob it is opening.
func newAEAD(passphrase string, salt []byte, time, memory uint32, threads uint8) (cipher.AEAD, error) {
	key := argon2.IDKey([]byte(passphrase), salt, time, memory, threads, keyLen)
	return chacha20poly1305.NewX(key)
}

// buildHeader lays out the fixed plaintext prefix. Params are stored so a reader
// need not know the cost that sealed a given blob.
func buildHeader(salt, nonce []byte) []byte {
	h := make([]byte, 0, headerLen)
	h = append(h, magic...)
	h = binary.BigEndian.AppendUint32(h, argonTime)
	h = binary.BigEndian.AppendUint32(h, argonMemory)
	h = append(h, argonThreads)
	h = append(h, salt...)
	h = append(h, nonce...)
	return h
}

// tarFiles serializes files to a tar, preserving perms and the secret flag.
func tarFiles(files []File) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range files {
		hdr := &tar.Header{
			Name: f.Path,
			Mode: int64(f.Mode.Perm()),
			Size: int64(len(f.Data)),
		}
		if f.Secret {
			hdr.PAXRecords = map[string]string{secretPAX: "1"}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("vault: tar header %q: %w", f.Path, err)
		}
		if _, err := tw.Write(f.Data); err != nil {
			return nil, fmt.Errorf("vault: tar body %q: %w", f.Path, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("vault: tar close: %w", err)
	}
	return buf.Bytes(), nil
}

// untarFiles reverses tarFiles.
func untarFiles(data []byte) ([]File, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	var files []File
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("vault: tar next: %w", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("vault: tar read %q: %w", hdr.Name, err)
		}
		files = append(files, File{
			Path:   hdr.Name,
			Mode:   fs.FileMode(hdr.Mode).Perm(),
			Secret: hdr.PAXRecords[secretPAX] == "1",
			Data:   body,
		})
	}
	return files, nil
}
