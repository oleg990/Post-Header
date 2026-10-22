package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// Фото двигается мышкой прямо на превью: тянуть — сдвиг кадра,
// колёсико — размер. Фото в посте тянется, если нажать на него,
// всё остальное поле двигает фото фона.

type photoDrag struct {
	target  int // 0 — ничего, 1 — фон, 2 — фото в посте
	last    f32.Point
	total   float32
	moved   bool // было перетаскивание: клик-увеличение не срабатывает
	clicked bool // нажали и отпустили, не двигая (в крупном просмотре — закрыть)
	scroll  float32
	hovered bool
}

const (
	dragNone = iota
	dragBg
	dragInline
)

// photoDrag обрабатывает мышь над превью карточки. Возвращает true,
// если на превью есть что двигать (тогда курсор — «рука»).
func (a *App) photoDrag(gtx C, d *photoDrag, c *variantCard, v variant, idx int) bool {
	bgOn := a.bgKind == 3 && a.photo != nil
	inlOn := a.inline.Image != nil && c.page == 0 && !v.inline.Empty()
	if !bgOn && !inlOn || len(v.pages) == 0 {
		return false
	}
	// где лежит картинка внутри поля (вписана по центру)
	area := gtx.Constraints.Max
	pb := v.pages[c.page].Bounds()
	sc := float32(math.Min(float64(area.X)/float64(pb.Dx()), float64(area.Y)/float64(pb.Dy())))
	off := f32.Pt((float32(area.X)-float32(pb.Dx())*sc)/2, (float32(area.Y)-float32(pb.Dy())*sc)/2)
	toPage := func(p f32.Point) image.Point {
		q := p.Sub(off).Mul(1 / sc)
		return image.Pt(int(q.X), int(q.Y))
	}

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:       d,
			Kinds:        pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Scroll | pointer.Enter | pointer.Leave,
			ScrollBounds: image.Rect(0, -1000, 0, 1000),
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter:
			d.hovered = true
		case pointer.Leave:
			d.hovered = false
		case pointer.Press:
			d.last, d.total, d.moved, d.clicked = e.Position, 0, false, false
			d.target = dragNone
			switch {
			case inlOn && toPage(e.Position).In(v.inline):
				d.target = dragInline
			case bgOn:
				d.target = dragBg
			}
			a.dragging = d.target != dragNone
		case pointer.Drag:
			if d.target == dragNone {
				break
			}
			delta := e.Position.Sub(d.last)
			d.last = e.Position
			d.total += abs(delta.X) + abs(delta.Y)
			if d.total > 4 {
				d.moved = true
			}
			a.dragPhoto(d.target, float64(delta.X/sc), float64(delta.Y/sc), pb, v.inline)
			a.submitFast(idx)
		case pointer.Release, pointer.Cancel:
			if e.Kind == pointer.Release && !d.moved {
				d.clicked = true
				gtx.Execute(op.InvalidateCmd{}) // клик обработается в следующем кадре
			}
			if d.target != dragNone && d.moved {
				a.dirty, a.dirtyAt = true, gtx.Now.Add(-350*time.Millisecond) // полный рендер всех вариантов
			}
			d.target = dragNone
			a.dragging = false
		case pointer.Scroll:
			target := dragBg
			if inlOn && toPage(e.Position).In(v.inline) || !bgOn {
				target = dragInline
			}
			if a.scrollPhoto(target, e.Scroll.Y, d) {
				a.dragging = true // показывать быстрые кадры, пока крутят колёсико
				a.submitFast(idx)
				a.dirty, a.dirtyAt = true, gtx.Now
			}
		}
	}

	event.Op(gtx.Ops, d)
	if d.target != dragNone {
		pointer.CursorGrabbing.Add(gtx.Ops)
	} else {
		pointer.CursorGrab.Add(gtx.Ops)
	}
	if d.hovered && d.target == dragNone {
		hint := "Тяните мышкой · колёсико — размер"
		if !bgOn && a.inline.Over {
			hint = "Тяните мышкой, чтобы выбрать кадр"
		}
		layoutHint(gtx, a, hint)
	}
	return true
}

// dragPhoto сдвигает фото на (dx, dy) пикселей картинки.
func (a *App) dragPhoto(target int, dx, dy float64, pb, box image.Rectangle) {
	clamp := func(v float64) float64 { return math.Max(-1, math.Min(v, 1)) }
	switch target {
	case dragBg:
		a.photoAdj.X = clamp(a.photoAdj.X + dx/float64(pb.Dx()))
		a.photoAdj.Y = clamp(a.photoAdj.Y + dy/float64(pb.Dy()))
	case dragInline:
		ib := a.inline.Image.Bounds()
		w, h := float64(box.Dx()), float64(box.Dy())
		s := math.Max(w/float64(ib.Dx()), h/float64(ib.Dy()))
		// тянем вниз — показывается верх снимка, вправо — левый край
		if hidden := float64(ib.Dy())*s - h; hidden > 1 {
			a.inline.Y = clamp(a.inline.Y - 2*dy/hidden)
		}
		if hidden := float64(ib.Dx())*s - w; hidden > 1 {
			a.inline.X = clamp(a.inline.X - 2*dx/hidden)
		}
	}
}

// scrollPhoto: колёсико меняет размер фона плавно, а высоту фото в посте —
// ступенькой на каждый заметный поворот.
func (a *App) scrollPhoto(target int, dy float32, d *photoDrag) bool {
	switch target {
	case dragBg:
		z := a.photoAdj.Zoom
		if z == 0 {
			z = 1
		}
		step := 0.05
		if dy > 0 {
			step = -step
		}
		nz := math.Max(0.4, math.Min(3, math.Round((z+step)*100)/100))
		if nz == z {
			return false
		}
		a.photoAdj.Zoom = nz
		return true
	case dragInline:
		if a.inline.Over { // фото и так на всё поле
			return false
		}
		d.scroll += dy
		if abs(d.scroll) < 40 {
			return false
		}
		n := a.inline.Size
		if d.scroll > 0 {
			n--
		} else {
			n++
		}
		d.scroll = 0
		if n < 0 || n > 2 {
			return false
		}
		a.inline.Size = n
		return true
	}
	return false
}

// layoutHint — подсказка внизу превью.
func layoutHint(gtx C, a *App, txt string) {
	m := op.Record(gtx.Ops)
	cg := gtx
	cg.Constraints.Min = image.Point{}
	dims := layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(cg, label(a.th, 11.5, txt, cText, font.Medium).Layout)
	call := m.Stop()
	x := (gtx.Constraints.Max.X - dims.Size.X) / 2
	y := gtx.Constraints.Max.Y - dims.Size.Y - gtx.Dp(8)
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	r := image.Rectangle{Max: dims.Size}
	paint.FillShape(gtx.Ops, alpha(rgb(0x000000), 0.72), clip.UniformRRect(r, dims.Size.Y/2).Op(gtx.Ops))
	call.Add(gtx.Ops)
}

func (a *App) anyDrag() bool {
	for i := range a.cards {
		if a.cards[i].drag.target != dragNone {
			return true
		}
	}
	return a.zoomDrag.target != dragNone
}
