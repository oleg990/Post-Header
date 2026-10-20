package ui

import (
	"image"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"postheader/internal/fontscan"
	"postheader/internal/platform"
)

// settingsPanel выезжает справа поверх главного экрана.
type settingsPanel struct {
	open     bool
	slide    ease
	backdrop widget.Clickable

	address      widget.Editor
	addressFocus ease
	logoBtn      Button
	logoReset    Button
	fontPrev     Button
	fontNext     Button
	dirBtn       Button
	doneBtn      Button
	list         widget.List

	families []fontscan.Family
	scanning bool
	logoOp   paint.ImageOp
	logoSrc  image.Image
}

func (s *settingsPanel) show(gtx C, a *App) {
	s.open = true
	s.list.Axis = layout.Vertical
	if s.families == nil && !s.scanning {
		s.scanning = true
		go func() {
			fams := fontscan.Scan(platform.FontDirs())
			a.post(func() { s.families, s.scanning = fams, false })
		}()
	}
}

func (s *settingsPanel) fontIndex(a *App) int {
	for i, f := range s.families {
		if f.Regular == a.cfg.FontPath {
			return i + 1
		}
	}
	return 0 // встроенный Inter
}

func (s *settingsPanel) setFont(a *App, idx int) {
	n := len(s.families) + 1
	idx = (idx%n + n) % n
	if idx == 0 {
		a.cfg.FontName, a.cfg.FontPath, a.cfg.FontBold = "", "", ""
	} else {
		f := s.families[idx-1]
		a.cfg.FontName, a.cfg.FontPath, a.cfg.FontBold = f.Name, f.Regular, f.Bold
	}
	a.submit()
}

func (s *settingsPanel) update(gtx C, a *App) {
	for {
		ev, ok := s.address.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			a.cfg.Address = s.address.Text()
			a.dirty, a.dirtyAt = true, gtx.Now
		}
	}
	if s.logoBtn.Clicked(gtx) {
		go func() {
			p, err := platform.OpenImageFile("Логотип (лучше PNG с прозрачным фоном)")
			a.post(func() {
				if err != nil {
					a.notify(err.Error(), true)
				} else if p != "" {
					a.cfg.LogoPath = p
					a.loadLogo()
					a.submit()
				}
			})
		}()
	}
	if s.logoReset.Clicked(gtx) {
		a.cfg.LogoPath = ""
		a.loadLogo()
		a.submit()
	}
	if s.fontPrev.Clicked(gtx) {
		s.setFont(a, s.fontIndex(a)-1)
	}
	if s.fontNext.Clicked(gtx) {
		s.setFont(a, s.fontIndex(a)+1)
	}
	if s.dirBtn.Clicked(gtx) {
		platform.OpenFolder(a.outDir())
	}
	if s.doneBtn.Clicked(gtx) || s.backdrop.Clicked(gtx) {
		s.open = false
		a.saveConfig()
	}
}

func (s *settingsPanel) layout(gtx C, a *App) {
	p := s.slide.to(gtx, boolf(s.open), 11)
	if p < 0.002 {
		return
	}
	s.update(gtx, a)
	if s.logoSrc != a.logo && a.logo != nil {
		s.logoSrc, s.logoOp = a.logo, paint.NewImageOp(a.logo)
	}

	// затемнение; клик по нему закрывает панель
	paint.FillShape(gtx.Ops, alpha(rgb(0x000000), 0.8*p), clip.Rect{Max: gtx.Constraints.Max}.Op())
	s.backdrop.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })

	w := gtx.Dp(460)
	x := gtx.Constraints.Max.X - int(float32(w)*easeOut(p))
	defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(w, gtx.Constraints.Max.Y))
	r := image.Rectangle{Max: gtx.Constraints.Max}
	fillGrad(gtx, r, 0, rgb(0x17150F), rgb(0x0E0D0A))
	paint.FillShape(gtx.Ops, alpha(cGold, 0.5), clip.Rect{Max: image.Pt(gtx.Dp(1), r.Max.Y)}.Op())
	// панель перехватывает клики, чтобы они не закрывали её через затемнение
	var blocker widget.Clickable
	blocker.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })

	th := a.th
	section := func(title string, body layout.Widget) layout.Widget {
		return func(gtx C) D {
			return layout.Inset{Bottom: 22}.Layout(gtx, func(gtx C) D {
				return group(gtx, th, title, body)
			})
		}
	}
	rows := []layout.Widget{
		func(gtx C) D {
			return layout.Inset{Bottom: 24}.Layout(gtx, label(th, 22, "Настройки", cGold, font.Bold).Layout)
		},
		section("Логотип в шапке", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					h := gtx.Dp(84)
					gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, h))
					fillRRect(gtx, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(10), rgb(0x0A0908))
					strokeRRect(gtx, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(10), float32(gtx.Dp(1)), cLine)
					return layout.UniformInset(12).Layout(gtx, widget.Image{Src: s.logoOp, Fit: widget.Contain, Position: layout.W}.Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return s.logoBtn.Layout(gtx, th, "Выбрать файл…", btnGhost, false) }),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx C) D {
							return s.logoReset.Layout(gtx, th, "Встроенный «Адреса»", btnChip, a.cfg.LogoPath == "")
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Rigid(label(th, 12, "Лучше всего PNG с прозрачным фоном. JPG на тёмном фоне тоже подойдёт: фон уберётся сам.", cMuted, font.Normal).Layout),
			)
		}),
		section("Адрес в шапке", func(gtx C) D {
			return fieldFrame(gtx, gtx.Focused(&s.address), &s.addressFocus, editorStyle(th, &s.address, "мкр. Степной 18").Layout)
		}),
		section("Шрифт постов", func(gtx C) D {
			name := "Inter (встроенный)"
			if a.cfg.FontName != "" {
				name = a.cfg.FontName
			}
			hint := "Шрифты Windows с кириллицей. Символы, которых нет в шрифте (например ₽), возьмутся из Inter."
			if s.scanning {
				hint = "Ищу шрифты на компьютере…"
			} else if s.families != nil {
				hint = itoa(len(s.families)) + " шрифтов с кириллицей. " + hint
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return s.fontPrev.Layout(gtx, th, "‹", btnChip, false) }),
						layout.Flexed(1, func(gtx C) D {
							return layout.Center.Layout(gtx, label(th, 15, name, cText, font.SemiBold).Layout)
						}),
						layout.Rigid(func(gtx C) D { return s.fontNext.Layout(gtx, th, "›", btnChip, false) }),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Rigid(label(th, 12, hint, cMuted, font.Normal).Layout),
			)
		}),
		section("Папка для готовых постов", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(label(th, 13, a.outDir(), cText, font.Normal).Layout),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx C) D { return s.dirBtn.Layout(gtx, th, "Открыть папку", btnGhost, false) }),
			)
		}),
		section("О программе", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(label(th, 13, Signature+" · версия "+Version, alpha(cGold, 0.8), font.Medium).Layout),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Rigid(label(th, 11.5, "Эмодзи: Twemoji (CC-BY 4.0) и Noto Color Emoji (OFL). Шрифт: Inter (OFL).", cMuted, font.Normal).Layout),
			)
		}),
		func(gtx C) D { return s.doneBtn.Layout(gtx, th, "Готово", btnPrimary, false) },
	}
	layout.Inset{Top: 28, Left: 28, Right: 28, Bottom: 20}.Layout(gtx, func(gtx C) D {
		return s.list.Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
	})
	_ = time.Now
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
