package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

type pt struct{ X, Y float64 }

// canvas — холст одной картинки плюс помощники для текста, эмодзи и фигур.
// k — масштаб относительно ширины 1080 px, все размеры в стилях задаются для 1080.
type canvas struct {
	img   *image.RGBA
	k     float64
	fonts *Fonts
	emoji *EmojiSet
	pal   palette
}

func (c *canvas) W() float64 { return float64(c.img.Bounds().Dx()) }
func (c *canvas) H() float64 { return float64(c.img.Bounds().Dy()) }

func rgb(hex uint32) color.RGBA {
	return color.RGBA{uint8(hex >> 16), uint8(hex >> 8), uint8(hex), 255}
}

func withAlpha(c color.RGBA, a float64) color.RGBA {
	// image/color ждёт премультиплицированные значения.
	return color.RGBA{uint8(float64(c.R) * a), uint8(float64(c.G) * a), uint8(float64(c.B) * a), uint8(255 * a)}
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

// vGradient заливает прямоугольник вертикальным градиентом.
func (c *canvas) vGradient(r image.Rectangle, top, bottom color.RGBA) {
	r = r.Intersect(c.img.Bounds())
	h := float64(r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		col := lerp(top, bottom, float64(y-r.Min.Y)/h)
		i := c.img.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x++ {
			c.img.Pix[i], c.img.Pix[i+1], c.img.Pix[i+2], c.img.Pix[i+3] = col.R, col.G, col.B, 255
			i += 4
		}
	}
}

// glow — мягкое радиальное свечение поверх фона.
func (c *canvas) glow(cx, cy, radius float64, col color.RGBA, strength float64) {
	b := c.img.Bounds()
	x0, x1 := int(math.Max(0, cx-radius)), int(math.Min(float64(b.Max.X), cx+radius))
	y0, y1 := int(math.Max(0, cy-radius)), int(math.Min(float64(b.Max.Y), cy+radius))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy) / radius
			if d >= 1 {
				continue
			}
			a := strength * c.pal.Glow * (1 - d) * (1 - d)
			i := c.img.PixOffset(x, y)
			p := c.img.Pix[i : i+3 : i+3]
			p[0] = uint8(float64(p[0])*(1-a) + float64(col.R)*a)
			p[1] = uint8(float64(p[1])*(1-a) + float64(col.G)*a)
			p[2] = uint8(float64(p[2])*(1-a) + float64(col.B)*a)
		}
	}
}

// fillPath заливает многоугольник со сглаживанием. Растеризатор берётся
// размером с рамку фигуры, а не всей картинки, — так дешевле.
func (c *canvas) fillPath(pts []pt, col color.Color) {
	if len(pts) < 3 {
		return
	}
	minX, minY, maxX, maxY := pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	r := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1)
	z := vector.NewRasterizer(r.Dx(), r.Dy())
	ox, oy := float32(r.Min.X), float32(r.Min.Y)
	z.MoveTo(float32(pts[0].X)-ox, float32(pts[0].Y)-oy)
	for _, p := range pts[1:] {
		z.LineTo(float32(p.X)-ox, float32(p.Y)-oy)
	}
	z.ClosePath()
	if r.In(c.img.Bounds()) {
		z.Draw(c.img, r, image.NewUniform(col), image.Point{})
		return
	}
	// Фигура выходит за край: рисуем маску и накладываем только видимую часть.
	clip := r.Intersect(c.img.Bounds())
	if clip.Empty() {
		return
	}
	mask := image.NewAlpha(image.Rect(0, 0, r.Dx(), r.Dy()))
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	draw.DrawMask(c.img, clip, image.NewUniform(col), image.Point{}, mask, clip.Min.Sub(r.Min), draw.Over)
}

func arc(cx, cy, r, a0, a1 float64, steps int) []pt {
	out := make([]pt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(steps)
		out = append(out, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return out
}

func (c *canvas) roundRect(x0, y0, x1, y1, r float64, col color.Color) {
	var p []pt
	p = append(p, arc(x1-r, y0+r, r, -math.Pi/2, 0, 10)...)
	p = append(p, arc(x1-r, y1-r, r, 0, math.Pi/2, 10)...)
	p = append(p, arc(x0+r, y1-r, r, math.Pi/2, math.Pi, 10)...)
	p = append(p, arc(x0+r, y0+r, r, math.Pi, 1.5*math.Pi, 10)...)
	c.fillPath(p, col)
}

func (c *canvas) circle(cx, cy, r float64, col color.Color) {
	c.fillPath(arc(cx, cy, r, 0, 2*math.Pi, 40), col)
}

// polyline рисует ломаную толщиной w со скруглёнными стыками.
func (c *canvas) polyline(pts []pt, w float64, col color.Color) {
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		nx, ny := -dy/l*w/2, dx/l*w/2
		c.fillPath([]pt{{a.X + nx, a.Y + ny}, {b.X + nx, b.Y + ny}, {b.X - nx, b.Y - ny}, {a.X - nx, a.Y - ny}}, col)
	}
	for _, p := range pts {
		c.circle(p.X, p.Y, w/2, col)
	}
}

func (c *canvas) rect(x0, y0, x1, y1 float64, col color.Color) {
	c.fillPath([]pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}, col)
}

// --- текст ---

// segments режет строку на куски по шрифту: свой шрифт или запасной Inter.
func (c *canvas) segments(s string, w Weight, px float64) ([]string, []font.Face) {
	if !c.fonts.custom {
		return []string{s}, []font.Face{c.fonts.Face(w, px)}
	}
	var texts []string
	var faces []font.Face
	start, prev := 0, true
	for i, r := range s {
		has := c.fonts.Has(w, r)
		if i > start && has != prev {
			texts = append(texts, s[start:i])
			faces = append(faces, c.fonts.face(w, px, !prev))
			start = i
		}
		prev = has
	}
	texts = append(texts, s[start:])
	faces = append(faces, c.fonts.face(w, px, !prev))
	return texts, faces
}

func (c *canvas) textWidth(s string, w Weight, px float64) float64 {
	texts, faces := c.segments(s, w, px)
	total := 0.0
	for i := range texts {
		total += fix(font.MeasureString(faces[i], texts[i]))
	}
	return total
}

// text рисует строку; y — базовая линия.
func (c *canvas) text(s string, x, y float64, w Weight, px float64, col color.Color) float64 {
	texts, faces := c.segments(s, w, px)
	d := font.Drawer{Dst: c.img, Src: image.NewUniform(col), Dot: fixed.Point26_6{X: toFix(x), Y: toFix(y)}}
	for i := range texts {
		d.Face = faces[i]
		d.DrawString(texts[i])
	}
	return fix(d.Dot.X) - x
}

// emojiAt рисует эмодзи квадратом size с левым верхним углом (x, y).
func (c *canvas) emojiAt(name string, x, y, size float64) {
	img, err := c.emoji.Image(name, int(size+0.5))
	if err != nil {
		return
	}
	p := image.Pt(int(x+0.5), int(y+0.5))
	draw.Draw(c.img, img.Bounds().Add(p), img, image.Point{}, draw.Over)
}

func fix(v fixed.Int26_6) float64   { return float64(v) / 64 }
func toFix(v float64) fixed.Int26_6 { return fixed.Int26_6(v*64 + 0.5) }

// stroke — плавная открытая кривая одним многоугольником. В отличие от polyline
// не даёт «бусин» на стыках при полупрозрачном цвете.
func (c *canvas) stroke(pts []pt, w float64, col color.Color) {
	if len(pts) < 2 {
		return
	}
	left := make([]pt, len(pts))
	right := make([]pt, len(pts))
	for i := range pts {
		a, b := pts[i], pts[i]
		if i > 0 {
			a = pts[i-1]
		}
		if i+1 < len(pts) {
			b = pts[i+1]
		}
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		nx, ny := -dy/l*w/2, dx/l*w/2
		left[i] = pt{pts[i].X + nx, pts[i].Y + ny}
		right[len(pts)-1-i] = pt{pts[i].X - nx, pts[i].Y - ny}
	}
	c.fillPath(append(left, right...), col)
}
