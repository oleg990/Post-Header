package ui

import (
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget/material"

	"postheader/internal/spell"
)

// Проверка орфографии: под полем текста список слов, которых нет в словаре,
// с вариантами исправления. Слово можно добавить в личный словарь.

func spellLabel(a *App) string {
	switch {
	case a.spell == nil:
		return "Орфография"
	case len(a.mistakes) == 0:
		return "Ошибок нет"
	default:
		return "Ошибки: " + itoa(len(a.mistakes))
	}
}

// recheck вызывается при каждом изменении текста.
func (a *App) recheck() {
	if a.spell == nil {
		a.mistakes = nil
		return
	}
	// Метки размера {+…} убираем, чтобы «{+Д}ругим» проверялось как одно слово.
	clean, pos := stripSizeMarks([]rune(a.text.Text()))
	a.mistakes = a.spell.Check(string(clean), 30)
	for i := range a.mistakes {
		a.mistakes[i].Start = pos[a.mistakes[i].Start]
		a.mistakes[i].End = pos[a.mistakes[i].End-1] + 1
	}
}

// stripSizeMarks убирает «{+», «{--», «}» и возвращает, где каждая
// оставшаяся буква стояла в исходном тексте.
func stripSizeMarks(r []rune) ([]rune, []int) {
	out := make([]rune, 0, len(r))
	pos := make([]int, 0, len(r))
	open := false
	for i := 0; i < len(r); i++ {
		if !open && r[i] == '{' && i+1 < len(r) && (r[i+1] == '+' || r[i+1] == '-') {
			j := i + 1
			for j < len(r) && r[j] == r[i+1] {
				j++
			}
			i, open = j-1, true
			continue
		}
		if open && r[i] == '}' {
			open = false
			continue
		}
		out = append(out, r[i])
		pos = append(pos, i)
	}
	return out, pos
}

func (m *markupBar) ensureSpellButtons(n int) {
	if len(m.spWord) < n {
		m.spWord = make([]Button, n)
		m.spAdd = make([]Button, n)
		m.spSug = make([][3]Button, n)
	}
}

// locate находит ошибку в текущем тексте: по сохранённой позиции, а если
// текст сдвинулся — первое вхождение слова.
func locate(text []rune, ms spell.Mistake) (int, int, bool) {
	if ms.End <= len(text) && string(text[ms.Start:ms.End]) == ms.Word {
		return ms.Start, ms.End, true
	}
	i := strings.Index(string(text), ms.Word)
	if i < 0 {
		return 0, 0, false
	}
	s := len([]rune(string(text)[:i]))
	return s, s + len([]rune(ms.Word)), true
}

func (m *markupBar) updateSpell(gtx C, a *App) bool {
	m.ensureSpellButtons(len(a.mistakes))
	ed := &a.text
	for i, ms := range a.mistakes {
		if m.spWord[i].Clicked(gtx) {
			if s, e, ok := locate([]rune(ed.Text()), ms); ok {
				ed.SetCaret(s, e)
				gtx.Execute(key.FocusCmd{Tag: ed})
			}
		}
		for j, sug := range ms.Suggest {
			if j < 3 && m.spSug[i][j].Clicked(gtx) {
				if s, e, ok := locate([]rune(ed.Text()), ms); ok {
					ed.SetCaret(s, e)
					ed.Insert(sug)
					a.recheck()
					return true
				}
			}
		}
		if m.spAdd[i].Clicked(gtx) {
			a.cfg.UserWords = append(a.cfg.UserWords, ms.Word)
			a.spell.AddUser(ms.Word)
			a.saveConfig()
			a.recheck()
			a.notify("«"+ms.Word+"» добавлено в ваш словарь", false)
			return false
		}
	}
	return false
}

func (m *markupBar) layoutSpell(gtx C, a *App) D {
	th := a.th
	if a.spell == nil {
		return label(th, 12.5, "Словарь загружается…", cMuted, font.Normal).Layout(gtx)
	}
	if len(a.mistakes) == 0 {
		return label(th, 12.5, "Ошибок не нашёл. Проверяются русские слова; аббревиатуры вроде ЖК пропускаются.", cMuted, font.Normal).Layout(gtx)
	}
	m.spList.Axis = layout.Vertical
	m.ensureSpellButtons(len(a.mistakes))
	gtx.Constraints.Max.Y = gtx.Dp(132)
	gtx.Constraints.Min.Y = 0
	return material.List(th, &m.spList).Layout(gtx, len(a.mistakes), func(gtx C, i int) D {
		ms := a.mistakes[i]
		ch := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				return m.spWord[i].Layout(gtx, th, ms.Word, btnChip, false)
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(label(th, 13, "→", cMuted, font.Normal).Layout),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
		}
		if len(ms.Suggest) == 0 {
			ch = append(ch, layout.Rigid(label(th, 12, "нет вариантов", cMuted, font.Normal).Layout))
		}
		for j, sug := range ms.Suggest {
			if j >= 3 {
				break
			}
			j, sug := j, sug
			ch = append(ch, layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: 4}.Layout(gtx, func(gtx C) D { return m.spSug[i][j].Layout(gtx, th, sug, btnGhost, false) })
			}))
		}
		ch = append(ch, layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D { return m.spAdd[i].Layout(gtx, th, "В словарь", btnChip, false) })
		}))
		return layout.Inset{Bottom: 4, Right: 8}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, ch...)
		})
	})
}
