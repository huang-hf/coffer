package secret

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
	"filippo.io/age/armor"
)

var ErrAgeKeyAlreadyExists = errors.New("age key already exists")

const (
	ageKeyName = "key"
)

// EnsureAgeKey checks if the age key exists at storeDir; if not, generates one.
// Returns ErrAgeKeyAlreadyExists if the key already exists.
func EnsureAgeKey(storeDir string) error {
	path := AgeKeyPath(storeDir)
	if _, err := os.Stat(path); err == nil {
		return ErrAgeKeyAlreadyExists
	}
	return GenerateAgeKey(path)
}

// AgeKeyPath returns the path to the age private key file.
func AgeKeyPath(storeDir string) string {
	return filepath.Join(storeDir, ageKeyName)
}

// GenerateAgeKey generates a new age X25519 identity and writes it to path (0600).
func GenerateAgeKey(path string) error {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("generating age key: %w", err)
	}

	encoded := identity.String()

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating key directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("writing age key: %w", err)
	}

	return nil
}

// LoadAgeKey reads an age private key from path and returns the identity.
func LoadAgeKey(path string) (*age.X25519Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading age key: %w", err)
	}

	identity, err := age.ParseX25519Identity(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing age key: %w", err)
	}

	return identity, nil
}

// ageEncrypt encrypts plaintext with the given identity's recipient (public key).
func ageEncrypt(plaintext []byte, identity *age.X25519Identity) ([]byte, error) {
	recipient := identity.Recipient()

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("age encrypt: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("age encrypt write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("age encrypt close: %w", err)
	}

	return buf.Bytes(), nil
}

// ageEncryptArmored encrypts plaintext with ASCII-armored output.
func ageEncryptArmored(plaintext []byte, identity *age.X25519Identity) ([]byte, error) {
	recipient := identity.Recipient()

	var buf bytes.Buffer
	a := armor.NewWriter(&buf)
	w, err := age.Encrypt(a, recipient)
	if err != nil {
		return nil, fmt.Errorf("age encrypt (armored): %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("age encrypt write (armored): %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("age encrypt close (armored): %w", err)
	}
	if err := a.Close(); err != nil {
		return nil, fmt.Errorf("age armor close: %w", err)
	}

	return buf.Bytes(), nil
}

// ageDecrypt decrypts data using the identity (private key).
// It tries binary decryption first, then armored fallback.
func ageDecrypt(ciphertext []byte, identity *age.X25519Identity) ([]byte, error) {
	out, err := ageDecryptRaw(ciphertext, identity)
	if err == nil {
		return out, nil
	}

	// Try armored format
	return ageDecryptRaw(ciphertext, identity, armor.NewReader)
}

type readerOpt func(io.Reader) io.Reader

func withArmor(r io.Reader) io.Reader {
	return armor.NewReader(r)
}

func ageDecryptRaw(data []byte, identity *age.X25519Identity, opts ...readerOpt) ([]byte, error) {
	r := io.Reader(bytes.NewReader(data))
	for _, opt := range opts {
		r = opt(r)
	}

	d, err := age.Decrypt(r, identity)
	if err != nil {
		return nil, fmt.Errorf("age decrypt: %w", err)
	}

	out, err := io.ReadAll(d)
	if err != nil {
		return nil, fmt.Errorf("age decrypt read: %w", err)
	}

	return out, nil
}
