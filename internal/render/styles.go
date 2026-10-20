package render

import (
	"image/color"
	"math"
	"unicode/utf8"
)

// Оформление, которое отличает варианты друг от друга: баннер-заголовок,
// рамка, узор на фоне, выравнивание по центру.

type Align int

const (
	AlignLeft Align = iota
	AlignCenter
)

// banner — крупный заголовок на золотой плашке над текстом.
type banner struct {
	lines [][]span
	size  float64
	pad   float64
	h     float64
}

// takeBanner забирает из первого блока заголовок (или короткую первую
// строку) для плашки. nil — подходящего заголовка нет.
func takeBanner(blocks []block) ([]block, []span) {
	if len(blocks) == 0 || len(blocks[0].paras) == 0 {
		return blocks, nil
	}
	p := blocks[0].paras[0]
	n := 0
	for _, sp := range p.spans {
		n += utf8.RuneCountInString(sp.text)
	}
	alone := len(blocks[0].paras) == 1 && len(blocks) == 1
	if p.kind == pDash || p.kind == pDashCont || alone || (p.kind != pHeading && n > 48) {
		return blocks, nil
	}
	rest := append([]block(nil), blocks...)
	rest[0].paras = rest[0].paras[1:]
	if len(rest[0].paras) == 0 {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return blocks, nil
	}
	return rest, p.spans
}

func (c *canvas) layoutBanner(spans []span, width float64) *banner {
	k := c.k
	b := &banner{pad: 30 * k}
	for size := 66 * k; ; size -= 2 * k {
		b.size = size
		b.lines = c.wrap(spans, width-2*b.pad, size, Bold)
		if len(b.lines) <= 2 || size <= 40*k {
			break
		}
	}
	b.h = float64(len(b.lines))*b.size*1.18 + 2*b.pad*0.8
	return b
}

func (c *canvas) drawBanner(b *banner, x, y, w float64) {
	k := c.k
	c.glow(x+w/2, y+b.h/2, w*0.55, c.pal.Gold, 0.18)
	c.roundRect(x, y, x+w, y+b.h, 18*k, c.pal.HeadBottom)
	c.roundRect(x, y, x+w, y+b.h-3*k, 18*k, c.pal.Gold)
	// блик по верхнему краю плашки
	c.roundRect(x+18*k, y+3*k, x+w-18*k, y+6*k, 1.5*k, withAlpha(c.pal.Shine, 0.55))
	ly := y + b.pad*0.8
	for _, l := range b.lines {
		lw := lineWidth(l)
		lx := x + (w-lw)/2
		base := ly + b.size*1.18/2 + b.size*0.36
		for _, sp := range l {
			switch sp.kind {
			case spEmoji:
				es := b.size * 1.1
				c.emojiAt(sp.emoji, lx, ly+b.size*1.18/2-es/2, es)
			case spWord:
				c.text(sp.text, lx, base, Bold, b.size, c.pal.OnGold)
			}
			lx += sp.w
		}
		ly += b.size * 1.18
	}
}

func lineWidth(l []span) float64 {
	w := 0.0
	for _, sp := range l {
		w += sp.w
	}
	return w
}

// postFrame — тонкая двойная золотая рамка вокруг текста с ромбами в углах.
func (c *canvas) postFrame(x0, y0, x1, y1 float64) {
	k := c.k
	box := func(d, w float64, a float64) {
		p := []pt{{x0 + d, y0 + d}, {x1 - d, y0 + d}, {x1 - d, y1 - d}, {x0 + d, y1 - d}, {x0 + d, y0 + d}}
		c.polyline(p, w, withAlpha(c.pal.Gold, a))
	}
	box(0, 2*k, 0.75)
	box(9*k, 1*k, 0.4)
	for _, p := range []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}} {
		c.diamond(p.X, p.Y, 11*k, c.pal.Gold)
	}
}

func (c *canvas) diamond(cx, cy, r float64, col color.Color) {
	c.fillPath([]pt{{cx, cy - r}, {cx + r, cy}, {cx, cy + r}, {cx - r, cy}}, col)
}

// ornament — разделитель для центрированного текста: линии и ромб.
func (c *canvas) ornament(cx, y, half float64) {
	k := c.k
	c.goldRule(cx-half, cx-16*k, y, 1.4*k, 0.6)
	c.goldRule(cx+16*k, cx+half, y, 1.4*k, 0.6)
	c.diamond(cx, y, 7*k, c.pal.Gold)
}

// hexPattern — едва заметная сетка шестиугольников по фону (фирменный мотив).
func (c *canvas) hexPattern(alpha float64) {
	k := c.k
	r := 64 * k
	w := math.Sqrt(3) * r
	col := withAlpha(c.pal.Gold, alpha)
	for row := 0; float64(row)*r*1.5 < c.H()+r; row++ {
		y := float64(row) * r * 1.5
		off := 0.0
		if row%2 == 1 {
			off = w / 2
		}
		// сильнее к низу кадра, сверху сетка почти исчезает
		fade := math.Min(1, y/c.H()*1.4)
		if fade < 0.15 {
			continue
		}
		for x := -w + off; x < c.W()+w; x += w {
			c.polyline(hexagon(x, y, r), 1.2*k, withAlpha(col, fade))
		}
	}
}
