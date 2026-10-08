package injector

import "testing"

type stubInjector struct {
	pastes int
	err    error
	closed bool
}

func (s *stubInjector) Paste() error {
	if s.err != nil {
		return s.err
	}
	s.pastes++
	return nil
}

func (s *stubInjector) Close() error { s.closed = true; return nil }

var _ Injector = (*stubInjector)(nil)

func TestStubPaste(t *testing.T) {
	s := &stubInjector{}
	if err := s.Paste(); err != nil {
		t.Fatal(err)
	}
	if s.pastes != 1 {
		t.Fatalf("pastes = %d, want 1", s.pastes)
	}
	if err := s.Close(); err != nil || !s.closed {
		t.Fatalf("Close err=%v closed=%v", err, s.closed)
	}
}
