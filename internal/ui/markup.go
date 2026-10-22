package ui

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
)

// Кнопки разметки под полем текста: делают то же, что ручная разметка,
// но не нужно помнить правила. Каждая кнопка работает как переключатель:
// повторное нажатие возвращает как было. Ctrl+Z тоже отменяет.

type markupBar struct {
	item, num, heading, gold, emojiBtn, spellBtn Button
	bigger, smaller                              Button
	emojiOpen, spellOpen                         bool
	emojis                                       []Button

	spWord, spAdd []Button
	spSug         [][3]Button
	spList        widget.List

	// исходный текст строк, превращённых в заголовок: чтобы вернуть регистр
	headingOrig map[string]string
}

// Эмодзи, которые чаще всего нужны в постах о недвижимости.
var quickEmoji = []string{"✅", "❗", "🔥", "🎁", "🏠", "🏢", "🔑", "📍", "💰", "💳", "📄", "📞", "👉", "⭐", "🎉", "⚡"}

func (m *markupBar) update(gtx C, a *App) {
	ed := &a.text
	changed := true
	switch {
	case m.item.Clicked(gtx):
		toggleItems(ed)
	case m.num.Clicked(gtx):
		toggleNumbers(ed)
	case m.heading.Clicked(gtx):
		m.toggleHeading(ed)
	case m.gold.Clicked(gtx):
		toggleGold(ed)
	case m.bigger.Clicked(gtx):
		resizeSel(ed, 1)
	case m.smaller.Clicked(gtx):
		resizeSel(ed, -1)
	default:
		changed = false
	}
	if m.emojiBtn.Clicked(gtx) {
		m.emojiOpen = !m.emojiOpen
	}
	if m.spellBtn.Clicked(gtx) {
		m.spellOpen = !m.spellOpen
	}
	if m.updateSpell(gtx, a) {
		changed = true
	}
	if len(m.emojis) < len(quickEmoji) {
		m.emojis = make([]Button, len(quickEmoji))
	}
	for i := range quickEmoji {
		if m.emojis[i].Clicked(gtx) {
			ed.Insert(quickEmoji[i])
			changed = true
		}
	}
	if changed {
		gtx.Execute(key.FocusCmd{Tag: ed})
		a.dirty, a.dirtyAt = true, gtx.Now
	}
}

func (m *markupBar) layout(gtx C, a *App) D {
	row := func(items ...layout.FlexChild) layout.Widget {
		return func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...) }
	}
	chip := func(b *Button, txt string, sel bool) layout.FlexChild {
		return layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Right: 4}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return b.Layout(gtx, a.th, txt, btnChip, sel)
			})
		})
	}
	rows := []layout.FlexChild{
		layout.Rigid(row(
			chip(&m.item, "– Пункт", false),
			chip(&m.num, "1. Список", false),
			chip(&m.heading, "Заголовок", false),
			chip(&m.bigger, "Крупнее", false),
		)),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(row(
			chip(&m.gold, "Золотом", false),
			chip(&m.emojiBtn, "Эмодзи", m.emojiOpen),
			chip(&m.spellBtn, spellLabel(a), m.spellOpen),
			chip(&m.smaller, "Мельче", false),
		)),
	}
	if m.emojiOpen {
		var ch []layout.FlexChild
		for i := range quickEmoji {
			i := i
			ch = append(ch, layout.Flexed(1, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return m.emojis[i].Layout(gtx, a.th, quickEmoji[i], btnChip, false)
			}))
		}
		half := len(ch) / 2
		rows = append(rows,
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(row(ch[:half]...)),
			layout.Rigid(row(ch[half:]...)),
		)
	}
	if m.spellOpen {
		rows = append(rows,
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx C) D { return m.layoutSpell(gtx, a) }),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
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

func replaceRange(ed *widget.Editor, s, e int, repl string) {
	ed.SetCaret(s, e)
	ed.Insert(repl)
	ed.SetCaret(s, s+len([]rune(repl)))
}

// eachLine применяет f к строкам выделения (пустые строки не трогает).
func eachLine(ed *widget.Editor, f func(lines []string) []string) {
	r := []rune(ed.Text())
	s, e := selection(ed)
	ls, le := lineRange(r, s, e)
	lines := strings.Split(string(r[ls:le]), "\n")
	replaceRange(ed, ls, le, strings.Join(f(lines), "\n"))
}

var numPrefixRe = regexp.MustCompile(`^\d{1,2}[.)]\s+`)

func itemPrefix(l string) string {
	t := strings.TrimSpace(l)
	for _, p := range []string{"- ", "– ", "— ", "• "} {
		if strings.HasPrefix(t, p) {
			return p
		}
	}
	return ""
}

func stripPrefixes(l string) string {
	t := strings.TrimSpace(l)
	if p := itemPrefix(t); p != "" {
		t = strings.TrimSpace(t[len(p):])
	}
	return numPrefixRe.ReplaceAllString(t, "")
}

// toggleItems превращает строки выделения в пункты «- » или обратно.
func toggleItems(ed *widget.Editor) {
	eachLine(ed, func(lines []string) []string {
		all := true
		for _, l := range lines {
			if strings.TrimSpace(l) != "" && itemPrefix(l) == "" {
				all = false
			}
		}
		for i, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			if all {
				lines[i] = stripPrefixes(l)
			} else {
				lines[i] = "- " + stripPrefixes(l)
			}
		}
		return lines
	})
}

// toggleNumbers нумерует строки выделения 1., 2., 3. или снимает нумерацию.
func toggleNumbers(ed *widget.Editor) {
	eachLine(ed, func(lines []string) []string {
		all := true
		for _, l := range lines {
			if strings.TrimSpace(l) != "" && !numPrefixRe.MatchString(strings.TrimSpace(l)) {
				all = false
			}
		}
		n := 0
		for i, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			if all {
				lines[i] = stripPrefixes(l)
			} else {
				n++
				lines[i] = strconv.Itoa(n) + ". " + stripPrefixes(l)
			}
		}
		return lines
	})
}

// toggleHeading делает строку заголовком (прописными) или возвращает
// её прежний вид.
func (m *markupBar) toggleHeading(ed *widget.Editor) {
	if m.headingOrig == nil {
		m.headingOrig = map[string]string{}
	}
	r := []rune(ed.Text())
	s, e := selection(ed)
	if s == e {
		s, e = lineRange(r, s, e)
	}
	cur := string(r[s:e])
	if isUpper(cur) {
		orig, ok := m.headingOrig[cur]
		if !ok {
			orig = sentenceCase(cur)
		}
		replaceRange(ed, s, e, orig)
		return
	}
	up := strings.ToUpper(cur)
	m.headingOrig[up] = cur
	replaceRange(ed, s, e, up)
}

func isUpper(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return letters > 0
}

// sentenceCase: «ПРИЯТНЫЙ БОНУС» → «Приятный бонус».
func sentenceCase(s string) string {
	rs := []rune(strings.ToLower(s))
	for i, r := range rs {
		if unicode.IsLetter(r) {
			rs[i] = unicode.ToUpper(r)
			break
		}
	}
	return string(rs)
}

// toggleGold обрамляет выделение звёздочками **…** (золотой цвет на картинке)
// или снимает их, если они уже есть.
func toggleGold(ed *widget.Editor) {
	r := []rune(ed.Text())
	s, e := selection(ed)
	sel := string(r[s:e])
	switch {
	case len([]rune(sel)) >= 4 && strings.HasPrefix(sel, "**") && strings.HasSuffix(sel, "**"):
		replaceRange(ed, s, e, sel[2:len(sel)-2])
	case s >= 2 && e+2 <= len(r) && string(r[s-2:s]) == "**" && string(r[e:e+2]) == "**":
		replaceRange(ed, s-2, e+2, sel)
	case s == e:
		ed.Insert("****")
		ed.SetCaret(s+2, s+2)
	default:
		replaceRange(ed, s, e, "**"+sel+"**")
	}
}

// resizeSel делает выделенные слова или буквы крупнее (d=1) или мельче (d=-1):
// оборачивает их в {+…} / {-…}, а повторное нажатие меняет шаг. Без выделения
// берётся слово под курсором. Выделение остаётся на тексте, чтобы жать дальше.
func resizeSel(ed *widget.Editor, d int) {
	r := []rune(ed.Text())
	s, e := selection(ed)
	if s == e {
		isW := func(c rune) bool { return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '%' }
		for s > 0 && isW(r[s-1]) {
			s--
		}
		for e < len(r) && isW(r[e]) {
			e++
		}
		if s == e {
			return
		}
	}
	// уже размеченная группа: «{++текст}» выделена целиком или только текст внутри
	gs, ge, lvl := s, e, 0
	if sel := r[s:e]; len(sel) > 3 && sel[0] == '{' && sel[len(sel)-1] == '}' && (sel[1] == '+' || sel[1] == '-') {
		n := 1
		for n < len(sel)-1 && sel[n] == sel[1] {
			n++
		}
		lvl = n - 1
		if sel[1] == '-' {
			lvl = -lvl
		}
		s, e = gs+n, ge-1
	} else {
		j := s
		for j > 0 && (r[j-1] == '+' || r[j-1] == '-') && (j == s || r[j-1] == r[j]) {
			j--
		}
		if j < s && j > 0 && r[j-1] == '{' && e < len(r) && r[e] == '}' {
			lvl = s - j
			if r[j] == '-' {
				lvl = -lvl
			}
			gs, ge = j-1, e+1
		}
	}
	lvl += d
	if lvl > 14 {
		lvl = 14
	}
	if lvl < -3 {
		lvl = -3
	}
	inner := string(r[s:e])
	repl, off := inner, 0
	switch {
	case lvl > 0:
		repl, off = "{"+strings.Repeat("+", lvl)+inner+"}", 1+lvl
	case lvl < 0:
		repl, off = "{"+strings.Repeat("-", -lvl)+inner+"}", 1-lvl
	}
	ed.SetCaret(gs, ge)
	ed.Insert(repl)
	ed.SetCaret(gs+off, gs+off+len([]rune(inner)))
}
