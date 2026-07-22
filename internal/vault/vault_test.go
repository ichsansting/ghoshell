package vault

import (
	"bytes"
	"encoding/binary"
	"io/fs"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

// sample returns a fresh set of path-keyed files spanning a normal config file
// and a 0600 secret — the two cases launch cares about.
func sample() []File {
	return []File{
		{Path: "config/fish/config.fish", Mode: 0o644, Data: []byte("set -gx EDITOR vim\n")},
		{Path: ".ssh/id_ed25519", Mode: 0o600, Secret: true, Data: []byte("-----KEY-----\n")},
		{Path: "empty", Mode: 0o644, Data: []byte{}},
	}
}

func TestRoundTrip(t *testing.T) {
	const pass = "correct horse battery staple"
	in := sample()

	blob, err := Seal(pass, in)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := Unseal(pass, blob)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("got %d files, want %d", len(out), len(in))
	}
	byPath := map[string]File{}
	for _, f := range out {
		byPath[f.Path] = f
	}
	for _, want := range in {
		got, ok := byPath[want.Path]
		if !ok {
			t.Errorf("missing file %q", want.Path)
			continue
		}
		if !bytes.Equal(got.Data, want.Data) {
			t.Errorf("%q data = %q, want %q", want.Path, got.Data, want.Data)
		}
		if got.Mode != want.Mode {
			t.Errorf("%q mode = %o, want %o", want.Path, got.Mode, want.Mode)
		}
		if got.Secret != want.Secret {
			t.Errorf("%q secret = %v, want %v", want.Path, got.Secret, want.Secret)
		}
	}
}

func TestWrongPassphraseFailsLoud(t *testing.T) {
	blob, err := Seal("right", sample())
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := Unseal("wrong", blob)
	if err == nil {
		t.Fatal("Unseal with wrong passphrase returned nil error — must fail loud")
	}
	if out != nil {
		t.Errorf("Unseal with wrong passphrase returned %d files — must be nil (no partial extract)", len(out))
	}
}

func TestReSealRotatesSaltAndNonce(t *testing.T) {
	in := sample()
	a, err := Seal("pass", in)
	if err != nil {
		t.Fatalf("Seal a: %v", err)
	}
	b, err := Seal("pass", in)
	if err != nil {
		t.Fatalf("Seal b: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two seals of identical plaintext produced identical blobs — salt/nonce not rotated")
	}
	// Both must still decrypt to the same plaintext.
	for _, blob := range [][]byte{a, b} {
		if _, err := Unseal("pass", blob); err != nil {
			t.Fatalf("Unseal after rotation: %v", err)
		}
	}
}

func TestTamperedHeaderFailsLoud(t *testing.T) {
	blob, err := Seal("pass", sample())
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	// Flip a byte inside the salt (still within the plaintext header, but not a
	// cost field, so it exercises AAD authentication rather than the cost-range
	// guard). The header is AEAD additional-data, so this must fail auth.
	tampered := append([]byte(nil), blob...)
	tampered[offSalt] ^= 0xff
	if _, err := Unseal("pass", tampered); err == nil {
		t.Fatal("tampered header authenticated — header must be bound as AEAD additional data")
	}
}

// TestUnsealRejectsAbsurdCost guards the trust boundary: the KDF runs on
// header-supplied cost before AEAD auth, so a corrupt/hostile header claiming a
// huge time or memory must be rejected fast, not fed to Argon2id (a DoS).
func TestUnsealRejectsAbsurdCost(t *testing.T) {
	blob, err := Seal("pass", sample())
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	tampered := append([]byte(nil), blob...)
	// Max out the time field (offTime, big-endian u32) → would be ~4.3e9 passes.
	tampered[offTime] = 0xff
	tampered[offTime+1] = 0xff
	tampered[offTime+2] = 0xff
	tampered[offTime+3] = 0xff
	if _, err := Unseal("pass", tampered); err == nil {
		t.Fatal("absurd Argon2id cost accepted — must fail fast before the KDF runs")
	}
}

func TestUnsealRejectsShortBlob(t *testing.T) {
	if _, err := Unseal("pass", []byte("nope")); err == nil {
		t.Fatal("short blob accepted — want a loud length/format error")
	}
}

// TestUnsealHonorsHeaderParams locks the invariant that Unseal derives the key
// from the cost recorded in the blob, not from the package constants. A blob
// built at a cost that differs from argonTime/Memory/Threads must still open —
// otherwise raising the seal-time cost would orphan every older blob.
func TestUnsealHonorsHeaderParams(t *testing.T) {
	const pass = "p"
	plaintext, err := tarFiles(sample())
	if err != nil {
		t.Fatalf("tarFiles: %v", err)
	}
	salt := make([]byte, saltLen)
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	salt[0], nonce[0] = 1, 2 // deterministic, distinct from any real seal

	// Cheaper-than-default cost, deliberately != argonTime/Memory/Threads.
	const t2, m2, th2 = uint32(1), uint32(8 * 1024), uint8(1)
	header := make([]byte, 0, headerLen)
	header = append(header, magic...)
	header = binary.BigEndian.AppendUint32(header, t2)
	header = binary.BigEndian.AppendUint32(header, m2)
	header = append(header, th2)
	header = append(header, salt...)
	header = append(header, nonce...)

	aead, err := newAEAD(pass, salt, t2, m2, th2)
	if err != nil {
		t.Fatalf("newAEAD: %v", err)
	}
	blob := aead.Seal(header, nonce, plaintext, header)

	out, err := Unseal(pass, blob)
	if err != nil {
		t.Fatalf("Unseal of non-default-cost blob failed — header params not honored: %v", err)
	}
	if len(out) != len(sample()) {
		t.Fatalf("got %d files, want %d", len(out), len(sample()))
	}
}

// Guard the invariant the launch layer relies on: a secret file keeps 0600 and
// its secret flag across the round-trip regardless of directory nesting.
func TestSecretPermsPreserved(t *testing.T) {
	in := []File{{Path: "a/b/c/token", Mode: fs.FileMode(0o600), Secret: true, Data: []byte("s")}}
	blob, err := Seal("p", in)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := Unseal("p", blob)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if len(out) != 1 || out[0].Mode != 0o600 || !out[0].Secret {
		t.Fatalf("got %+v, want single 0600 secret file", out)
	}
}
