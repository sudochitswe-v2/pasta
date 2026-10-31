package screenshot

import (
	"errors"
	"testing"
)

type mockCapturer struct {
	pngData []byte
	err     error
}

func (m *mockCapturer) CapturePNG() ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.pngData, nil
}

func TestMockCapturer(t *testing.T) {
	mockData := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	m := &mockCapturer{pngData: mockData}

	data, err := m.CapturePNG()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != len(mockData) {
		t.Fatalf("got %d bytes, want %d", len(data), len(mockData))
	}

	wantErr := errors.New("capture failure")
	mFail := &mockCapturer{err: wantErr}
	_, err = mFail.CapturePNG()
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}
}

func TestNewCapturer(t *testing.T) {
	// Should return a capturer or ErrUnavailable depending on the environment.
	c, err := New()
	if err != nil && !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unexpected error type: %v", err)
	}
	if err == nil && c == nil {
		t.Fatal("expected non-nil capturer when err is nil")
	}
}
