package updateauth

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"testing"
)

func TestVerifyRootInstallerPayload(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("signed deb bytes")
	digest := sha512.Sum512(payload)
	signature, err := private.Sign(rand.Reader, digest[:], &ed25519.Options{Hash: crypto.SHA512})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyWithKey(bytes.NewReader(payload), signature, public); err != nil {
		t.Fatal(err)
	}
	if err := VerifyWithKey(bytes.NewReader([]byte("tampered deb")), signature, public); err == nil {
		t.Fatal("modified package accepted")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if err := VerifyWithKey(bytes.NewReader(payload), signature, other); err == nil {
		t.Fatal("wrong signing key accepted")
	}
}
