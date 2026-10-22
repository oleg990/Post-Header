package render

// Вёрстка блоков: перенос строк, висячие отступы, расчёт высоты при базовом кегле s.

type lineOut struct {
	x, y, h float64 // x — от левого края текстовой колонки, y — от верха блока
	size    float64
	big     float64 // самый крупный множитель в строке (≥ 1)
	weight  Weight
	heading bool
	mark    string // золотое «–» или номер «1.» перед пунктом
	spans   []span
}

type blockOut struct {
	h     float64
	icon  IconKind
	lines []lineOut
}

type metrics struct {
	s, textW float64
	iconCol  float64 // ширина колонки иконок (0 — без иконок)
	blockGap float64
}

func (c *canvas) layoutBlocks(blocks []block, m metrics) []blockOut {
	out := make([]blockOut, 0, len(blocks))
	s := m.s
	for _, b := range blocks {
		var bo blockOut
		bo.icon = b.icon
		y := 0.0
		for pi, p := range b.paras {
			size, weight, lh, indent := s, Regular, 1.32, 0.0
			switch p.kind {
			case pHeading:
				size, weight, lh = s*1.22, Bold, 1.25
				if pi > 0 {
					y += s * 0.35
				}
			case pDash, pDashCont:
				indent = s * 0.95
				if p.num != "" {
					indent = s * 1.35 // место под «10.»
				}
			}
			lines := c.wrap(p.spans, m.textW-indent, size, weight)
			for li, l := range lines {
				big := maxScale(l)
				h := size*lh + size*(big-1)*1.05
				bo.lines = append(bo.lines, lineOut{
					x: indent, y: y, h: h, size: size, big: big, weight: weight,
					heading: p.kind == pHeading,
					mark:    markFor(p, li),
					spans:   l,
				})
				y += h
			}
		}
		bo.h = y
		if m.iconCol > 0 && bo.h < m.iconCol*0.82 {
			bo.h = m.iconCol * 0.82
		}
		out = append(out, bo)
	}
	return out
}

// maxScale — самый крупный кусок строки; строка не бывает ниже обычной.
func maxScale(l []span) float64 {
	m := 1.0
	for _, sp := range l {
		if sp.kind != spSpace && sp.sc() > m {
			m = sp.sc()
		}
	}
	return m
}

func markFor(p para, li int) string {
	switch {
	case p.kind != pDash || li > 0:
		return ""
	case p.num != "":
		return p.num
	default:
		return "–"
	}
}

func stackHeight(bs []blockOut, gap float64) float64 {
	h := 0.0
	for i, b := range bs {
		if i > 0 {
			h += gap
		}
		h += b.h
	}
	return h
}

func (c *canvas) spanWidth(sp *span, size float64, w Weight) float64 {
	size *= sp.sc()
	switch sp.kind {
	case spEmoji:
		return size*1.15 + size*0.12
	case spSpace:
		return c.textWidth(" ", w, size)
	}
	if sp.accent && w == Regular {
		w = SemiBold
	}
	return c.textWidth(sp.text, w, size)
}

// wrap — жадный перенос по словам; слово длиннее строки режется по буквам.
func (c *canvas) wrap(spans []span, maxW, size float64, w Weight) [][]span {
	var lines [][]span
	var cur []span
	curW := 0.0
	pendingSpace := false
	spaceW := c.textWidth(" ", w, size)
	flush := func() {
		lines = append(lines, cur)
		cur, curW = nil, 0
	}
	for _, sp := range spans {
		if sp.kind == spSpace {
			pendingSpace = len(cur) > 0
			continue
		}
		sp.w = c.spanWidth(&sp, size, w)
		add := sp.w
		if pendingSpace {
			add += spaceW
		}
		if len(cur) > 0 && curW+add > maxW {
			// часть слова другого размера переносится вместе со всем словом
			var carry []span
			if sp.glue && !pendingSpace {
				j := len(cur) - 1
				for j > 0 && cur[j].glue {
					j--
				}
				if j > 0 {
					carry = append(carry, cur[j:]...)
					cur = cur[:j]
					for len(cur) > 0 && cur[len(cur)-1].kind == spSpace {
						cur = cur[:len(cur)-1]
					}
				}
			}
			flush()
			pendingSpace = false
			cur = carry
			curW = lineWidth(carry)
		}
		if pendingSpace {
			cur = append(cur, span{kind: spSpace, text: " ", w: spaceW})
			curW += spaceW
			pendingSpace = false
		}
		if sp.kind == spWord && sp.w > maxW {
			for _, part := range c.splitWord(sp, maxW, size, w) {
				if len(cur) > 0 {
					flush()
				}
				cur, curW = []span{part}, part.w
			}
			continue
		}
		cur = append(cur, sp)
		curW += sp.w
	}
	if len(cur) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

func (c *canvas) splitWord(sp span, maxW, size float64, w Weight) []span {
	var parts []span
	rs := []rune(sp.text)
	start := 0
	for i := 1; i <= len(rs); i++ {
		part := sp
		part.text = string(rs[start:i])
		if i < len(rs) && c.spanWidth(&part, size, w) <= maxW {
			continue
		}
		if i-start > 1 && i < len(rs) {
			part.text = string(rs[start : i-1])
			i--
		}
		part.w = c.spanWidth(&part, size, w)
		parts = append(parts, part)
		start = i
	}
	return parts
}

// paginate раскладывает блоки по слайдам высотой pageH.
// Блок, не влезающий на пустой слайд, режется по строкам.
func paginate(bs []blockOut, pageH, gap float64) [][]blockOut {
	var pages [][]blockOut
	var cur []blockOut
	used := 0.0
	for _, b := range bs {
		need := b.h
		if len(cur) > 0 {
			need += gap
		}
		if used+need <= pageH {
			cur = append(cur, b)
			used += need
			continue
		}
		if len(cur) > 0 {
			pages = append(pages, cur)
			cur, used = nil, 0
		}
		for b.h > pageH {
			head, tail := splitBlock(b, pageH)
			pages = append(pages, []blockOut{head})
			b = tail
		}
		cur, used = []blockOut{b}, b.h
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	return pages
}

func splitBlock(b blockOut, maxH float64) (blockOut, blockOut) {
	head := blockOut{icon: b.icon}
	tail := blockOut{icon: IconAuto} // у продолжения иконки нет
	for _, l := range b.lines {
		if l.y+l.h <= maxH || len(head.lines) == 0 {
			head.lines = append(head.lines, l)
			head.h = l.y + l.h
			continue
		}
		if len(tail.lines) == 0 {
			tail.h = 0
		}
		off := head.h
		l.y -= off
		tail.lines = append(tail.lines, l)
		tail.h = l.y + l.h
	}
	return head, tail
}
