package keys

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Parse converts a key specification like "Ctrl+A" or "F5" to tcell values.
// It returns the key, optional rune, and modifier mask.
func Parse(spec string) (tcell.Key, rune, tcell.ModMask, error) {
	if spec == "" {
		return 0, 0, 0, fmt.Errorf("empty key specification")
	}

	parts := strings.Split(spec, "+")
	base := strings.TrimSpace(parts[len(parts)-1])
	var mods tcell.ModMask
	shiftUsed := false
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "ctrl", "control":
			mods |= tcell.ModCtrl
		case "alt":
			mods |= tcell.ModAlt
		case "shift":
			mods |= tcell.ModShift
			shiftUsed = true
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
		r = unicode.ToLower(r)
		if shiftUsed {
			// Shift cannot be reliably detected for letters in
			// terminals. Normalize by removing the Shift modifier
			// and matching case-insensitively.
			mods &^= tcell.ModShift
		}
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
			case 'h', 'j', 'k', 'l', 'q':
				return true
			}
		}
	}

	// System-reserved combinations like Ctrl+C should not be reused.
	if mod == tcell.ModCtrl && key == tcell.KeyRune {
		switch unicode.ToLower(r) {
		case 'c', 'd', 'z':
			return true
		}
	}
	return false
}

// CtrlKeyForRune maps a letter rune to its corresponding tcell KeyCtrlX value.
// For runes outside a-z, tcell.KeyRune is returned.
func CtrlKeyForRune(r rune) tcell.Key {
	switch unicode.ToLower(r) {
	case 'a':
		return tcell.KeyCtrlA
	case 'b':
		return tcell.KeyCtrlB
	case 'c':
		return tcell.KeyCtrlC
	case 'd':
		return tcell.KeyCtrlD
	case 'e':
		return tcell.KeyCtrlE
	case 'f':
		return tcell.KeyCtrlF
	case 'g':
		return tcell.KeyCtrlG
	case 'h':
		return tcell.KeyCtrlH
	case 'i':
		return tcell.KeyCtrlI
	case 'j':
		return tcell.KeyCtrlJ
	case 'k':
		return tcell.KeyCtrlK
	case 'l':
		return tcell.KeyCtrlL
	case 'm':
		return tcell.KeyCtrlM
	case 'n':
		return tcell.KeyCtrlN
	case 'o':
		return tcell.KeyCtrlO
	case 'p':
		return tcell.KeyCtrlP
	case 'q':
		return tcell.KeyCtrlQ
	case 'r':
		return tcell.KeyCtrlR
	case 's':
		return tcell.KeyCtrlS
	case 't':
		return tcell.KeyCtrlT
	case 'u':
		return tcell.KeyCtrlU
	case 'v':
		return tcell.KeyCtrlV
	case 'w':
		return tcell.KeyCtrlW
	case 'x':
		return tcell.KeyCtrlX
	case 'y':
		return tcell.KeyCtrlY
	case 'z':
		return tcell.KeyCtrlZ
	}
	return tcell.KeyRune
}

// NormalizeEvent converts an EventKey into a canonical (key,rune,mod) triple.
// Ctrl+A style events are normalized to KeyRune with the corresponding rune.
func NormalizeEvent(ev *tcell.EventKey) (tcell.Key, rune, tcell.ModMask) {
	key := ev.Key()
	r := ev.Rune()
	mod := ev.Modifiers()

	switch key {
	case tcell.KeyCtrlA:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'a'
		}
	case tcell.KeyCtrlB:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'b'
		}
	case tcell.KeyCtrlC:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'c'
		}
	case tcell.KeyCtrlD:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'd'
		}
	case tcell.KeyCtrlE:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'e'
		}
	case tcell.KeyCtrlF:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'f'
		}
	case tcell.KeyCtrlG:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'g'
		}
	case tcell.KeyCtrlH:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'h'
		}
	case tcell.KeyCtrlI:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'i'
		} else {
			key = tcell.KeyTab
		}
	case tcell.KeyCtrlJ, tcell.KeyCtrlM:
		if mod&tcell.ModCtrl != 0 {
			key = tcell.KeyRune
			if ev.Key() == tcell.KeyCtrlM {
				r = 'm'
			} else {
				r = 'j'
			}
		} else {
			key = tcell.KeyEnter
		}
	case tcell.KeyCtrlK:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'k'
		}
	case tcell.KeyCtrlL:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'l'
		}
	case tcell.KeyCtrlN:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'n'
		}
	case tcell.KeyCtrlO:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'o'
		}
	case tcell.KeyCtrlP:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'p'
		}
	case tcell.KeyCtrlQ:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'q'
		}
	case tcell.KeyCtrlR:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'r'
		}
	case tcell.KeyCtrlS:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 's'
		}
	case tcell.KeyCtrlT:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 't'
		}
	case tcell.KeyCtrlU:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'u'
		}
	case tcell.KeyCtrlV:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'v'
		}
	case tcell.KeyCtrlW:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'w'
		}
	case tcell.KeyCtrlX:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'x'
		}
	case tcell.KeyCtrlY:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'y'
		}
	case tcell.KeyCtrlZ:
		if mod&tcell.ModCtrl != 0 {
			key, r = tcell.KeyRune, 'z'
		}
	}
	return key, r, mod
}
