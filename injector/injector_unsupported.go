//go:build !linux && !windows

package injector

import "errors"

type unsupportedInjector struct{}

// New on unsupported platforms always fails; pasta targets Linux and Windows.
func New() (Injector, error) {
	return nil, errors.New("injector: unsupported platform")
}

func (u *unsupportedInjector) Paste() error { return ErrUnavailable }

// Type is unavailable off Linux/Windows.
func (u *unsupportedInjector) Type(_ string) error { return ErrUnavailable }
func (u *unsupportedInjector) Close() error        { return nil }
