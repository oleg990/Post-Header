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
	"gioui.org/widget"

	"postheader/internal/oracle"
)

// Предсказание дня: справа в верхней карточке, если там хватает места.
// Одно нажатие в сутки, новое предсказание открывается в 9:00.
// Четыре оформления на выбор: шар, печенье, ключ, гексагон.

type oracleBox struct {
	btn        widget.Clickable
	prev, next Button
	anim       timeline
	animating  bool
	hover      ease
}

var oracleStyles = []struct{ name, prompt, verb string }{
	{"Шар", "Встряхните шар — он знает, каким будет ваш день.", "Встряхнуть шар"},
	{"Печенье", "Разломите печенье — внутри записка для вас.", "Разломить печенье"},
	{"Ключ", "Поверните ключ — за дверью ваш день.", "Повернуть ключ"},
	{"Гексагон", "Подбросьте гексагон — он покажет знак дня.", "Подбросить гексагон"},
}

const oracleAnim = 1500 * time.Millisecond

func (a *App) oracleRevealed() bool { return a.cfg.OracleDay == oracle.Day(time.Now()) }

func (a *App) updateOracle(gtx C) {
	o := &a.oracle
	st := a.cfg.OracleStyle
	switched := false
	if o.prev.Clicked(gtx) {
		st, switched = (st+len(oracleStyles)-1)%len(oracleStyles), true
	}
	if o.next.Clicked(gtx) {
		st, switched = (st+1)%len(oracleStyles), true
	}
	if switched {
		a.cfg.OracleStyle = st
		a.saveConfig()
		if a.oracleRevealed() { // показать то же предсказание в новом оформлении
			o.anim.restart(gtx.Now)
			o.animating = true
		}
	}
	if o.btn.Clicked(gtx) && !o.animating {
		if !a.oracleRevealed() {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			idx, seen := oracle.Pick(a.cfg.OracleSeen, r)
			a.cfg.OracleDay, a.cfg.OracleIdx, a.cfg.OracleSeen = oracle.Day(time.Now()), idx, seen
			a.saveConfig()
		}
		o.anim.restart(gtx.Now)
		o.animating = true
	}
}

// layoutOracle рисует блок во всю отведённую область; если места мало —
// не рисует ничего.
func (a *App) layoutOracle(gtx C) D {
	if gtx.Constraints.Max.X < gtx.Dp(340) || gtx.Constraints.Max.Y < gtx.Dp(120) {
		return D{}
	}
	a.updateOracle(gtx)
	o := &a.oracle
	th := a.th
	size := gtx.Constraints.Max
	t := float32(1)
	if o.animating {
		t = float32(gtx.Now.Sub(o.anim.start)) / float32(oracleAnim)
		if t >= 1 {
			t, o.animating = 1, false
		} else {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	revealed := a.oracleRevealed()
	rv := float32(0) // насколько открыто предсказание
	if revealed {
		rv = smooth((t - 0.6) / 0.4)
	}
	hv := o.hover.to(gtx, boolf(o.btn.Hovered()), 10)

	// подложка и кнопка на всю область
	r := image.Rectangle{Max: size}
	rad := gtx.Dp(14)
	fillGrad(gtx, r, rad, alpha(cPanelHi, 0.9), alpha(cBg, 0.6))
	strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), mix(cLine, alpha(cGold, 0.6), hv+rv*0.4))
	o.btn.Layout(gtx, func(gtx C) D {
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return D{Size: size}
	})

	// объект слева
	s := size.Y - gtx.Dp(24)
	if max := gtx.Dp(150); s > max {
		s = max
	}
	func() {
		defer op.Offset(image.Pt(gtx.Dp(14), (size.Y-s)/2)).Push(gtx.Ops).Pop()
		st := a.cfg.OracleStyle % len(oracleStyles)
		anim := t
		if !o.animating {
			anim = 1
		}
		k := oracleKit{gtx: gtx, s: float32(s), t: anim, moving: o.animating, rv: rv, hv: hv}
		switch st {
		case 0:
			k.ball()
		case 1:
			k.cookie(revealed)
		case 2:
			k.key(revealed)
		case 3:
			k.hex(revealed)
		}
	}()

	// текст справа
	x := gtx.Dp(14) + s + gtx.Dp(18)
	cg := gtx
	cg.Constraints = layout.Exact(image.Pt(size.X-x-gtx.Dp(12), size.Y-gtx.Dp(24)))
	defer op.Offset(image.Pt(x, gtx.Dp(12))).Push(gtx.Ops).Pop()
	st := oracleStyles[a.cfg.OracleStyle%len(oracleStyles)]
	layout.Flex{Axis: layout.Vertical}.Layout(cg,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, label(th, 11.5, "ПРЕДСКАЗАНИЕ ДНЯ", cGold, font.SemiBold).Layout),
				layout.Rigid(func(gtx C) D { return o.prev.Layout(gtx, th, "‹", btnChip, false) }),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: 6, Right: 6}.Layout(gtx, label(th, 11.5, st.name+" · "+itoa(a.cfg.OracleStyle%len(oracleStyles)+1)+"/4", cMuted, font.Medium).Layout)
				}),
				layout.Rigid(func(gtx C) D { return o.next.Layout(gtx, th, "›", btnChip, false) }),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				if !revealed {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(label(th, 14, st.prompt, cText, font.Normal).Layout),
						layout.Rigid(layout.Spacer{Height: 8}.Layout),
						layout.Rigid(label(th, 12.5, "Нажмите, чтобы узнать →", alpha(cGoldLight, 0.6+0.4*hv), font.Medium).Layout),
					)
				}
				txt := oracle.Phrases[a.cfg.OracleIdx%len(oracle.Phrases)]
				l := label(th, 16, "«"+txt+"»", alpha(cGoldLight, rv), font.Medium)
				l.MaxLines = 4
				return l.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			msg := "Одно предсказание в день, новое открывается в 9:00"
			if revealed {
				when := "завтра"
				if n := oracle.Next(time.Now()); n.Day() == time.Now().Day() {
					when = "сегодня"
				}
				msg = "Следующее предсказание — " + when + " в 9:00"
			}
			return label(th, 11.5, msg, cMuted, font.Normal).Layout(gtx)
		}),
	)
	return D{Size: size}
}

func smooth(p float32) float32 {
	p = clamp01(p)
	return p * p * (3 - 2*p)
}

// --- рисование объектов ---

type oracleKit struct {
	gtx    C
	s      float32 // сторона квадрата
	t      float32 // ход анимации 0..1
	moving bool
	rv     float32 // насколько открыто
	hv     float32 // наведение
}

func (k oracleKit) circle(c f32.Point, r float32, col color.NRGBA) {
	rr := image.Rect(int(c.X-r), int(c.Y-r), int(c.X+r), int(c.Y+r))
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

func (k oracleKit) rect(x0, y0, x1, y1 float32, col color.NRGBA) {
	k.poly([]f32.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}, col)
}

func (k oracleKit) dp(v float32) float32 { return float32(k.gtx.Dp(1)) * v }

// glowAt — мягкое золотое сияние из нескольких полупрозрачных кругов.
func (k oracleKit) glowAt(c f32.Point, r, a float32) {
	for i := 5; i >= 1; i-- {
		f := float32(i) / 5
		k.circle(c, r*f, alpha(cGold, a*0.12*(1.2-f)))
	}
}

// ball — тёмный шар с золотым ободком и окошком, как «шар судьбы».
func (k oracleKit) ball() {
	s := k.s
	shake := float32(0)
	if k.moving {
		shake = float32(math.Sin(float64(k.t)*math.Pi*9)) * (1 - k.t) * s * 0.06
	}
	c := f32.Pt(s/2+shake, s/2)
	R := s * 0.42
	k.glowAt(c, R*1.25, 0.6*k.rv+0.25*k.hv)
	k.circle(c, R, rgb(0x15120D))
	k.circle(f32.Pt(c.X-R*0.18, c.Y-R*0.2), R*0.72, alpha(rgb(0x2A251C), 0.55))
	k.ring(c, R, k.dp(2), alpha(cGold, 0.85))
	k.circle(f32.Pt(c.X-R*0.45, c.Y-R*0.45), R*0.11, alpha(rgb(0xFFFFFF), 0.1))
	// окошко
	w := R * 0.48
	k.circle(c, w, rgb(0x080706))
	k.circle(c, w, alpha(cGold, 0.28*k.rv))
	k.ring(c, w, k.dp(1.5), alpha(cGold, 0.6))
	strokeHex(k.gtx, c.X, c.Y, w*0.62, k.dp(1.5), 1, alpha(cGold, 0.7*(1-k.rv)+0.2))
	if k.rv > 0 {
		h := w * 0.62
		k.poly([]f32.Point{{X: c.X, Y: c.Y - h*0.7}, {X: c.X + h*0.62, Y: c.Y + h*0.42}, {X: c.X - h*0.62, Y: c.Y + h*0.42}}, alpha(cGoldLight, k.rv))
	}
}

// cookie — золотое печенье, которое разламывается, и записка внутри.
func (k oracleKit) cookie(revealed bool) {
	s := k.s
	c := f32.Pt(s/2, s/2+s*0.05)
	open := float32(0)
	if revealed {
		open = smooth((k.t - 0.35) / 0.35)
	}
	wiggle := float32(0)
	if k.moving && k.t < 0.4 {
		wiggle = float32(math.Sin(float64(k.t)*math.Pi*12)) * 0.08 * (1 - k.t/0.4)
	}
	k.glowAt(c, s*0.5, 0.5*k.rv+0.2*k.hv)
	// записка
	if open > 0 {
		pw, ph := s*0.86*open, s*0.13
		k.rect(c.X-pw/2, c.Y-ph/2, c.X+pw/2, c.Y+ph/2, rgb(0xF4EBDA))
		if open > 0.6 {
			a := (open - 0.6) / 0.4
			k.rect(c.X-pw*0.32, c.Y-k.dp(1), c.X+pw*0.32, c.Y+k.dp(1), alpha(cGoldDeep, a))
		}
	}
	R := s * 0.36
	// половинка: «D» на своей стороне, по линии разлома — зубчики
	half := func(side float32) {
		dx := side * R * 0.5 * open
		ang := side*0.3*open + wiggle
		pivot := f32.Pt(c.X, c.Y+R*0.7)
		defer op.Affine(f32.Affine2D{}.Offset(f32.Pt(dx, 0)).Rotate(f32.Pt(pivot.X+dx, pivot.Y), ang)).Push(k.gtx.Ops).Pop()
		shape := func(rx, ry, cy float32) []f32.Point {
			var pts []f32.Point
			for i := 0; i <= 18; i++ {
				a := -math.Pi/2 + math.Pi*float64(i)/18
				pts = append(pts, f32.Pt(c.X+side*rx*float32(math.Cos(a)), cy+ry*float32(math.Sin(a))))
			}
			for i := 5; i >= 1; i-- { // зубчатый край разлома
				y := cy - ry + 2*ry*float32(i)/6
				pts = append(pts, f32.Pt(c.X+side*k.dp(float32(1+2*(i%2))), y))
			}
			return pts
		}
		k.poly(shape(R, R*0.8, c.Y), rgb(0xB9823A))
		k.poly(shape(R*0.86, R*0.6, c.Y-R*0.12), rgb(0xD9A85C))
		k.poly(shape(R*0.5, R*0.3, c.Y-R*0.25), alpha(cGoldLight, 0.6))
	}
	half(-1)
	half(1)
}

// key — золотой ключ входит в скважину и поворачивается.
func (k oracleKit) key(revealed bool) {
	s := k.s
	// дверная накладка
	pw, ph := s*0.5, s*0.8
	px, py := s*0.28-pw/2, s/2-ph/2
	kc := f32.Pt(px+pw/2, py+ph*0.42)
	k.glowAt(kc, s*0.45, 0.55*k.rv+0.2*k.hv)
	fillRRect(k.gtx, image.Rect(int(px), int(py), int(px+pw), int(py+ph)), int(s*0.08), rgb(0x1A1610))
	strokeRRect(k.gtx, image.Rect(int(px), int(py), int(px+pw), int(py+ph)), int(s*0.08), k.dp(1.5), alpha(cGold, 0.8))
	k.circle(kc, s*0.055, rgb(0x050403))
	k.poly([]f32.Point{{X: kc.X - s*0.03, Y: kc.Y}, {X: kc.X + s*0.03, Y: kc.Y}, {X: kc.X + s*0.05, Y: kc.Y + s*0.13}, {X: kc.X - s*0.05, Y: kc.Y + s*0.13}}, rgb(0x050403))
	if k.rv > 0 { // свет из-за двери
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			c, sn := float32(math.Cos(a)), float32(math.Sin(a))
			k.poly([]f32.Point{
				{X: kc.X + c*s*0.12, Y: kc.Y + sn*s*0.12}, {X: kc.X + c*s*0.2 - sn*k.dp(1.5), Y: kc.Y + sn*s*0.2 + c*k.dp(1.5)},
				{X: kc.X + c*s*0.2 + sn*k.dp(1.5), Y: kc.Y + sn*s*0.2 - c*k.dp(1.5)},
			}, alpha(cGoldLight, 0.7*k.rv))
		}
	}
	slide, turn := float32(1), float32(0)
	if revealed {
		turn = smooth((k.t - 0.45) / 0.35)
		if k.moving {
			slide = smooth(k.t / 0.4)
		}
	} else {
		slide = 0.6
	}
	L := s * 0.62
	off := (1 - slide) * s * 0.3
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(0, 0), turn*math.Pi/2).Offset(f32.Pt(kc.X+off, kc.Y))).Push(k.gtx.Ops).Pop()
	w := s * 0.055
	k.rect(-L*0.05, -w/2, L*0.62, w/2, cGold)
	k.rect(-L*0.02, w/2, L*0.08, w/2+s*0.07, cGold)
	k.rect(L*0.14, w/2, L*0.22, w/2+s*0.05, cGold)
	bow := f32.Pt(L*0.78, 0)
	k.circle(bow, L*0.2, cGold)
	k.circle(bow, L*0.09, rgb(0x15120D))
	k.circle(f32.Pt(bow.X-L*0.07, bow.Y-L*0.08), L*0.05, alpha(rgb(0xFFFFFF), 0.25))
}

// hex — фирменный гексагон крутится и показывает звезду дня.
func (k oracleKit) hex(revealed bool) {
	s := k.s
	c := f32.Pt(s/2, s/2)
	R := s * 0.4
	ang, sc := float32(0), float32(1)
	if k.moving {
		e := easeOut(k.t)
		ang = e * math.Pi * 3
		sc = 1 + 0.15*float32(math.Sin(float64(k.t)*math.Pi))
	}
	k.glowAt(c, R*1.3, 0.6*k.rv+0.25*k.hv)
	defer op.Affine(f32.Affine2D{}.Scale(c, f32.Pt(sc, sc)).Rotate(c, ang)).Push(k.gtx.Ops).Pop()
	paint.FillShape(k.gtx.Ops, rgb(0x16130E), clip.Outline{Path: hexPath(k.gtx, c.X, c.Y, R, 1)}.Op())
	strokeHex(k.gtx, c.X, c.Y, R, k.dp(3), 1, cGold)
	strokeHex(k.gtx, c.X, c.Y, R*0.78, k.dp(1), 1, alpha(cGold, 0.5))
	if revealed && k.rv > 0 {
		var pts []f32.Point
		for i := 0; i < 10; i++ {
			r := R * 0.48
			if i%2 == 1 {
				r = R * 0.2
			}
			a := -math.Pi/2 + float64(i)*math.Pi/5
			pts = append(pts, f32.Pt(c.X+r*float32(math.Cos(a)), c.Y+r*float32(math.Sin(a))))
		}
		k.poly(pts, alpha(cGoldLight, k.rv))
	} else {
		k.circle(c, R*0.1, alpha(cGold, 0.7))
	}
}
