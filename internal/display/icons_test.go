package display

import "testing"

func TestIconText(t *testing.T) {
	t.Parallel()

	if got := IconText("ok", "Connected", true); got != "ok Connected" {
		t.Fatalf("expected icon-prefixed text, got %q", got)
	}
	if got := IconText("ok", "Connected", false); got != "Connected" {
		t.Fatalf("expected plain text, got %q", got)
	}
}
