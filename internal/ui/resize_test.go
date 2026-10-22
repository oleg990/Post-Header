package ui

import (
	"testing"

	"gioui.org/widget"
)

func TestResizeSel(t *testing.T) {
	var ed widget.Editor
	ed.SetText("скидка 10% на всё")
	ed.SetCaret(0, 6) // «скидка»
	steps := []struct {
		d    int
		want string
	}{
		{1, "{+скидка} 10% на всё"},
		{1, "{++скидка} 10% на всё"},
		{-1, "{+скидка} 10% на всё"},
		{-1, "скидка 10% на всё"},
		{-1, "{-скидка} 10% на всё"},
	}
	for i, st := range steps {
		resizeSel(&ed, st.d)
		if ed.Text() != st.want {
			t.Fatalf("шаг %d: %q, ждали %q", i, ed.Text(), st.want)
		}
		if got := ed.SelectedText(); got != "скидка" {
			t.Fatalf("шаг %d: выделено %q", i, got)
		}
	}
	// выделена одна буква — крупнее только она
	ed.SetText("другим")
	ed.SetCaret(0, 1)
	resizeSel(&ed, 1)
	if ed.Text() != "{+д}ругим" {
		t.Fatalf("буква: %q", ed.Text())
	}
}

func TestResizeSeveralLines(t *testing.T) {
	var ed widget.Editor
	ed.SetText("Первая строка\n- Вторая\n\nТретья")
	ed.SetCaret(0, len([]rune(ed.Text())))
	resizeSel(&ed, 1)
	want := "{+Первая строка}\n- {+Вторая}\n\n{+Третья}"
	if ed.Text() != want {
		t.Fatalf("1: %q", ed.Text())
	}
	resizeSel(&ed, 1) // выделение осталось на тексте — повторное нажатие меняет шаг
	want = "{++Первая строка}\n- {++Вторая}\n\n{++Третья}"
	if ed.Text() != want {
		t.Fatalf("2: %q", ed.Text())
	}
	resizeSel(&ed, -1)
	resizeSel(&ed, -1)
	if ed.Text() != "Первая строка\n- Вторая\n\nТретья" {
		t.Fatalf("3: %q", ed.Text())
	}
}
