package ui

import (
	"image"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"postheader/internal/render"
)

// welcome — знакомство при первом запуске: имя и телефон сотрудника.
// Телефон потом сам подставляется в посты, имя — в приветствие.
type welcome struct {
	open        bool
	appear      timeline
	closing     bool
	closeAt     time.Time
	name, phone widget.Editor
	nameF, telF ease
	start, skip Button
}

func (wl *welcome) show(a *App) {
	wl.open = true
	wl.name.SingleLine, wl.phone.SingleLine = true, true
	wl.name.Submit, wl.phone.Submit = true, true
	wl.name.SetText(a.cfg.UserName)
	wl.phone.SetText(a.cfg.UserPhone)
}

func (wl *welcome) finish(gtx C, a *App, save bool) {
	if save {
		a.setProfile(wl.name.Text(), wl.phone.Text())
	}
	a.cfg.Welcomed = true
	a.saveConfig()
	wl.closing, wl.closeAt = true, gtx.Now
}

func (wl *welcome) layout(gtx C, a *App) {
	if !wl.open {
		return
	}
	submitted := false
	for _, ed := range []*widget.Editor{&wl.name, &wl.phone} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.SubmitEvent); ok {
				submitted = true
			}
		}
	}
	if !wl.closing && (wl.start.Clicked(gtx) || submitted) {
		wl.finish(gtx, a, true)
	}
	if !wl.closing && wl.skip.Clicked(gtx) {
		wl.finish(gtx, a, false)
	}

	p := wl.appear.at(gtx, 0, 450*time.Millisecond)
	if wl.closing {
		q := float32(gtx.Now.Sub(wl.closeAt)) / float32(300*time.Millisecond)
		if q >= 1 {
			wl.open = false
			return
		}
		gtx.Execute(op.InvalidateCmd{})
		p = 1 - easeOut(q)
	}

	paint.FillShape(gtx.Ops, alpha(cBg, 0.92*p), clip.Rect{Max: gtx.Constraints.Max}.Op())
	// перехват кликов под окном знакомства
	var blocker widget.Clickable
	blocker.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })

	w := gtx.Dp(480)
	m := op.Record(gtx.Ops)
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y)}
	dims := layout.UniformInset(32).Layout(cgtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(label(a.th, 24, "Добро пожаловать!", cGold, font.Bold).Layout),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(label(a.th, 13.5, "Представьтесь, пожалуйста. Ваш телефон будет сам подставляться в посты, его можно поменять для любого поста.", cMuted, font.Normal).Layout),
			layout.Rigid(layout.Spacer{Height: 22}.Layout),
			layout.Rigid(label(a.th, 11.5, "ВАШЕ ИМЯ", cGold, font.SemiBold).Layout),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx C) D {
				return fieldFrame(gtx, gtx.Focused(&wl.name), &wl.nameF, editorStyle(a.th, &wl.name, "Например, Мария").Layout)
			}),
			layout.Rigid(layout.Spacer{Height: 14}.Layout),
			layout.Rigid(label(a.th, 11.5, "ВАШ РАБОЧИЙ ТЕЛЕФОН", cGold, font.SemiBold).Layout),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx C) D {
				return fieldFrame(gtx, gtx.Focused(&wl.phone), &wl.telF, editorStyle(a.th, &wl.phone, "+7 920 000-00-00").Layout)
			}),
			layout.Rigid(func(gtx C) D {
				f := render.FormatPhone(wl.phone.Text())
				if f == "" || f == strings.TrimSpace(wl.phone.Text()) {
					return D{}
				}
				return layout.Inset{Top: 6}.Layout(gtx, label(a.th, 12.5, "На картинке: "+f, alpha(cGold, 0.85), font.Medium).Layout)
			}),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return wl.start.Layout(gtx, a.th, "Начать работу", btnPrimary, false) }),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(func(gtx C) D { return wl.skip.Layout(gtx, a.th, "Позже", btnGhost, false) }),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 14}.Layout),
			layout.Rigid(label(a.th, 11.5, "Имя и телефон можно поменять в Настройках.", cMuted, font.Normal).Layout),
		)
	})
	call := m.Stop()

	x := (gtx.Constraints.Max.X - dims.Size.X) / 2
	y := (gtx.Constraints.Max.Y-dims.Size.Y)/2 + int(float32(gtx.Dp(16))*(1-p))
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	r := image.Rectangle{Max: dims.Size}
	rad := gtx.Dp(18)
	fillGrad(gtx, r, rad, rgb(0x1A1712), rgb(0x0F0E0B))
	strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), alpha(cGold, 0.55))
	call.Add(gtx.Ops)
	if p < 1 {
		fillRRect(gtx, r, rad, alpha(cBg, 1-p))
	}
}
