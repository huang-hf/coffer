//go:build !darwin && !linux && !windows

package secret

func NewStore() (Store, error) {
	return newFileStore()
}
