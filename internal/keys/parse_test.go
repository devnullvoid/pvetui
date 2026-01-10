package keys

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	cases := []struct {
		spec string
		key  tcell.Key
		r    rune
		mod  tcell.ModMask
	}{
		{"A", tcell.KeyRune, 'a', tcell.ModShift},
		{"Shift+A", tcell.KeyRune, 'a', tcell.ModShift},
		{"Ctrl+A", tcell.KeyRune, 'a', tcell.ModCtrl | tcell.ModShift},
		{"Ctrl+Shift+A", tcell.KeyRune, 'a', tcell.ModCtrl | tcell.ModShift},
		{"Backtab", tcell.KeyBacktab, 0, 0},
		{"Shift+Tab", tcell.KeyBacktab, 0, 0},
		{"Alt+1", tcell.KeyRune, '1', tcell.ModAlt},
		{"Opt+1", tcell.KeyRune, '1', tcell.ModAlt},
		{"Alt+#", tcell.KeyRune, '3', tcell.ModAlt | tcell.ModShift},
		{"Opt+#", tcell.KeyRune, '3', tcell.ModAlt | tcell.ModShift},
		{"Alt+Shift+3", tcell.KeyRune, '3', tcell.ModAlt | tcell.ModShift},
		{"Opt+Shift+3", tcell.KeyRune, '3', tcell.ModAlt | tcell.ModShift},
		{"Win+A", tcell.KeyRune, 'a', tcell.ModMeta | tcell.ModShift},
		{"Shift+F1", tcell.KeyF1, 0, tcell.ModShift},
		{"Shift+3", tcell.KeyRune, '3', tcell.ModShift},
	}

	for _, tc := range cases {
		key, r, mod, err := Parse(tc.spec)
		assert.NoError(t, err, tc.spec)
		assert.Equal(t, tc.key, key, tc.spec)
		assert.Equal(t, tc.r, r, tc.spec)
		assert.Equal(t, tc.mod, mod, tc.spec)
	}
}

func TestCanonicalIDCaseInsensitive(t *testing.T) {
	id1 := CanonicalID(tcell.KeyRune, 'a', 0)
	id2 := CanonicalID(tcell.KeyRune, 'A', 0)
	assert.Equal(t, id1, id2)
}

func TestNormalizeEvent_Tab(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	key, r, mod := NormalizeEvent(ev)
	assert.Equal(t, tcell.KeyTab, key)
	assert.Zero(t, r)
	assert.Zero(t, mod)

	ctrl := tcell.NewEventKey(tcell.KeyCtrlI, 0, tcell.ModCtrl)
	key, r, mod = NormalizeEvent(ctrl)
	assert.Equal(t, tcell.KeyTab, key)
	assert.Zero(t, r)
	assert.Equal(t, tcell.ModCtrl, mod)
}

func TestNormalizeEvent_Backtab(t *testing.T) {
	backtab := tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
	key, r, mod := NormalizeEvent(backtab)
	assert.Equal(t, tcell.KeyBacktab, key)
	assert.Zero(t, r)
	assert.Zero(t, mod)

	shiftTab := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift)
	key, r, mod = NormalizeEvent(shiftTab)
	assert.Equal(t, tcell.KeyBacktab, key)
	assert.Zero(t, r)
	assert.Zero(t, mod)

	ctrlShiftTab := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModCtrl|tcell.ModShift)
	key, r, mod = NormalizeEvent(ctrlShiftTab)
	// tcell may coalesce Ctrl+Shift+Tab into KeyBacktab while preserving Ctrl.
	assert.Equal(t, tcell.KeyBacktab, key)
	assert.Zero(t, r)
	assert.Equal(t, tcell.ModCtrl, mod)
}

func TestNormalizeNavigationEvent_Backtab(t *testing.T) {
	shiftTab := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift)
	norm := NormalizeNavigationEvent(shiftTab)
	assert.Equal(t, tcell.KeyBacktab, norm.Key())
	assert.Zero(t, norm.Modifiers())
}

func TestNormalizeEvent_ShiftDigit(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyRune, '#', tcell.ModShift)
	key, r, mod := NormalizeEvent(ev)
	assert.Equal(t, tcell.KeyRune, key)
	assert.Equal(t, '3', r)
	assert.Equal(t, tcell.ModShift, mod)
}
