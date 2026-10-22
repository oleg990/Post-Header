// Package ui — окно Post Header на Gio: всё рисуется своими силами,
// поэтому оформление одинаковое на Windows 7 и 11.
package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"gioui.org/widget/material"

	"postheader/assets"
)

// Подпись автора в интерфейсе (на картинки не попадает).
const Signature = "Post Header · автор — Олег"

var (
	cBg        = rgb(0x0B0A08)
	cPanel     = rgb(0x15130F)
	cPanelHi   = rgb(0x1C1914)
	cLine      = rgb(0x2C261C)
	cGold      = rgb(0xD9A85C)
	cGoldLight = rgb(0xF0C77E)
	cGoldDeep  = rgb(0xA87A34)
	cText      = rgb(0xEDE9E1)
	cMuted     = rgb(0x8E877A)
	cDark      = rgb(0x14110C)
	cDanger    = rgb(0xE06A5A)
)

func rgb(h uint32) color.NRGBA {
	return color.NRGBA{R: uint8(h >> 16), G: uint8(h >> 8), B: uint8(h), A: 255}
}

func alpha(c color.NRGBA, a float32) color.NRGBA {
	c.A = uint8(float32(c.A) * clamp01(a))
	return c
}

func mix(a, b color.NRGBA, t float32) color.NRGBA {
	t = clamp01(t)
	f := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), f(a.A, b.A)}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func newTheme() (*material.Theme, error) {
	var coll []font.FontFace
	faces := []struct {
		data   []byte
		weight font.Weight
	}{
		{assets.InterRegular, font.Normal},
		{assets.InterMedium, font.Medium},
		{assets.InterSemiBold, font.SemiBold},
		{assets.InterDisplayBold, font.Bold},
	}
	for _, f := range faces {
		face, err := opentype.Parse(f.data)
		if err != nil {
			return nil, err
		}
		coll = append(coll, font.FontFace{Font: font.Font{Typeface: "Inter", Weight: f.weight}, Face: face})
	}
	if face, err := opentype.Parse(assets.CormorantItalic); err == nil {
		coll = append(coll, font.FontFace{Font: font.Font{Typeface: "Cormorant", Style: font.Italic, Weight: font.SemiBold}, Face: face})
	}
	// Цветные эмодзи в поле ввода: шейпер берёт их отсюда, когда в Inter нет глифа.
	emoji, err := opentype.Parse(assets.NotoColorEmoji)
	if err != nil {
		return nil, err
	}
	coll = append(coll, font.FontFace{Font: font.Font{Typeface: "Noto Color Emoji"}, Face: emoji})

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(coll))
	th.Face = "Inter"
	th.Palette = material.Palette{Fg: cText, Bg: cBg, ContrastBg: cGold, ContrastFg: cDark}
	th.TextSize = 14
	return th, nil
}

// backdrop — фон окна: почти чёрный с тёплыми золотыми бликами, как на постах.
func backdrop(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	glows := []struct{ x, y, r, a float64 }{
		{0.5, -0.1, 0.75, 0.22},
		{0.08, 0.95, 0.5, 0.10},
		{0.95, 0.6, 0.45, 0.07},
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b := 11.0, 10.0, 8.0
			for _, gl := range glows {
				d := math.Hypot(float64(x)/float64(w)-gl.x, (float64(y)/float64(h)-gl.y)*float64(h)/float64(w)) / gl.r
				if d < 1 {
					a := gl.a * (1 - d) * (1 - d)
					r += (90 - r) * a
					g += (66 - g) * a
					b += (30 - b) * a
				}
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(r), uint8(g), uint8(b), 255
		}
	}
	return img
}
