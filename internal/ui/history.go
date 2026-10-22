package ui

import (
	"image"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"postheader/internal/history"
	"postheader/internal/render"
)

// История: последние 10 постов, которые сохранили или скопировали.
// Любой можно открыть — вернутся текст, фото и все настройки.

type historyPanel struct {
	open     bool
	slide    ease
	backdrop widget.Clickable
	list     widget.List
	closeBtn Button

	entries []history.Entry
	thumbs  map[string]*paint.ImageOp
	openBtn []widget.Clickable
	delBtn  []Button
	hover   []ease
}

func (h *historyPanel) show(a *App) {
	h.open = true
	h.list.Axis = layout.Vertical
	h.entries = history.Load()
	if h.thumbs == nil {
		h.thumbs = map[string]*paint.ImageOp{}
	}
	for _, e := range h.entries {
		if _, ok := h.thumbs[e.ID]; ok {
			continue
		}
		if img, err := decodeFile(history.ThumbPath(e.ID)); err == nil {
			op := paint.NewImageOp(img)
			h.thumbs[e.ID] = &op
		} else {
			h.thumbs[e.ID] = nil
		}
	}
}

// snapshot — текущий пост для истории.
func (a *App) snapshot() history.Entry {
	e := history.Entry{
		Text: a.text.Text(), Phone: a.phone.Text(), PhoneName: a.phoneName.Text(), PhoneAt: a.phoneAt,
		Format: a.format, Background: a.bgKind, Overflow: a.overflow, Align: a.align, TextSize: a.textSize,
		Icons: map[int]int{},
	}
	for k, v := range a.icons {
		e.Icons[k] = int(v)
	}
	if a.bgKind == 3 {
		e.PhotoPath = a.photoPath
		e.PhotoX, e.PhotoY, e.PhotoZoom = a.photoAdj.X, a.photoAdj.Y, a.photoAdj.Zoom
		e.PhotoSharp, e.PhotoLight = a.photoAdj.Sharp, a.photoAdj.Light
	}
	if a.inline.Image != nil {
		e.InlinePath, e.InlineSize, e.InlineBottom, e.InlineY = a.inlinePath, a.inline.Size, a.inline.Bottom, a.inline.Y
	}
	return e
}

// remember сохраняет текущий пост в историю (вызывается при сохранении
// и копировании картинки).
func (a *App) remember(v variant) {
	if strings.TrimSpace(a.text.Text()) == "" {
		return
	}
	e := a.snapshot()
	var thumb image.Image
	if len(v.previews) > 0 {
		thumb = v.previews[0]
	}
	go history.Add(e, thumb)
}

// restore открывает пост из истории.
func (a *App) restore(e history.Entry) {
	a.text.SetText(e.Text)
	a.phone.SetText(e.Phone)
	a.phoneName.SetText(e.PhoneName)
	a.phoneAt, a.format, a.overflow, a.align, a.textSize = e.PhoneAt, e.Format, e.Overflow, e.Align, e.TextSize
	a.icons = map[int]render.IconKind{}
	for k, v := range e.Icons {
		a.icons[k] = render.IconKind(v)
	}
	a.bgKind = e.Background
	missing := []string{}
	if e.Background == 3 {
		a.bgKind = 0
		if img, err := decodeFile(e.PhotoPath); err == nil {
			a.photo, a.photoName, a.photoPath, a.bgKind = img, filepath.Base(e.PhotoPath), e.PhotoPath, 3
			a.photoAdj = render.PhotoAdjust{X: e.PhotoX, Y: e.PhotoY, Zoom: e.PhotoZoom, Sharp: e.PhotoSharp, Light: e.PhotoLight}
		} else {
			missing = append(missing, "фото фона")
		}
	}
	a.inline, a.inlinePath = render.InlinePhoto{}, ""
	if e.InlinePath != "" {
		if img, err := decodeFile(e.InlinePath); err == nil {
			a.inline = render.InlinePhoto{Image: img, Size: e.InlineSize, Bottom: e.InlineBottom, Y: e.InlineY}
			a.inlinePath = e.InlinePath
		} else {
			missing = append(missing, "фото в посте")
		}
	}
	a.submit()
	if len(missing) > 0 {
		a.notify("Пост открыт, но не нашлось "+strings.Join(missing, " и ")+": файл переместили или удалили", true)
	} else {
		a.notify("Пост открыт из истории", false)
	}
}

func (h *historyPanel) update(gtx C, a *App) {
	if len(h.openBtn) < len(h.entries) {
		h.openBtn = make([]widget.Clickable, len(h.entries))
		h.delBtn = make([]Button, len(h.entries))
		h.hover = make([]ease, len(h.entries))
	}
	for i := range h.entries {
		if h.openBtn[i].Clicked(gtx) {
			a.restore(h.entries[i])
			h.open = false
			return
		}
		if h.delBtn[i].Clicked(gtx) {
			es, _ := history.Remove(h.entries[i].ID)
			h.entries = es
			return
		}
	}
	if h.closeBtn.Clicked(gtx) || h.backdrop.Clicked(gtx) {
		h.open = false
	}
}

func (h *historyPanel) layout(gtx C, a *App) {
	p := h.slide.to(gtx, boolf(h.open), 11)
	if p < 0.002 {
		return
	}
	h.update(gtx, a)

	paint.FillShape(gtx.Ops, alpha(rgb(0x000000), 0.6*p), clip.Rect{Max: gtx.Constraints.Max}.Op())
	h.backdrop.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })

	w := gtx.Dp(520)
	x := gtx.Constraints.Max.X - int(float32(w)*easeOut(p))
	defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(w, gtx.Constraints.Max.Y))
	r := image.Rectangle{Max: gtx.Constraints.Max}
	fillGrad(gtx, r, 0, rgb(0x17150F), rgb(0x0E0D0A))
	paint.FillShape(gtx.Ops, alpha(cGold, 0.5), clip.Rect{Max: image.Pt(gtx.Dp(1), r.Max.Y)}.Op())
	var blocker widget.Clickable
	blocker.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })

	th := a.th
	layout.Inset{Top: 28, Left: 28, Right: 28, Bottom: 20}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, label(th, 22, "История постов", cGold, font.Bold).Layout),
					layout.Rigid(func(gtx C) D { return h.closeBtn.Layout(gtx, th, "Закрыть", btnGhost, false) }),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(label(th, 12.5, "Последние 10 постов, которые вы сохранили или скопировали. Нажмите на пост, чтобы открыть и доработать.", cMuted, font.Normal).Layout),
			layout.Rigid(layout.Spacer{Height: 16}.Layout),
			layout.Flexed(1, func(gtx C) D {
				if len(h.entries) == 0 {
					return label(th, 13.5, "Пока пусто. Пост попадёт сюда, когда вы его сохраните или скопируете.", cText, font.Normal).Layout(gtx)
				}
				return material.List(th, &h.list).Layout(gtx, len(h.entries), func(gtx C, i int) D {
					return layout.Inset{Bottom: 10, Right: 6}.Layout(gtx, func(gtx C) D { return h.layoutEntry(gtx, a, i) })
				})
			}),
		)
	})
}

func (h *historyPanel) layoutEntry(gtx C, a *App, i int) D {
	th := a.th
	e := h.entries[i]
	hv := h.hover[i].to(gtx, boolf(h.openBtn[i].Hovered()), 12)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	m := op.Record(gtx.Ops)
	d := h.openBtn[i].Layout(gtx, func(gtx C) D {
		return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					sz := image.Pt(gtx.Dp(76), gtx.Dp(96))
					gtx.Constraints = layout.Exact(sz)
					fillRRect(gtx, image.Rectangle{Max: sz}, gtx.Dp(6), rgb(0x0A0908))
					if t := h.thumbs[e.ID]; t != nil {
						defer clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(6)).Push(gtx.Ops).Pop()
						widget.Image{Src: *t, Fit: widget.Cover, Position: layout.N}.Layout(gtx)
					}
					return D{Size: sz}
				}),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(label(th, 11.5, e.Time.Format("02.01.2006 15:04")+" · "+shortFormat(e.Format), alpha(cGold, 0.8), font.Medium).Layout),
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(func(gtx C) D {
							l := label(th, 14, e.Title(), cText, font.SemiBold)
							l.MaxLines = 2
							return l.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(func(gtx C) D {
							l := label(th, 12, preview2(e.Text), cMuted, font.Normal)
							l.MaxLines = 2
							return l.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx C) D { return h.delBtn[i].Layout(gtx, th, "Удалить", btnChip, false) }),
			)
		})
	})
	call := m.Stop()
	rr := image.Rectangle{Max: d.Size}
	rad := gtx.Dp(12)
	fillRRect(gtx, rr, rad, mix(cPanel, cPanelHi, hv))
	strokeRRect(gtx, rr, rad, float32(gtx.Dp(1)), mix(cLine, alpha(cGold, 0.7), hv))
	call.Add(gtx.Ops)
	return d
}

// preview2 — текст после первой строки, одной строкой.
func preview2(s string) string {
	lines := strings.Fields(strings.ReplaceAll(s, "\n", " "))
	t := strings.Join(lines, " ")
	if r := []rune(t); len(r) > 160 {
		t = string(r[:160]) + "…"
	}
	return t
}

func clampIdx(i, n int) int {
	if i < 0 || i >= n {
		return 0
	}
	return i
}

// shortFormat — «Сторис / вертикальный 9:16 (…)» → «Сторис / вертикальный 9:16».
func shortFormat(i int) string {
	n := render.Formats[clampIdx(i, len(render.Formats))].Name
	if j := strings.Index(n, " ("); j > 0 {
		n = n[:j]
	}
	return n
}
