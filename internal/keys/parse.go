package keys

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// shiftedDigits maps shifted number runes to their unshifted counterparts.
var shiftedDigits = map[rune]rune{
	'!': '1',
	'@': '2',
	'#': '3',
	'$': '4',
	'%': '5',
	'^': '6',
	'&': '7',
	'*': '8',
	'(': '9',
	')': '0',
}

// Parse converts a key specification like "Ctrl+A" or "F5" to tcell values.
// It returns the key, optional rune, and modifier mask.
func Parse(spec string) (tcell.Key, rune, tcell.ModMask, error) {
	if spec == "" {
		return 0, 0, 0, fmt.Errorf("empty key specification")
	}

	parts := strings.Split(spec, "+")
	base := strings.TrimSpace(parts[len(parts)-1])
	var mods tcell.ModMask
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "ctrl", "control":
			mods |= tcell.ModCtrl
		case "alt", "opt":
			mods |= tcell.ModAlt
		case "shift":
			mods |= tcell.ModShift
		case "meta", "win", "windows", "cmd", "super":
			mods |= tcell.ModMeta
		case "":
			// ignore empty segment like "Ctrl+"
		default:
			return 0, 0, 0, fmt.Errorf("unknown modifier %q", p)
		}
	}

	b := strings.ToUpper(base)
	switch b {
	case "TAB":
		// "Ctrl+Tab" is not a standard combination that all terminals support
		// sending, but we can support the string configuration.
		return tcell.KeyTab, 0, mods, nil
	case "ENTER", "RETURN":
		return tcell.KeyEnter, 0, mods, nil
	case "ESC", "ESCAPE":
		return tcell.KeyEsc, 0, mods, nil
	case "UP":
		return tcell.KeyUp, 0, mods, nil
	case "DOWN":
		return tcell.KeyDown, 0, mods, nil
	case "LEFT":
		return tcell.KeyLeft, 0, mods, nil
	case "RIGHT":
		return tcell.KeyRight, 0, mods, nil
	}

	if strings.HasPrefix(b, "F") {
		if n, err := strconv.Atoi(strings.TrimPrefix(b, "F")); err == nil {
			switch n {
			case 1:
				return tcell.KeyF1, 0, mods, nil
			case 2:
				return tcell.KeyF2, 0, mods, nil
			case 3:
				return tcell.KeyF3, 0, mods, nil
			case 4:
				return tcell.KeyF4, 0, mods, nil
			case 5:
				return tcell.KeyF5, 0, mods, nil
			case 6:
				return tcell.KeyF6, 0, mods, nil
			case 7:
				return tcell.KeyF7, 0, mods, nil
			case 8:
				return tcell.KeyF8, 0, mods, nil
			case 9:
				return tcell.KeyF9, 0, mods, nil
			case 10:
				return tcell.KeyF10, 0, mods, nil
			case 11:
				return tcell.KeyF11, 0, mods, nil
			case 12:
				return tcell.KeyF12, 0, mods, nil
			}
		}
	}

	if len([]rune(base)) == 1 {
		r := []rune(base)[0]
		if u, ok := shiftedDigits[r]; ok {
			r = u
			mods |= tcell.ModShift
		}
		if unicode.IsUpper(r) {
			mods |= tcell.ModShift
		}
		r = unicode.ToLower(r)
		return tcell.KeyRune, r, mods, nil
	}

	return 0, 0, 0, fmt.Errorf("unknown key %q", base)
}

// Validate returns an error if the key specification is not recognized.
func Validate(spec string) error {
	_, _, _, err := Parse(spec)
	return err
}

// CanonicalID returns a unique identifier for a parsed key combination.
func CanonicalID(key tcell.Key, r rune, mod tcell.ModMask) string {
	if key == tcell.KeyRune {
		r = unicode.ToLower(r)
	}
	return fmt.Sprintf("%d:%d:%d", key, r, mod)
}

// IsReserved reports whether the given key combination is reserved for
// navigation and should not be reassigned. Only unmodified keys are checked.
func IsReserved(key tcell.Key, r rune, mod tcell.ModMask) bool {
	// Unmodified navigation keys cannot be remapped.
	if mod == 0 {
		switch key {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
			tcell.KeyEsc, tcell.KeyEnter, tcell.KeyBackspace, tcell.KeyBackspace2,
			tcell.KeyTab:
			return true
		case tcell.KeyRune:
			switch unicode.ToLower(r) {
			case 'h', 'j', 'k', 'l':
				return true
			}
		}
	}

	// System-reserved combinations like Ctrl+C should not be reused.
	if key == tcell.KeyRune && (mod&tcell.ModCtrl) != 0 {
		other := mod &^ (tcell.ModCtrl | tcell.ModShift)
		if other == 0 {
			switch unicode.ToLower(r) {
			case 'c', 'd':
				return true
			}
		}
	}
	return false
}

// NormalizeEvent converts an EventKey into a canonical (key,rune,mod) triple.
// Ctrl+A style events are normalized to KeyRune with the corresponding rune.
func NormalizeEvent(ev *tcell.EventKey) (tcell.Key, rune, tcell.ModMask) {
	key := ev.Key()
	r := ev.Rune()
	mod := ev.Modifiers()

	if key == tcell.KeyCtrlI {
		key = tcell.KeyTab
	}

	// If a rune is uppercase, the shift modifier should be active.
	isRuneKey := key == tcell.KeyRune
	if isRuneKey {
		if unicode.IsUpper(r) {
			mod |= tcell.ModShift
		}
		if _, ok := shiftedDigits[r]; ok {
			mod |= tcell.ModShift
		}
	}

	// Normalize Ctrl+<char> keys to a rune and ModCtrl. This makes bindings
	// more consistent, as some terminals send KeyCtrlX and others send a rune
	// with a Ctrl modifier. We must preserve existing modifiers.
	ctrlRune, isCtrlKey := ToChar(key)
	if isCtrlKey {
		key = tcell.KeyRune
		r = ctrlRune
		mod |= tcell.ModCtrl
	}

	if key == tcell.KeyRune {
		if u, ok := shiftedDigits[r]; ok {
			r = u
		}
		r = unicode.ToLower(r)
	}
	return key, r, mod
}

// ToChar converts a tcell.Key to its rune representation if it's a Ctrl+<char> key.
func ToChar(k tcell.Key) (rune, bool) {
	switch k {
	// These have special key codes and should not be treated as runes.
	case tcell.KeyCtrlH: // Backspace
		return 0, false
	case tcell.KeyCtrlI: // Tab
		return 0, false
	case tcell.KeyCtrlM: // Enter
		return 0, false
	}
	if k >= tcell.KeyCtrlA && k <= tcell.KeyCtrlZ {
		return 'a' + rune(k-tcell.KeyCtrlA), true
	}
	// Handle other special Ctrl keys if necessary
	switch k {
	case tcell.KeyCtrlSpace:
		return ' ', true
	case tcell.KeyCtrlUnderscore:
		return '_', true
	case tcell.KeyCtrlRightSq:
		return ']', true
	case tcell.KeyCtrlBackslash:
		return '\\', true
	case tcell.KeyCtrlCarat:
		return '^', true
	}
	return 0, false
}
