package ui

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/widget"
)

// Кнопки разметки над полем текста: делают то же, что ручная разметка,
// но не нужно помнить правила.

type markupBar struct {
	item, heading, gold, block Button
}

func (m *markupBar) update(gtx C, a *App) {
	ed := &a.text
	changed := true
	switch {
	case m.item.Clicked(gtx):
		toggleItems(ed)
	case m.heading.Clicked(gtx):
		upperLines(ed)
	case m.gold.Clicked(gtx):
		wrapGold(ed)
	case m.block.Clicked(gtx):
		ed.Insert("\n\n")
	default:
		changed = false
	}
	if changed {
		gtx.Execute(focusCmd(ed))
		a.dirty, a.dirtyAt = true, gtx.Now
	}
}

func selection(ed *widget.Editor) (int, int) {
	s, e := ed.Selection()
	if s > e {
		s, e = e, s
	}
	return s, e
}

// lineRange — начало первой и конец последней строки, задетых выделением.
func lineRange(r []rune, s, e int) (int, int) {
	for s > 0 && r[s-1] != '\n' {
		s--
	}
	for e < len(r) && r[e] != '\n' {
		e++
	}
	return s, e
}

// toggleItems превращает строки выделения в пункты «- » или обратно.
func toggleItems(ed *widget.Editor) {
	r := []rune(ed.Text())
	s, e := selection(ed)
	ls, le := lineRange(r, s, e)
	lines := strings.Split(string(r[ls:le]), "\n")
	all := true
	for _, l := range lines {
		if strings.TrimSpace(l) != "" && !hasItemPrefix(l) {
			all = false
		}
	}
	for i, l := range lines {
		switch {
		case strings.TrimSpace(l) == "":
		case all:
			lines[i] = strings.TrimLeft(strings.TrimSpace(l)[len(itemPrefix(l)):], " ")
		case !hasItemPrefix(l):
			lines[i] = "- " + l
		}
	}
	replaceRange(ed, ls, le, strings.Join(lines, "\n"))
}

func hasItemPrefix(l string) bool { return itemPrefix(l) != "" }

func itemPrefix(l string) string {
	t := strings.TrimSpace(l)
	for _, p := range []string{"- ", "– ", "— ", "• "} {
		if strings.HasPrefix(t, p) {
			return p
		}
	}
	return ""
}

// upperLines делает заголовок: выделенный текст (или текущую строку) — прописными.
func upperLines(ed *widget.Editor) {
	r := []rune(ed.Text())
	s, e := selection(ed)
	if s == e {
		s, e = lineRange(r, s, e)
	}
	replaceRange(ed, s, e, strings.ToUpper(string(r[s:e])))
}

// wrapGold обрамляет выделение звёздочками **…** — золотой цвет на картинке.
func wrapGold(ed *widget.Editor) {
	s, e := selection(ed)
	sel := ed.SelectedText()
	ed.Insert("**" + sel + "**")
	if s == e {
		ed.SetCaret(s+2, s+2)
	} else {
		ed.SetCaret(s, e+4)
	}
}

func replaceRange(ed *widget.Editor, s, e int, repl string) {
	ed.SetCaret(s, e)
	ed.Insert(repl)
	ed.SetCaret(s, s+len([]rune(repl)))
}

func focusCmd(ed *widget.Editor) key.FocusCmd { return key.FocusCmd{Tag: ed} }
