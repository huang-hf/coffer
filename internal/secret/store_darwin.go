//go:build darwin

package secret

import "os"

func NewStore() (Store, error) {
	if os.Getenv("COFFER_USE_KEYCHAIN") == "true" {
		return NewKeychainStore("coffer"), nil
	}
	return newFileStore()
}
