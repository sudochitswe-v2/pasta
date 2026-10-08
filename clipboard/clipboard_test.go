package clipboard

import "testing"

// Compile-time interface compliance for test doubles.
type stubClipboard struct {
	texts []string
	err   error
}

func (s *stubClipboard) SetText(text string) error {
	if s.err != nil {
		return s.err
	}
	s.texts = append(s.texts, text)
	return nil
}

var _ Clipboard = (*stubClipboard)(nil)

func TestMaxTextBytes(t *testing.T) {
	if MaxTextBytes != 64*1024 {
		t.Fatalf("MaxTextBytes = %d, want 65536", MaxTextBytes)
	}
}

func TestStubSetText(t *testing.T) {
	s := &stubClipboard{}
	if err := s.SetText("hello"); err != nil {
		t.Fatal(err)
	}
	if len(s.texts) != 1 || s.texts[0] != "hello" {
		t.Fatalf("texts = %q", s.texts)
	}
}
