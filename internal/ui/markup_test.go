package ui

import (
	"testing"

	"gioui.org/widget"
)

func edWith(text string, s, e int) *widget.Editor {
	ed := new(widget.Editor)
	ed.SetText(text)
	ed.SetCaret(s, e)
	return ed
}

func TestHeadingToggle(t *testing.T) {
	var m markupBar
	ed := edWith("Приятный бонус!\nтекст", 3, 3)
	m.toggleHeading(ed)
	if got := ed.Text(); got != "ПРИЯТНЫЙ БОНУС!\nтекст" {
		t.Fatalf("up = %q", got)
	}
	ed.SetCaret(3, 3)
	m.toggleHeading(ed)
	if got := ed.Text(); got != "Приятный бонус!\nтекст" {
		t.Errorf("back = %q", got)
	}
}

func TestGoldAndLists(t *testing.T) {
	ed := edWith("скидка 10%", 7, 10)
	toggleGold(ed)
	if ed.Text() != "скидка **10%**" {
		t.Fatalf("gold = %q", ed.Text())
	}
	toggleGold(ed) // выделение — «**10%**»
	if ed.Text() != "скидка 10%" {
		t.Errorf("ungold = %q", ed.Text())
	}
	ed = edWith("a\n- b\nc", 0, 7)
	toggleNumbers(ed)
	if ed.Text() != "1. a\n2. b\n3. c" {
		t.Fatalf("num = %q", ed.Text())
	}
	toggleNumbers(ed)
	if ed.Text() != "a\nb\nc" {
		t.Errorf("unnum = %q", ed.Text())
	}
}
