//go:build !linux && !windows

package screenshot

type stubCapturer struct{}

func newCapturer() (Capturer, error) {
	return nil, ErrUnavailable
}

func (c *stubCapturer) CapturePNG() ([]byte, error) {
	return nil, ErrUnavailable
}
