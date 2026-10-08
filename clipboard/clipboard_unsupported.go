//go:build !linux && !windows

package clipboard

import "errors"

// New on unsupported platforms always fails; pasta targets Linux and Windows.
func New() (Clipboard, error) {
	return nil, errors.New("clipboard: unsupported platform")
}
