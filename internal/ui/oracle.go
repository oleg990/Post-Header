package ui

import (
	"image"
	"image/color"
	"math"
	"math/rand"
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

	"postheader/internal/oracle"
)

// Предсказание дня: справа в верхней карточке, если там хватает места.
// Одно нажатие в сутки, новое предсказание открывается в 9:00.
// Блок нарочно в своём стиле — «ночное небо», фиолетовый и золото,
// шрифт Cormorant, — чтобы не сливаться с рабочей частью программы.

type oracleBox struct {
	btn       widget.Clickable
	start     time.Time
	animating bool
	hover     ease
}

// Палитра блока.
var (
	oSkyTop  = rgb(0x1E1548)
	oSkyBot  = rgb(0x08061A)
	oLilac   = rgb(0xB9A6FF)
	oLilacDk = rgb(0x6F5CC9)
	oCream   = rgb(0xFFF1D6)
	oGold    = rgb(0xF2C66D)
	oHexFill = rgb(0x231A52)
)

const (
	oracleAnim = 4200 * time.Millisecond // гексагон: заряд, вращение, вспышка, знак
	oracleType = 32 * time.Millisecond   // печать фразы — по букве
	typeFrom   = 0.8                     // с какой доли анимации печатается фраза
)

// Знаки дня: выпадают вместе с фразой.
var oracleSigns = []string{"звезда", "солнце", "луна", "сердце", "дом", "кристалл"}

func (a *App) oracleRevealed() bool { return a.cfg.OracleDay == oracle.Day(time.Now()) }

func (a *App) updateOracle(gtx C) {
	o := &a.oracle
	if o.btn.Clicked(gtx) && !o.animating {
		if !a.oracleRevealed() {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			idx, seen := oracle.Pick(a.cfg.OracleSeen, r)
			a.cfg.OracleDay, a.cfg.OracleIdx, a.cfg.OracleSeen = oracle.Day(time.Now()), idx, seen
			a.saveConfig()
		}
		o.start, o.animating = gtx.Now, true
	}
}

// layoutOracle рисует блок во всю отведённую область; если места мало —
// не рисует ничего.
func (a *App) layoutOracle(gtx C) D {
	if gtx.Constraints.Max.X < gtx.Dp(360) || gtx.Constraints.Max.Y < gtx.Dp(120) {
		return D{}
	}
	a.updateOracle(gtx)
	o := &a.oracle
	size := gtx.Constraints.Max
	revealed := a.oracleRevealed()
	phrase := []rune(oracle.Phrases[a.cfg.OracleIdx%len(oracle.Phrases)])

	// ход анимации: T 0..1 — гексагон, потом печатается фраза
	T, typed := float32(1), len(phrase)
	el := time.Duration(0)
	if o.animating {
		el = gtx.Now.Sub(o.start)
		T = clamp01(float32(el) / float32(oracleAnim))
		typed = int((el - time.Duration(typeFrom*float32(oracleAnim))) / oracleType)
		if typed < 0 {
			typed = 0
		}
		if T >= 1 && typed >= len(phrase) {
			o.animating, typed = false, len(phrase)
		} else {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	hv := o.hover.to(gtx, boolf(o.btn.Hovered()), 8)

	// небо
	r := image.Rectangle{Max: size}
	rad := gtx.Dp(16)
	fillGrad(gtx, r, rad, oSkyTop, oSkyBot)
	func() {
		defer clip.UniformRRect(r, rad).Push(gtx.Ops).Pop()
		sky(gtx, size, el, o.animating)
	}()
	strokeRRect(gtx, r, rad, float32(gtx.Dp(1.2)), mix(alpha(oLilacDk, 0.7), alpha(oGold, 0.8), hv*0.6))
	o.btn.Layout(gtx, func(gtx C) D {
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return D{Size: size}
	})

	// гексагон слева
	s := size.Y - gtx.Dp(20)
	if m := gtx.Dp(160); s > m {
		s = m
	}
	sign := a.cfg.OracleIdx % len(oracleSigns)
	func() {
		defer clip.UniformRRect(r, rad).Push(gtx.Ops).Pop() // искры и вспышка не вылетают за рамку
		defer op.Offset(image.Pt(gtx.Dp(14), (size.Y-s)/2)).Push(gtx.Ops).Pop()
		k := oracleKit{gtx: gtx, s: float32(s), T: T, moving: o.animating, revealed: revealed, hv: hv}
		k.hex(sign, float32(el.Seconds()))
	}()

	// текст справа
	th := a.th
	x := gtx.Dp(14) + s + gtx.Dp(22)
	cg := gtx
	cg.Constraints = layout.Exact(image.Pt(size.X-x-gtx.Dp(16), size.Y-gtx.Dp(26)))
	defer op.Offset(image.Pt(x, gtx.Dp(13))).Push(gtx.Ops).Pop()
	layout.Flex{Axis: layout.Vertical}.Layout(cg,
		layout.Rigid(oracleLabel(th, 19, "Предсказание дня", alpha(oLilac, 0.95))),
		layout.Flexed(1, func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				if !revealed {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(oracleLabel(th, 21, "Подбросьте гексагон — он покажет знак и слово дня.", oCream)),
						layout.Rigid(layout.Spacer{Height: 6}.Layout),
						layout.Rigid(label(th, 12.5, "Нажмите, чтобы узнать", alpha(oGold, 0.65+0.35*hv), font.Medium).Layout),
					)
				}
				if typed == 0 {
					return D{}
				}
				n := typed
				if n > len(phrase) {
					n = len(phrase)
				}
				txt := string(phrase[:n])
				l := material.Label(th, unit.Sp(23), txt)
				l.Font = font.Font{Typeface: "Cormorant", Style: font.Italic, Weight: font.SemiBold}
				l.Color = oCream
				l.MaxLines = 3
				return l.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			msg := "Одно предсказание в день · новое открывается в 9:00"
			if revealed && !o.animating {
				when := "завтра"
				if oracle.Next(time.Now()).Day() == time.Now().Day() {
					when = "сегодня"
				}
				msg = "Знак дня — " + oracleSigns[sign] + " · следующее предсказание " + when + " в 9:00"
			}
			return label(th, 11.5, msg, alpha(oLilac, 0.6), font.Normal).Layout(gtx)
		}),
	)
	return D{Size: size}
}

func oracleLabel(th *material.Theme, size unit.Sp, s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		l := material.Label(th, size, s)
		l.Font = font.Font{Typeface: "Cormorant", Style: font.Italic, Weight: font.SemiBold}
		l.Color = c
		return l.Layout(gtx)
	}
}

func smooth(p float32) float32 {
	p = clamp01(p)
	return p * p * (3 - 2*p)
}

// phase — доля участка [a, b] анимации, со сглаживанием.
func phase(T, a, b float32) float32 { return smooth((T - a) / (b - a)) }

// sky — звёзды и туманность. Звёзды мерцают, только пока идёт анимация,
// чтобы окно не перерисовывалось без дела.
func sky(gtx C, size image.Point, el time.Duration, moving bool) {
	w, h := float32(size.X), float32(size.Y)
	k := oracleKit{gtx: gtx}
	k.softGlow(f32.Pt(w*0.72, h*0.25), h*0.9, oLilacDk, 0.35)
	k.softGlow(f32.Pt(w*0.92, h*0.95), h*0.7, rgb(0x2B6C8F), 0.25)
	r := rand.New(rand.NewSource(7))
	t := el.Seconds()
	for i := 0; i < 46; i++ {
		x, y := r.Float32()*w, r.Float32()*h
		base := 0.25 + 0.5*r.Float32()
		rs := float32(gtx.Dp(0.7)) + r.Float32()*float32(gtx.Dp(1.1))
		a := base
		if moving {
			a = base * (0.55 + 0.45*float32(math.Sin(t*(1.5+float64(i%5))+float64(i))))
		}
		k.circle(f32.Pt(x, y), rs, alpha(oCream, a))
	}
}

// --- гексагон ---

type oracleKit struct {
	gtx      C
	s        float32 // сторона квадрата
	T        float32 // ход анимации 0..1
	moving   bool
	revealed bool
	hv       float32
}

func (k oracleKit) dp(v float32) float32 { return float32(k.gtx.Dp(1)) * v }

func (k oracleKit) circle(c f32.Point, r float32, col color.NRGBA) {
	rr := image.Rect(int(c.X-r), int(c.Y-r), int(c.X+r+0.5), int(c.Y+r+0.5))
	paint.FillShape(k.gtx.Ops, col, clip.Ellipse(rr).Op(k.gtx.Ops))
}

func (k oracleKit) ring(c f32.Point, r, w float32, col color.NRGBA) {
	rr := image.Rect(int(c.X-r), int(c.Y-r), int(c.X+r), int(c.Y+r))
	paint.FillShape(k.gtx.Ops, col, clip.Stroke{Path: clip.Ellipse(rr).Path(k.gtx.Ops), Width: w}.Op())
}

func (k oracleKit) poly(pts []f32.Point, col color.NRGBA) {
	var p clip.Path
	p.Begin(k.gtx.Ops)
	p.MoveTo(pts[0])
	for _, q := range pts[1:] {
		p.LineTo(q)
	}
	p.Close()
	paint.FillShape(k.gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
}

// softGlow — мягкое сияние из полупрозрачных кругов.
func (k oracleKit) softGlow(c f32.Point, r float32, col color.NRGBA, a float32) {
	if a <= 0 {
		return
	}
	for i := 14; i >= 1; i-- {
		f := float32(i) / 14
		k.circle(c, r*f, alpha(col, a*0.05*(1.15-f)))
	}
}

func (k oracleKit) hexAt(c f32.Point, R, ang, sx float32, fill, stroke color.NRGBA, w float32) {
	defer op.Affine(f32.Affine2D{}.Rotate(c, ang).Scale(c, f32.Pt(sx, 1))).Push(k.gtx.Ops).Pop()
	if fill.A > 0 {
		paint.FillShape(k.gtx.Ops, fill, clip.Outline{Path: hexPath(k.gtx, c.X, c.Y, R, 1)}.Op())
	}
	strokeHex(k.gtx, c.X, c.Y, R, w, 1, stroke)
}

// hex — сама анимация:
//
//	0.00–0.22 заряд: гексагон сжимается, искры стягиваются к нему;
//	0.18–0.72 вращение: быстрый оборот с переворотом и шлейфом, вокруг летают искры;
//	0.70–0.85 вспышка: кольцо света и разлёт искр;
//	0.76–1.00 знак дня проявляется с лёгким «пружинящим» увеличением.
func (k oracleKit) hex(sign int, t float32) {
	s := k.s
	c := f32.Pt(s/2, s/2)
	R := s * 0.36
	T := k.T
	if !k.moving {
		T = 1
	}
	charge := phase(T, 0, 0.22)
	spin := phase(T, 0.18, 0.72)
	burst := clamp01((T - 0.7) / 0.15)
	reveal := float32(0)
	if k.revealed {
		reveal = clamp01((T - 0.76) / 0.24)
	}
	idle := !k.moving

	// сияние за гексагоном
	glow := 0.35 + 0.25*k.hv
	if k.moving {
		glow = 0.35 + 0.9*charge*(1-burst) + 0.6*reveal
	} else if k.revealed {
		glow = 0.75 + 0.2*k.hv
	}
	k.softGlow(c, R*1.9, oGold, glow*0.8)
	k.softGlow(c, R*1.5, oLilac, glow*0.5)

	sc := float32(1)
	ang := float32(0)
	sx := float32(1)
	if k.moving {
		sc = 1 - 0.14*charge*(1-spin) + 0.18*float32(math.Sin(float64(spin)*math.Pi))
		ang = spin * math.Pi * 6
		sx = 0.35 + 0.65*float32(math.Abs(math.Cos(float64(spin)*math.Pi*3)))
		if burst > 0 {
			sx = 1
			sc = 1 + 0.12*float32(math.Sin(float64(burst)*math.Pi))
		}
	}

	// искры: стягиваются, кружат, разлетаются
	if k.moving {
		for i := 0; i < 14; i++ {
			base := float64(i) * 2 * math.Pi / 14
			var rr float32
			var a float32
			switch {
			case T < 0.22:
				rr = R * (2.1 - 0.9*charge)
				a = charge
			case T < 0.7:
				rr = R * (1.2 + 0.08*float32(math.Sin(float64(t)*6+base)))
				a = 1
			default:
				rr = R * (1.2 + 1.6*burst)
				a = 1 - burst
			}
			aa := base + float64(spin)*math.Pi*4 + float64(charge)*0.8
			p := f32.Pt(c.X+rr*float32(math.Cos(aa)), c.Y+rr*float32(math.Sin(aa)))
			col := oGold
			if i%3 == 0 {
				col = oLilac
			}
			k.circle(p, k.dp(1.8+float32(i%3)*0.6), alpha(col, a))
			k.circle(p, k.dp(4.5), alpha(col, a*0.18))
		}
	}

	// шлейф при вращении
	if k.moving && spin > 0 && spin < 1 {
		for i := 4; i >= 1; i-- {
			back := float32(i) * 0.06
			ga := (spin - back) * math.Pi * 6
			k.hexAt(c, R*sc, ga, sx, color.NRGBA{}, alpha(oLilac, 0.22/float32(i)), k.dp(2))
		}
	}

	// гексагон
	k.hexAt(c, R*sc, ang, sx, oHexFill, oGold, k.dp(3))
	k.hexAt(c, R*sc*0.8, ang, sx, color.NRGBA{}, alpha(oLilac, 0.55), k.dp(1))
	if idle && !k.revealed {
		k.circle(c, R*0.1, alpha(oGold, 0.8))
	}

	// вспышка
	if burst > 0 && burst < 1 {
		k.ring(c, R*(1+1.6*burst), k.dp(3*(1-burst)+0.5), alpha(oCream, 1-burst))
		k.circle(c, R*1.1, alpha(oCream, 0.35*(1-burst)*(1-burst)))
	}

	// знак дня
	if reveal > 0 {
		e := reveal
		pop := e * (1 + 0.25*float32(math.Sin(float64(e)*math.Pi))) // с перелётом
		k.sign(sign, c, R*0.48*pop, alpha(oGold, e))
	}
}

// sign рисует знак дня размером примерно r.
func (k oracleKit) sign(n int, c f32.Point, r float32, col color.NRGBA) {
	if r <= 0 {
		return
	}
	switch n % len(oracleSigns) {
	case 0: // звезда
		var pts []f32.Point
		for i := 0; i < 10; i++ {
			rr := r
			if i%2 == 1 {
				rr = r * 0.42
			}
			a := -math.Pi/2 + float64(i)*math.Pi/5
			pts = append(pts, f32.Pt(c.X+rr*float32(math.Cos(a)), c.Y+rr*float32(math.Sin(a))))
		}
		k.poly(pts, col)
	case 1: // солнце
		k.circle(c, r*0.45, col)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cs, sn := float32(math.Cos(a)), float32(math.Sin(a))
			w := r * 0.12
			k.poly([]f32.Point{
				{X: c.X + cs*r*0.6 - sn*w, Y: c.Y + sn*r*0.6 + cs*w},
				{X: c.X + cs*r*0.6 + sn*w, Y: c.Y + sn*r*0.6 - cs*w},
				{X: c.X + cs*r, Y: c.Y + sn*r},
			}, col)
		}
	case 2: // луна: круг, перекрытый кругом цвета гексагона
		k.circle(c, r*0.8, col)
		k.circle(f32.Pt(c.X+r*0.38, c.Y-r*0.2), r*0.7, oHexFill)
	case 3: // сердце
		var p clip.Path
		p.Begin(k.gtx.Ops)
		pt := func(x, y float32) f32.Point { return f32.Pt(c.X+x*r, c.Y+y*r) }
		p.MoveTo(pt(0, 0.85))
		p.CubeTo(pt(-1.15, 0.05), pt(-0.65, -0.95), pt(0, -0.35))
		p.CubeTo(pt(0.65, -0.95), pt(1.15, 0.05), pt(0, 0.85))
		p.Close()
		paint.FillShape(k.gtx.Ops, col, clip.Outline{Path: p.End()}.Op())
	case 4: // дом
		k.poly([]f32.Point{{X: c.X, Y: c.Y - r}, {X: c.X + r, Y: c.Y - r*0.1}, {X: c.X - r, Y: c.Y - r*0.1}}, col)
		k.poly([]f32.Point{{X: c.X - r*0.7, Y: c.Y - r*0.1}, {X: c.X + r*0.7, Y: c.Y - r*0.1}, {X: c.X + r*0.7, Y: c.Y + r*0.85}, {X: c.X - r*0.7, Y: c.Y + r*0.85}}, col)
		k.poly([]f32.Point{{X: c.X - r*0.2, Y: c.Y + r*0.25}, {X: c.X + r*0.2, Y: c.Y + r*0.25}, {X: c.X + r*0.2, Y: c.Y + r*0.85}, {X: c.X - r*0.2, Y: c.Y + r*0.85}}, oHexFill)
	default: // кристалл
		k.poly([]f32.Point{{X: c.X - r*0.55, Y: c.Y - r*0.55}, {X: c.X + r*0.55, Y: c.Y - r*0.55}, {X: c.X + r*0.9, Y: c.Y - r*0.15}, {X: c.X, Y: c.Y + r}, {X: c.X - r*0.9, Y: c.Y - r*0.15}}, col)
		k.poly([]f32.Point{{X: c.X - r*0.9, Y: c.Y - r*0.15}, {X: c.X + r*0.9, Y: c.Y - r*0.15}, {X: c.X, Y: c.Y + r}}, alpha(oCream, 0.25))
	}
}
