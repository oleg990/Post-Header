package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type D = layout.Dimensions
type C = layout.Context

// ease плавно ведёт значение к цели (наведение, нажатие, появление).
type ease struct {
	v    float32
	last time.Time
}

func (e *ease) to(gtx C, target, speed float32) float32 {
	now := gtx.Now
	if e.last.IsZero() {
		e.last = now
	}
	dt := float32(now.Sub(e.last).Seconds())
	e.last = now
	if dt > 0.1 {
		dt = 0.1
	}
	e.v += (target - e.v) * (1 - float32(math.Exp(float64(-speed*dt))))
	if d := target - e.v; d < 0.003 && d > -0.003 {
		e.v = target
	} else {
		gtx.Execute(op.InvalidateCmd{})
	}
	return e.v
}

// timeline — анимация по времени от старта (заставка, появление карточек).
type timeline struct{ start time.Time }

func (t *timeline) restart(now time.Time) { t.start = now }

// at возвращает прогресс 0..1 участка [from, from+dur] со сглаживанием.
func (t *timeline) at(gtx C, from, dur time.Duration) float32 {
	if t.start.IsZero() {
		t.start = gtx.Now
	}
	p := float32(gtx.Now.Sub(t.start)-from) / float32(dur)
	if p < 1 {
		gtx.Execute(op.InvalidateCmd{})
	}
	return easeOut(clamp01(p))
}

func easeOut(p float32) float32 { return 1 - (1-p)*(1-p)*(1-p) }

// --- рисование ---

func fillRRect(gtx C, r image.Rectangle, radius int, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, radius).Op(gtx.Ops))
}

func strokeRRect(gtx C, r image.Rectangle, radius int, width float32, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: clip.UniformRRect(r, radius).Path(gtx.Ops), Width: width}.Op())
}

func fillGrad(gtx C, r image.Rectangle, radius int, top, bottom color.NRGBA) {
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	paint.LinearGradientOp{
		Stop1: f32.Pt(0, float32(r.Min.Y)), Color1: top,
		Stop2: f32.Pt(0, float32(r.Max.Y)), Color2: bottom,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// fillGradRect — вертикальный градиент в прямоугольнике без скругления.
func fillGradRect(gtx C, r image.Rectangle, top, bottom color.NRGBA) {
	fillGrad(gtx, r, 0, top, bottom)
}

// hexPath — шестиугольник (фирменный элемент «Адреса»); frac < 1 рисует его
// частично — так в заставке он «прорисовывается» золотой линией.
func hexPath(gtx C, cx, cy, r float32, frac float32) clip.PathSpec {
	var p clip.Path
	p.Begin(gtx.Ops)
	pt := func(i float32) f32.Point {
		a := float64(-math.Pi/2 + float64(i)*math.Pi/3)
		return f32.Pt(cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a)))
	}
	p.MoveTo(pt(0))
	total := frac * 6
	for i := float32(1); i <= 6; i++ {
		if total >= i {
			p.LineTo(pt(i))
			continue
		}
		a, b := pt(i-1), pt(i)
		t := total - (i - 1)
		if t > 0 {
			p.LineTo(f32.Pt(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t))
		}
		break
	}
	return p.End()
}

func strokeHex(gtx C, cx, cy, r, width, frac float32, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: hexPath(gtx, cx, cy, r, frac), Width: width}.Op())
}

// --- текст ---

func label(th *material.Theme, size unit.Sp, s string, c color.NRGBA, w font.Weight) material.LabelStyle {
	l := material.Label(th, size, s)
	l.Color = c
	l.Font.Weight = w
	return l
}

// --- кнопки ---

type btnKind int

const (
	btnPrimary btnKind = iota // золотая
	btnGhost                  // контур
	btnChip                   // переключатель в группе
)

type Button struct {
	widget.Clickable
	hover, press ease
}

func (b *Button) Layout(gtx C, th *material.Theme, txt string, kind btnKind, selected bool) D {
	h := b.hover.to(gtx, boolf(b.Hovered()), 14)
	pr := b.press.to(gtx, boolf(b.Pressed()), 24)
	return b.Clickable.Layout(gtx, func(gtx C) D {
		var fg color.NRGBA
		weight := font.SemiBold
		switch {
		case kind == btnPrimary || (kind == btnChip && selected):
			fg = cDark
		case kind == btnGhost:
			fg = mix(cGold, cGoldLight, h)
		default:
			fg = mix(cMuted, cText, h)
			weight = font.Medium
		}
		padV, padH := unit.Dp(9), unit.Dp(16)
		if kind == btnChip {
			padV, padH = 7, 13
		}
		if gtx.Constraints.Min.X > 0 {
			padH = 6 // растянутая кнопка: текст центрируется, поля не нужны
		}
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: padV, Bottom: padV, Left: padH, Right: padH}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = 0
			l := label(th, 13.5, txt, fg, weight)
			l.MaxLines = 1
			return l.Layout(gtx)
		})
		call := m.Stop()
		natural := dims.Size.X
		if gtx.Constraints.Min.X > dims.Size.X {
			dims.Size.X = gtx.Constraints.Min.X
		}
		r := image.Rectangle{Max: dims.Size}
		rad := gtx.Dp(9)
		switch {
		case kind == btnPrimary || (kind == btnChip && selected):
			top := mix(cGoldLight, rgb(0xFFE3A8), h*0.6)
			bot := mix(cGold, cGoldDeep, pr*0.6)
			fillGrad(gtx, r, rad, top, bot)
		case kind == btnGhost:
			fillRRect(gtx, r, rad, alpha(cGold, 0.04+0.10*h))
			strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), alpha(cGold, 0.45+0.4*h))
		default:
			fillRRect(gtx, r, rad, mix(cPanelHi, rgb(0x241F17), h))
			strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), mix(cLine, alpha(cGold, 0.5), h))
		}
		// текст по центру, если кнопку растянули
		stack := op.Offset(image.Pt((dims.Size.X-natural)/2, 0)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		stack.Pop()
		defer clip.Rect(r).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return dims
	})
}

func boolf(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// segmented — группа переключателей «одно из»: возвращает новый выбор.
func segmented(gtx C, th *material.Theme, btns []Button, labels []string, selected *int) D {
	for i := range btns {
		if btns[i].Clicked(gtx) {
			*selected = i
		}
	}
	children := make([]layout.FlexChild, 0, len(labels)*2)
	for i := range labels {
		i := i
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 6}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return btns[i].Layout(gtx, th, labels[i], btnChip, *selected == i)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// card — тёмная панель со скруглением, тонкой рамкой и заголовком.
// fill — карточка занимает всю отведённую высоту (тело растягивается).
func card(gtx C, th *material.Theme, title string, body layout.Widget, fill bool) D {
	if fill {
		gtx.Constraints.Min = gtx.Constraints.Max
	} else {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	}
	m := op.Record(gtx.Ops)
	dims := layout.UniformInset(16).Layout(gtx, func(gtx C) D {
		if title == "" {
			return body(gtx)
		}
		bodyChild := layout.Rigid(body)
		if fill {
			bodyChild = layout.Flexed(1, body)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return label(th, 11.5, strings.ToUpper(title), cGold, font.SemiBold).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: 10}.Layout),
			bodyChild,
		)
	})
	call := m.Stop()
	r := image.Rectangle{Max: dims.Size}
	rad := gtx.Dp(14)
	fillGrad(gtx, r, rad, alpha(cPanelHi, 0.92), alpha(cPanel, 0.92))
	strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), cLine)
	call.Add(gtx.Ops)
	return dims
}

// editorStyle — поле ввода в фирменных цветах.
func editorStyle(th *material.Theme, ed *widget.Editor, hint string) material.EditorStyle {
	e := material.Editor(th, ed, hint)
	e.Color = cText
	e.HintColor = alpha(cMuted, 0.8)
	e.SelectionColor = alpha(cGold, 0.35)
	e.TextSize = 15
	e.LineHeightScale = 1.25
	return e
}

func fieldFrame(gtx C, focused bool, f *ease, w layout.Widget) D {
	v := f.to(gtx, boolf(focused), 12)
	m := op.Record(gtx.Ops)
	dims := layout.UniformInset(12).Layout(gtx, w)
	call := m.Stop()
	r := image.Rectangle{Max: dims.Size}
	rad := gtx.Dp(10)
	fillRRect(gtx, r, rad, rgb(0x0F0E0B))
	strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), mix(cLine, alpha(cGold, 0.7), v))
	call.Add(gtx.Ops)
	return dims
}
