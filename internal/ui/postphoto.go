package ui

import (
	"fmt"
	"math"

	"gioui.org/layout"

	"postheader/internal/platform"
	"postheader/internal/render"
)

// Размер текста вручную и фото внутри поста.

// updateTextSize — кнопки − Авто +. Первое нажатие ± берёт за основу
// кегль, который программа подобрала сама.
func (a *App) updateTextSize(gtx C) bool {
	old := a.textSize
	cur := a.textSize
	if cur == 0 {
		cur = 40
		for i, v := range a.variants {
			if i == 0 || v.base < cur {
				cur = v.base
			}
		}
		cur = math.Round(cur/2) * 2
	}
	if a.sizeBtns[0].Clicked(gtx) {
		a.textSize = math.Max(16, cur-2)
	}
	if a.sizeBtns[1].Clicked(gtx) {
		a.textSize = 0
	}
	if a.sizeBtns[2].Clicked(gtx) {
		a.textSize = math.Min(64, cur+2)
	}
	return a.textSize != old
}

func (a *App) layoutTextSize(gtx C) D {
	mid := "Авто"
	if a.textSize > 0 {
		mid = fmt.Sprintf("%.0f → авто", a.textSize)
	}
	return group(gtx, a.th, "Размер текста", func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return a.sizeBtns[0].Layout(gtx, a.th, "−  мельче", btnChip, false) }),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx C) D { return a.sizeBtns[1].Layout(gtx, a.th, mid, btnChip, a.textSize == 0) }),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx C) D { return a.sizeBtns[2].Layout(gtx, a.th, "+  крупнее", btnChip, false) }),
		)
	})
}

func (a *App) loadInline(path string) {
	img, err := decodeFile(path)
	if err != nil {
		a.notify("Не удалось открыть фото: "+err.Error(), true)
		return
	}
	a.inline.Image, a.inline.Y, a.inlinePath = img, 0, path
	if a.inline.Size == 0 && !a.inline.Bottom {
		a.inline.Size = 1
	}
	a.submit()
}

func (a *App) updateInline(gtx C) bool {
	if a.inlineBtn.Clicked(gtx) {
		go func() {
			p, err := platform.OpenImageFile("Фото для поста")
			a.post(func() {
				if err != nil {
					a.notify(err.Error(), true)
				} else if p != "" {
					a.loadInline(p)
				}
			})
		}()
	}
	if a.inline.Image == nil {
		return false
	}
	old := a.inline
	for i := range a.inlinePos {
		if a.inlinePos[i].Clicked(gtx) {
			a.inline.Bottom = i == 1
		}
	}
	for i := range a.inlineSizes {
		if a.inlineSizes[i].Clicked(gtx) {
			a.inline.Size = i
		}
	}
	if a.inlineMove[0].Clicked(gtx) {
		a.inline.Y = math.Max(-1, a.inline.Y-0.25)
	}
	if a.inlineMove[1].Clicked(gtx) {
		a.inline.Y = math.Min(1, a.inline.Y+0.25)
	}
	if a.inlineMove[2].Clicked(gtx) {
		a.inline, a.inlinePath = render.InlinePhoto{}, ""
		return true
	}
	return a.inline != old
}

func (a *App) layoutInline(gtx C) D {
	pos := 0
	if a.inline.Bottom {
		pos = 1
	}
	return layout.Flex{Alignment: layout.End}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return group(gtx, a.th, "Фото в посте", func(gtx C) D {
				return segmented(gtx, a.th, a.inlinePos[:], []string{"Над текстом", "Под текстом"}, &pos)
			})
		}),
		layout.Rigid(layout.Spacer{Width: 22}.Layout),
		layout.Rigid(func(gtx C) D {
			return group(gtx, a.th, "Высота", func(gtx C) D {
				return segmented(gtx, a.th, a.inlineSizes[:], []string{"Небольшое", "Среднее", "Крупное"}, &a.inline.Size)
			})
		}),
		layout.Rigid(layout.Spacer{Width: 22}.Layout),
		layout.Rigid(func(gtx C) D {
			return group(gtx, a.th, "Кадр", func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.inlineMove[0].Layout(gtx, a.th, "↑", btnChip, false) }),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(func(gtx C) D { return a.inlineMove[1].Layout(gtx, a.th, "↓", btnChip, false) }),
				)
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D { return a.inlineMove[2].Layout(gtx, a.th, "Убрать фото", btnGhost, false) })
		}),
	)
}
