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
		{"A", tcell.KeyRune, 'a', 0},
		{"Shift+A", tcell.KeyRune, 'a', 0},
		{"Ctrl+A", tcell.KeyRune, 'a', tcell.ModCtrl},
		{"Ctrl+Shift+A", tcell.KeyRune, 'a', tcell.ModCtrl},
		{"Alt+1", tcell.KeyRune, '1', tcell.ModAlt},
		{"Win+A", tcell.KeyRune, 'a', tcell.ModMeta},
		{"Shift+F1", tcell.KeyF1, 0, tcell.ModShift},
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
