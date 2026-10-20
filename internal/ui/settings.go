package ui

import (
	"image"
	"strings"

	"gioui.org/f32"
	"gioui.org/widget/material"

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
	fontToggle   Button
	fontOpen     bool
	fontSearch   widget.Editor
	fontSearchF  ease
	fontList     widget.List
	fontItems    []widget.Clickable
	dirBtn       Button
	dirPick      Button
	dirReset     Button
	doneBtn      Button
	list         widget.List

	name, phone   widget.Editor
	nameF, phoneF ease

	families []fontscan.Family
	scanning bool
	logoOp   paint.ImageOp
	logoSrc  image.Image
}

func (s *settingsPanel) show(gtx C, a *App) {
	s.open = true
	s.list.Axis = layout.Vertical
	s.fontList.Axis = layout.Vertical
	s.fontSearch.SingleLine = true
	s.name.SingleLine, s.phone.SingleLine = true, true
	s.name.SetText(a.cfg.UserName)
	s.phone.SetText(a.cfg.UserPhone)
	if s.families == nil && !s.scanning {
		s.scanning = true
		go func() {
			fams := fontscan.Scan(platform.FontDirs())
			a.post(func() { s.families, s.scanning = fams, false })
		}()
	}
}

// setFont: idx 0 — встроенный Inter, иначе семейство из списка.
func (s *settingsPanel) setFont(a *App, f *fontscan.Family) {
	if f == nil {
		a.cfg.FontName, a.cfg.FontPath, a.cfg.FontBold = "", "", ""
	} else {
		a.cfg.FontName, a.cfg.FontPath, a.cfg.FontBold = f.Name, f.Regular, f.Bold
	}
	a.submit()
}

// filtered — шрифты, подходящие под строку поиска (nil — встроенный Inter).
func (s *settingsPanel) filtered() []*fontscan.Family {
	q := strings.ToLower(strings.TrimSpace(s.fontSearch.Text()))
	out := []*fontscan.Family{}
	if q == "" || strings.Contains("inter встроенный", q) {
		out = append(out, nil)
	}
	for i := range s.families {
		if q == "" || strings.Contains(strings.ToLower(s.families[i].Name), q) {
			out = append(out, &s.families[i])
		}
	}
	return out
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
	for _, ed := range []*widget.Editor{&s.name, &s.phone} {
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.ChangeEvent); ok {
				a.setProfile(s.name.Text(), s.phone.Text())
			}
		}
	}
	if s.fontToggle.Clicked(gtx) {
		s.fontOpen = !s.fontOpen
	}
	if s.dirPick.Clicked(gtx) {
		go func() {
			p, err := platform.ChooseFolder("Куда сохранять готовые посты")
			a.post(func() {
				if err != nil {
					a.notify(err.Error(), true)
				} else if p != "" {
					a.cfg.OutDir = p
					a.saveConfig()
				}
			})
		}()
	}
	if s.dirReset.Clicked(gtx) {
		a.cfg.OutDir = ""
		a.saveConfig()
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
	paint.FillShape(gtx.Ops, alpha(rgb(0x000000), 0.45*p), clip.Rect{Max: gtx.Constraints.Max}.Op())
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
		section("Сотрудник", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return fieldFrame(gtx, gtx.Focused(&s.name), &s.nameF, editorStyle(th, &s.name, "Ваше имя").Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx C) D {
					return fieldFrame(gtx, gtx.Focused(&s.phone), &s.phoneF, editorStyle(th, &s.phone, "Рабочий телефон").Layout)
				}),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Rigid(label(th, 12, "Телефон сам подставляется в новые посты.", cMuted, font.Normal).Layout),
			)
		}),
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
		section("Шрифт постов", func(gtx C) D { return s.layoutFonts(gtx, a) }),
		section("Папка для готовых постов", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(label(th, 13, a.outDir(), cText, font.Normal).Layout),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return s.dirPick.Layout(gtx, th, "Выбрать папку…", btnGhost, false) }),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx C) D { return s.dirBtn.Layout(gtx, th, "Открыть", btnChip, false) }),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx C) D {
							if a.cfg.OutDir == "" {
								return D{}
							}
							return s.dirReset.Layout(gtx, th, "По умолчанию", btnChip, false)
						}),
					)
				}),
			)
		}),
		section("О программе", func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(label(th, 13, Signature+" · версия "+Version, alpha(cGold, 0.8), font.Medium).Layout),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx C) D { return a.upd.layoutAbout(gtx, a) }),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(label(th, 11.5, "Эмодзи: Twemoji (CC-BY 4.0) и Noto Color Emoji (OFL). Шрифт: Inter (OFL). Словарь: © 1997–2008 А. И. Лебедев (BSD).", cMuted, font.Normal).Layout),
			)
		}),
		func(gtx C) D { return s.doneBtn.Layout(gtx, th, "Готово", btnPrimary, false) },
	}
	layout.Inset{Top: 28, Left: 28, Right: 28, Bottom: 20}.Layout(gtx, func(gtx C) D {
		// Заголовок и превью закреплены сверху: меняете что-то ниже — сразу видно результат.
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: 16}.Layout(gtx, label(th, 22, "Настройки", cGold, font.Bold).Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: 18}.Layout(gtx, func(gtx C) D { return s.layoutPreview(gtx, a) })
			}),
			layout.Flexed(1, func(gtx C) D {
				return s.list.Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
			}),
		)
	})
}

// layoutPreview — живая миниатюра первого варианта (шапка и начало текста):
// видно, как меняется пост, пока вы правите настройки.
func (s *settingsPanel) layoutPreview(gtx C, a *App) D {
	th := a.th
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := "ТАК ВЫГЛЯДИТ ПОСТ"
			col := cGold
			if a.rendering {
				txt, col = "ОБНОВЛЯЮ…", cGoldLight
			}
			return layout.Inset{Bottom: 8}.Layout(gtx, label(th, 11.5, txt, col, font.SemiBold).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			w := gtx.Constraints.Max.X
			h := gtx.Dp(190)
			r := image.Rectangle{Max: image.Pt(w, h)}
			rad := gtx.Dp(10)
			fillRRect(gtx, r, rad, rgb(0x0A0908))
			if len(a.cards) > 0 && len(a.cards[0].ops) > 0 {
				c := &a.cards[0]
				op0 := c.ops[c.page]
				sz := op0.Size()
				func() {
					defer clip.UniformRRect(r, rad).Push(gtx.Ops).Pop()
					scale := float32(w) / float32(sz.X)
					defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale))).Push(gtx.Ops).Pop()
					op0.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
				}()
				// мягкое затухание снизу: видно, что это верх поста
				fillGradRect(gtx, image.Rect(0, h-gtx.Dp(50), w, h), alpha(cBg, 0), alpha(cBg, 0.9))
			} else {
				func() {
					gtx := gtx
					gtx.Constraints = layout.Exact(r.Max)
					layout.Center.Layout(gtx, label(th, 12.5, "Введите текст поста, и здесь появится превью", cMuted, font.Normal).Layout)
				}()
			}
			strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), alpha(cGold, 0.35))
			return D{Size: r.Max}
		}),
	)
}

// layoutFonts — текущий шрифт; по нажатию раскрывается список с поиском.
func (s *settingsPanel) layoutFonts(gtx C, a *App) D {
	th := a.th
	name := "Inter (встроенный)"
	if a.cfg.FontName != "" {
		name = a.cfg.FontName
	}
	hint := "Символы, которых нет в шрифте (например ₽), возьмутся из Inter."
	if s.scanning {
		hint = "Ищу шрифты на компьютере…"
	} else if s.families != nil {
		hint = itoa(len(s.families)) + " шрифтов с кириллицей. " + hint
	}
	arrow := "· сменить"
	if s.fontOpen {
		arrow = "· свернуть"
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return s.fontToggle.Layout(gtx, th, name+"   "+arrow, btnChip, false)
		}),
	}
	if s.fontOpen {
		items := s.filtered()
		if len(s.fontItems) < len(s.families)+1 {
			s.fontItems = make([]widget.Clickable, len(s.families)+1)
		}
		for i, f := range items {
			if s.fontItems[i].Clicked(gtx) {
				s.setFont(a, f)
			}
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx C) D {
				return fieldFrame(gtx, gtx.Focused(&s.fontSearch), &s.fontSearchF, editorStyle(th, &s.fontSearch, "Поиск шрифта…").Layout)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.Y = gtx.Dp(240)
				gtx.Constraints.Min.Y = 0
				return material.List(th, &s.fontList).Layout(gtx, len(items), func(gtx C, i int) D {
					f := items[i]
					n, sel := "Inter (встроенный)", a.cfg.FontPath == ""
					if f != nil {
						n, sel = f.Name, f.Regular == a.cfg.FontPath
					}
					return s.fontItems[i].Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						m := op.Record(gtx.Ops)
						col := cText
						if sel {
							col = cDark
						}
						d := layout.Inset{Top: 7, Bottom: 7, Left: 10, Right: 10}.Layout(gtx, label(th, 13.5, n, col, font.Normal).Layout)
						call := m.Stop()
						r := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, d.Size.Y)}
						switch {
						case sel:
							fillRRect(gtx, r, gtx.Dp(7), cGold)
						case s.fontItems[i].Hovered():
							fillRRect(gtx, r, gtx.Dp(7), cPanelHi)
						}
						call.Add(gtx.Ops)
						return D{Size: r.Max}
					})
				})
			}),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: 6}.Layout),
		layout.Rigid(label(th, 12, hint, cMuted, font.Normal).Layout),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
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
