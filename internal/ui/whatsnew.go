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
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// newsDialog — при запуске, если вышла новая версия, показывает, что в ней
// изменилось (текст из описания релиза, файл NOTES.md), и предлагает обновиться.
type newsDialog struct {
	open       bool
	appear     timeline
	closing    bool
	closeAt    time.Time
	tag, notes string
	info       bool // уже обновились: только «Понятно», без кнопки обновления
	now, later Button
	list       widget.List
}

func (n *newsDialog) show(tag, notes string) {
	n.open, n.closing, n.info = true, false, false
	n.appear = timeline{}
	n.tag, n.notes = tag, strings.TrimSpace(notes)
	n.list.Axis = layout.Vertical
}

// newsLine — строка описания: подзаголовок «Добавил:», пункт «- …» или текст.
type newsLine struct {
	text         string
	head, bullet bool
}

func parseNotes(s string) []newsLine {
	var out []newsLine
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "#"))
		switch {
		case l == "":
		case strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") || strings.HasPrefix(l, "• "):
			out = append(out, newsLine{text: strings.TrimSpace(string([]rune(l)[2:])), bullet: true})
		case strings.HasSuffix(l, ":"):
			out = append(out, newsLine{text: l, head: true})
		default:
			out = append(out, newsLine{text: l})
		}
	}
	return out
}

// showUpdated — окно после обновления: что появилось в программе.
func (n *newsDialog) showUpdated(notes string) {
	n.show(Version, notes)
	n.info = true
}

func (n *newsDialog) layout(gtx C, a *App) {
	if !n.open {
		return
	}
	if !n.closing && n.now.Clicked(gtx) {
		if !n.info {
			a.upd.install(a)
		}
		n.closing, n.closeAt = true, gtx.Now
	}
	if !n.closing && n.later.Clicked(gtx) {
		n.closing, n.closeAt = true, gtx.Now
	}
	p := n.appear.at(gtx, 0, 450*time.Millisecond)
	if n.closing {
		q := float32(gtx.Now.Sub(n.closeAt)) / float32(300*time.Millisecond)
		if q >= 1 {
			n.open = false
			return
		}
		gtx.Execute(op.InvalidateCmd{})
		p = 1 - easeOut(q)
	}
	paint.FillShape(gtx.Ops, alpha(cBg, 0.88*p), clip.Rect{Max: gtx.Constraints.Max}.Op())
	var blocker widget.Clickable
	blocker.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })

	th := a.th
	lines := parseNotes(n.notes)
	if len(lines) == 0 {
		lines = []newsLine{{text: "Олег выпустил новую версию программы."}}
	}
	w := gtx.Dp(540)
	m := op.Record(gtx.Ops)
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, gtx.Constraints.Max.Y-gtx.Dp(80))}
	dims := layout.UniformInset(32).Layout(cgtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(label(th, 12, n.kicker(), alpha(cGold, 0.85), font.SemiBold).Layout),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(label(th, 24, "Что нового", cGold, font.Bold).Layout),
			layout.Rigid(layout.Spacer{Height: 16}.Layout),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.Y = 0
				if mx := gtx.Dp(380); gtx.Constraints.Max.Y > mx {
					gtx.Constraints.Max.Y = mx
				}
				return material.List(th, &n.list).Layout(gtx, len(lines), func(gtx C, i int) D {
					l := lines[i]
					switch {
					case l.head:
						top := 0
						if i > 0 {
							top = 10
						}
						return layout.Inset{Top: unit.Dp(top), Bottom: 6}.Layout(gtx, label(th, 14.5, l.text, cGoldLight, font.SemiBold).Layout)
					case l.bullet:
						return layout.Inset{Bottom: 6, Right: 8}.Layout(gtx, func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(label(th, 14, "–  ", cGold, font.Bold).Layout),
								layout.Flexed(1, label(th, 14, l.text, cText, font.Normal).Layout),
							)
						})
					}
					return layout.Inset{Bottom: 6}.Layout(gtx, label(th, 14, l.text, cText, font.Normal).Layout)
				})
			}),
			layout.Rigid(layout.Spacer{Height: 22}.Layout),
			layout.Rigid(func(gtx C) D {
				if n.info {
					return n.now.Layout(gtx, th, "Понятно", btnPrimary, false)
				}
				txt := "Обновить сейчас"
				if a.upd.loading {
					txt = "Загрузка " + itoa(int(a.upd.progress*100)) + "%"
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return n.now.Layout(gtx, th, txt, btnPrimary, false) }),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(func(gtx C) D { return n.later.Layout(gtx, th, "Позже", btnGhost, false) }),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if n.info {
					return D{}
				}
				return layout.Spacer{Height: 12}.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if n.info {
					return D{}
				}
				return label(th, 11.5, "Обновление займёт меньше минуты, черновик поста сохранится. Обновиться можно и позже — кнопкой вверху окна.", cMuted, font.Normal).Layout(gtx)
			}),
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

func (n *newsDialog) kicker() string {
	if n.info {
		return "ПРОГРАММА ОБНОВЛЕНА"
	}
	return "ОБНОВЛЕНИЕ " + strings.TrimPrefix(n.tag, "v")
}
