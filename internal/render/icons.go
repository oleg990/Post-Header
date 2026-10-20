package render

import (
	"image/color"
	"math"
	"strings"
)

// Золотые контурные иконки слева от блоков, как в образцах.
// Рисуются векторно, поэтому чёткие в любом размере.

type IconKind int

const (
	IconAuto IconKind = iota
	IconCheck
	IconGift
	IconPercent
	IconAlert
	IconBuilding
	IconShield
	IconHouse
	IconPin
)

// IconChoices — список для выбора иконки вручную в интерфейсе.
var IconChoices = []struct {
	Kind IconKind
	ID   string
	Name string
}{
	{IconCheck, "check", "Галочка"},
	{IconGift, "gift", "Подарок"},
	{IconPercent, "percent", "Процент"},
	{IconAlert, "alert", "Восклицание"},
	{IconBuilding, "building", "Офис"},
	{IconShield, "shield", "Щит"},
	{IconHouse, "house", "Дом"},
	{IconPin, "pin", "Метка"},
}

type IconFrame int

const (
	FrameNone IconFrame = iota
	FrameRoundSquare
	FrameHexagon
)

func iconForEmoji(name string) IconKind {
	switch name {
	case "2705", "2714", "2611", "1f44d":
		return IconCheck
	case "2757", "2755", "26a0", "203c", "1f525":
		return IconAlert
	case "1f381", "1f389":
		return IconGift
	case "1f3e2", "1f3d9", "1f91d", "1f3ec":
		return IconBuilding
	case "1f3e0", "1f3e1", "1f511", "1f5dd":
		return IconHouse
	case "1f6e1", "1f512":
		return IconShield
	case "1f4cd", "1f4cc":
		return IconPin
	case "1f4b0", "1f4b8", "1f4b5":
		return IconPercent
	}
	return IconAuto
}

func iconForText(t string) IconKind {
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(t, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("скидк", "%", "кешбэк", "кэшбэк"):
		return IconPercent
	case has("бесплатно", "подар", "бонус"):
		return IconGift
	case has("офис", "дистанц", "встрет"):
		return IconBuilding
	case has("не страшно", "гарант", "безопасн", "по факту"):
		return IconShield
	case has("квартир", "дом", "новострой"):
		return IconHouse
	}
	return IconCheck
}

// icon рисует рамку и знак в квадрате (x, y, size).
func (c *canvas) icon(kind IconKind, frame IconFrame, x, y, size float64, gold color.RGBA) {
	lw := size * 0.055
	cx, cy := x+size/2, y+size/2
	switch frame {
	case FrameRoundSquare:
		c.glowBox(cx, cy, size*0.75, gold)
		r := size * 0.16
		c.strokeRoundRect(x+lw/2, y+lw/2, x+size-lw/2, y+size-lw/2, r, lw, gold)
	case FrameHexagon:
		c.glowBox(cx, cy, size*0.8, gold)
		c.polyline(hexagon(cx, cy, size/2-lw/2), lw, gold)
	}
	g := size * 0.5 // поле для знака
	gx, gy := cx-g/2, cy-g/2
	w := size * 0.07
	switch kind {
	case IconCheck:
		c.polyline([]pt{{gx + g*0.1, gy + g*0.52}, {gx + g*0.4, gy + g*0.8}, {gx + g*0.92, gy + g*0.22}}, w*1.1, gold)
	case IconAlert:
		c.roundRect(cx-w*0.75, gy, cx+w*0.75, gy+g*0.66, w*0.7, gold)
		c.circle(cx, gy+g*0.9, w*0.9, gold)
	case IconPercent:
		c.polyline([]pt{{gx + g*0.88, gy + g*0.08}, {gx + g*0.12, gy + g*0.92}}, w, gold)
		c.ring(gx+g*0.24, gy+g*0.22, g*0.17, w*0.85, gold)
		c.ring(gx+g*0.76, gy+g*0.78, g*0.17, w*0.85, gold)
	case IconGift:
		c.rect(gx+g*0.08, gy+g*0.3, gx+g*0.92, gy+g*0.5, gold)
		c.rect(gx+g*0.16, gy+g*0.56, gx+g*0.84, gy+g*0.98, gold)
		c.rect(cx-w*0.5, gy+g*0.3, cx+w*0.5, gy+g*0.98, c.pal.Cutout)
		c.polyline([]pt{{cx, gy + g*0.28}, {gx + g*0.22, gy + g*0.06}, {gx + g*0.3, gy + g*0.28}}, w*0.7, gold)
		c.polyline([]pt{{cx, gy + g*0.28}, {gx + g*0.78, gy + g*0.06}, {gx + g*0.7, gy + g*0.28}}, w*0.7, gold)
	case IconBuilding:
		c.rect(gx+g*0.1, gy+g*0.3, gx+g*0.5, gy+g, gold)
		c.rect(gx+g*0.55, gy, gx+g*0.92, gy+g, gold)
		for row := 0; row < 4; row++ {
			for col := 0; col < 2; col++ {
				wx := gx + g*(0.6+float64(col)*0.17)
				wy := gy + g*(0.1+float64(row)*0.2)
				c.rect(wx, wy, wx+g*0.1, wy+g*0.1, c.pal.Cutout)
			}
		}
	case IconShield:
		p := []pt{{cx, gy}, {gx + g*0.92, gy + g*0.16}, {gx + g*0.88, gy + g*0.6}, {cx, gy + g}, {gx + g*0.12, gy + g*0.6}, {gx + g*0.08, gy + g*0.16}}
		c.fillPath(p, gold)
	case IconHouse:
		c.polyline([]pt{{gx, gy + g*0.48}, {cx, gy + g*0.05}, {gx + g, gy + g*0.48}}, w, gold)
		c.rect(gx+g*0.18, gy+g*0.45, gx+g*0.82, gy+g*0.98, gold)
		c.rect(cx-g*0.12, gy+g*0.66, cx+g*0.12, gy+g*0.98, c.pal.Cutout)
	case IconPin:
		c.pin(cx, cy, g, w, gold)
	}
}

func (c *canvas) pin(cx, cy, g, w float64, col color.RGBA) {
	r := g * 0.32
	top := cy - g*0.18
	var p []pt
	p = append(p, arc(cx, top, r, math.Pi*0.75, math.Pi*2.25, 30)...)
	p = append(p, pt{cx, cy + g*0.48})
	p = append(p, p[0])
	c.polyline(p, w, col)
	c.ring(cx, top, r*0.35, w*0.9, col)
}

func hexagon(cx, cy, r float64) []pt {
	p := make([]pt, 0, 7)
	for i := 0; i <= 6; i++ {
		a := math.Pi/6 + float64(i)*math.Pi/3 // острые углы сверху и снизу
		p = append(p, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return p
}

func (c *canvas) ring(cx, cy, r, w float64, col color.Color) {
	c.polyline(arc(cx, cy, r, 0, 2*math.Pi, 36), w, col)
}

func (c *canvas) strokeRoundRect(x0, y0, x1, y1, r, w float64, col color.Color) {
	var p []pt
	p = append(p, arc(x1-r, y0+r, r, -math.Pi/2, 0, 8)...)
	p = append(p, arc(x1-r, y1-r, r, 0, math.Pi/2, 8)...)
	p = append(p, arc(x0+r, y1-r, r, math.Pi/2, math.Pi, 8)...)
	p = append(p, arc(x0+r, y0+r, r, math.Pi, 1.5*math.Pi, 8)...)
	p = append(p, p[0])
	c.polyline(p, w, col)
}

// glowBox — тёплое свечение за иконкой.
func (c *canvas) glowBox(cx, cy, r float64, gold color.RGBA) {
	c.glow(cx, cy, r, gold, 0.10)
}
