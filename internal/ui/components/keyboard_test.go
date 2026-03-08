package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestKeyMatchEmptySpecReturnsFalse(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)

	if keyMatch(ev, "") {
		t.Fatal("expected empty key spec to never match")
	}

	if keyMatch(ev, "   ") {
		t.Fatal("expected whitespace-only key spec to never match")
	}
}
