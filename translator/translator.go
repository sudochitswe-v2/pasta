// Package translator maps text to raw hardware key events for "Stealth
// Type" injection.
//
// The output is platform-independent: each rune becomes a key press on an
// abstract US-layout key, plus a Shift modifier when needed (e.g. '!' is
// Shift down, '1' down/up, Shift up). Platform injectors map KeyCode to
// evdev codes (Linux) or virtual-key codes (Windows) and pace events with
// micro-sleeps so polling-based secure apps don't drop input.
package translator

import (
	"fmt"
	"time"
)

// DefaultKeyDelay is the pause between raw key events. Polling-based secure
// apps and anti-cheat filters sample input state; without a 1–5ms gap,
// press/release pairs can land in the same poll window and get dropped.
const DefaultKeyDelay = 2 * time.Millisecond

// KeyCode identifies a physical key on a US-layout keyboard. Values are
// abstract; each platform injector maps them to native codes.
type KeyCode uint8

const (
	KeyUnknown KeyCode = iota
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	Key0
	KeyMinus
	KeyEqual
	KeyLeftBrace
	KeyRightBrace
	KeyBackslash
	KeySemicolon
	KeyApostrophe
	KeyGrave
	KeyComma
	KeyDot
	KeySlash
	KeySpace
	KeyEnter
	KeyTab
	KeyShift
)

// KeyEvent is a single raw make/break code.
type KeyEvent struct {
	Key  KeyCode
	Down bool
}

// keyDef is the (key, needs-shift) mapping for one rune.
type keyDef struct {
	key   KeyCode
	shift bool
}

// usLayout maps runes to US-layout keys. Letters map case-insensitively;
// uppercase implies Shift.
var usLayout = map[rune]keyDef{
	' ':  {KeySpace, false},
	'\n': {KeyEnter, false},
	'\r': {KeyEnter, false},
	'\t': {KeyTab, false},

	'`': {KeyGrave, false}, '~': {KeyGrave, true},
	'-': {KeyMinus, false}, '_': {KeyMinus, true},
	'=': {KeyEqual, false}, '+': {KeyEqual, true},
	'[': {KeyLeftBrace, false}, '{': {KeyLeftBrace, true},
	']': {KeyRightBrace, false}, '}': {KeyRightBrace, true},
	'\\': {KeyBackslash, false}, '|': {KeyBackslash, true},
	';': {KeySemicolon, false}, ':': {KeySemicolon, true},
	'\'': {KeyApostrophe, false}, '"': {KeyApostrophe, true},
	',': {KeyComma, false}, '<': {KeyComma, true},
	'.': {KeyDot, false}, '>': {KeyDot, true},
	'/': {KeySlash, false}, '?': {KeySlash, true},

	'1': {Key1, false}, '!': {Key1, true},
	'2': {Key2, false}, '@': {Key2, true},
	'3': {Key3, false}, '#': {Key3, true},
	'4': {Key4, false}, '$': {Key4, true},
	'5': {Key5, false}, '%': {Key5, true},
	'6': {Key6, false}, '^': {Key6, true},
	'7': {Key7, false}, '&': {Key7, true},
	'8': {Key8, false}, '*': {Key8, true},
	'9': {Key9, false}, '(': {Key9, true},
	'0': {Key0, false}, ')': {Key0, true},
}

func init() {
	for r := 'a'; r <= 'z'; r++ {
		usLayout[r] = keyDef{KeyA + KeyCode(r-'a'), false}
		usLayout[r-('a'-'A')] = keyDef{KeyA + KeyCode(r-'a'), true}
	}
}

// UnsupportedError reports the first rune with no key mapping. Typing fails
// loudly instead of silently dropping characters (critical for passwords).
type UnsupportedError struct {
	Rune  rune
	Index int
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("translator: unsupported character %q (U+%04X) at position %d", e.Rune, e.Rune, e.Index)
}

// Translate converts text into raw make/break events. Shifted characters
// expand to Shift down → key down → key up → Shift up.
func Translate(text string) ([]KeyEvent, error) {
	events := make([]KeyEvent, 0, len(text)*2)
	for i, r := range text {
		def, ok := usLayout[r]
		if !ok {
			return nil, &UnsupportedError{Rune: r, Index: i}
		}
		if def.shift {
			events = append(events, KeyEvent{KeyShift, true})
		}
		events = append(events, KeyEvent{def.key, true}, KeyEvent{def.key, false})
		if def.shift {
			events = append(events, KeyEvent{KeyShift, false})
		}
	}
	return events, nil
}

// EventCount returns how many raw events text will produce (for progress
// reporting). Returns the translation error unchanged on failure.
func EventCount(text string) (int, error) {
	ev, err := Translate(text)
	if err != nil {
		return 0, err
	}
	return len(ev), nil
}
