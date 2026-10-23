// Package updateauth is shared by the desktop updater and the root-owned package
// installer. Only the public verification key is embedded in release binaries.
package updateauth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"io"
	"strings"
)

//go:embed public-key.pem
var PublicKey []byte

func Verify(reader io.Reader, signature []byte) error {
	return VerifyWithKey(reader, signature, PublicKey)
}
func VerifyWithKey(reader io.Reader, signature, key []byte) error {
	if block, _ := pem.Decode(key); block != nil {
		key = block.Bytes
	}
	var public ed25519.PublicKey
	if len(key) == ed25519.PublicKeySize {
		public = ed25519.PublicKey(key)
	} else {
		parsed, err := x509.ParsePKIXPublicKey(key)
		if err != nil {
			return err
		}
		var ok bool
		public, ok = parsed.(ed25519.PublicKey)
		if !ok {
			return errors.New("Update public key must be Ed25519")
		}
	}
	hash := sha512.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return err
	}
	return ed25519.VerifyWithOptions(public, hash.Sum(nil), signature, &ed25519.Options{Hash: crypto.SHA512})
}
func DebMetadata(output, version, arch string) (string, error) {
	fields := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		if key, value, ok := strings.Cut(line, ": "); ok {
			fields[key] = strings.TrimSpace(value)
		}
	}
	fullVersion := fields["Version"]
	upstream := fullVersion
	if i := strings.LastIndex(upstream, "-"); i >= 0 {
		upstream = upstream[:i]
	}
	if fields["Package"] != "postika" || fullVersion == "" || version != "" && upstream != version || fields["Architecture"] != arch {
		return "", errors.New("Downloaded DEB does not match Postika, the update version, or this architecture")
	}
	return fullVersion, nil
}
