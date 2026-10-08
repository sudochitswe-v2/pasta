package translator

import (
	"errors"
	"testing"
)

func TestBangExpandsToShiftSequence(t *testing.T) {
	ev, err := Translate("!")
	if err != nil {
		t.Fatal(err)
	}
	want := []KeyEvent{
		{KeyShift, true}, {Key1, true}, {Key1, false}, {KeyShift, false},
	}
	if len(ev) != len(want) {
		t.Fatalf("events = %v, want %v", ev, want)
	}
	for i := range want {
		if ev[i] != want[i] {
			t.Fatalf("events = %v, want %v", ev, want)
		}
	}
}

func TestPlainCharHasNoShift(t *testing.T) {
	ev, err := Translate("a1 ")
	if err != nil {
		t.Fatal(err)
	}
	want := []KeyEvent{
		{KeyA, true}, {KeyA, false},
		{Key1, true}, {Key1, false},
		{KeySpace, true}, {KeySpace, false},
	}
	if len(ev) != len(want) {
		t.Fatalf("events = %v, want %v", ev, want)
	}
	for i := range want {
		if ev[i] != want[i] {
			t.Fatalf("events = %v, want %v", ev, want)
		}
	}
}

func TestCaseAndControls(t *testing.T) {
	ev, err := Translate("A\n\t")
	if err != nil {
		t.Fatal(err)
	}
	want := []KeyEvent{
		{KeyShift, true}, {KeyA, true}, {KeyA, false}, {KeyShift, false},
		{KeyEnter, true}, {KeyEnter, false},
		{KeyTab, true}, {KeyTab, false},
	}
	if len(ev) != len(want) {
		t.Fatalf("events = %v, want %v", ev, want)
	}
	for i := range want {
		if ev[i] != want[i] {
			t.Fatalf("event %d = %v, want %v", i, ev[i], want[i])
		}
	}
}

func TestUnsupportedRuneFails(t *testing.T) {
	_, err := Translate("héllo")
	if err == nil {
		t.Fatal("expected error for é")
	}
	var uerr *UnsupportedError
	if !errors.As(err, &uerr) {
		t.Fatalf("err = %T(%v), want *UnsupportedError", err, err)
	}
	if uerr.Rune != 'é' || uerr.Index != 1 {
		t.Fatalf("uerr = %+v, want é at 1", uerr)
	}
}

func TestEmptyTranslatesToNothing(t *testing.T) {
	ev, err := Translate("")
	if err != nil || len(ev) != 0 {
		t.Fatalf("events = %v, err = %v", ev, err)
	}
}

func TestFullPrintableASCII(t *testing.T) {
	s := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`~-_=+[{]}\\|;:'\",<.>/?!@#$%^&*() \n\t"
	if _, err := Translate(s); err != nil {
		t.Fatalf("printable ASCII failed: %v", err)
	}
}
