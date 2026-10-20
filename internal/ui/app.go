package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"postheader/assets"
	"postheader/internal/config"
	"postheader/internal/platform"
	"postheader/internal/render"
)

// Version подставляется при сборке: -ldflags "-X postheader/internal/ui.Version=v1.0.0".
var Version = "dev"

type App struct {
	w   *app.Window
	th  *material.Theme
	cfg config.Config
	wk  *worker
	ui  chan func() // действия из фоновых горутин, выполняются в кадре

	loadErr    error
	splash     timeline
	splashDone bool
	mainFade   timeline
	bg         paint.ImageOp

	text, phone           widget.Editor
	textFocus, phoneFocus ease
	pasteBtn, clearBtn    Button
	markup                markupBar

	fmtBtns                                  [3]Button
	bgBtns                                   [4]Button
	ovBtns                                   [2]Button
	cntBtns                                  [4]Button
	phoneAtBtns                              [3]Button
	format, bgKind, overflow, count, phoneAt int

	photo     image.Image
	photoName string
	photoAdj  render.PhotoAdjust
	photoMove [7]Button // ← → ↑ ↓ − + по центру
	sharpBtns [3]Button
	lightBtns [3]Button
	logo      image.Image

	blocks   []render.BlockSummary
	icons    map[int]render.IconKind
	iconBtns []Button
	iconPick int // блок, у которого открыт выбор иконки (-1 — ни у какого)
	pickBtns []Button
	iconList widget.List

	gen       int
	dirtyAt   time.Time
	dirty     bool
	rendering bool
	variants  []variant
	cards     []variantCard

	saveAllBtn, openDirBtn, settingsBtn Button
	toast                               string
	toastErr                            bool
	toastAt                             time.Time

	settings               settingsPanel
	upd                    updater
	welcome                welcome
	profileBtn, myPhoneBtn Button
	zoom                   *paint.ImageOp
	zoomBtn                widget.Clickable
}

type variantCard struct {
	save, copy, prev, next Button
	open                   widget.Clickable
	page                   int
	appear                 timeline
	hover                  ease
	ops                    []paint.ImageOp
}

// Run открывает окно и крутит цикл событий до закрытия.
func Run() error {
	a := &App{cfg: config.Load(), ui: make(chan func(), 16), icons: map[int]render.IconKind{}, iconPick: -1}
	a.w = new(app.Window)
	a.w.Option(app.Title("Post Header"), app.Size(1360, 860), app.MinSize(1200, 760), app.Maximized.Option())
	var err error
	if a.th, err = newTheme(); err != nil {
		return err
	}
	a.bg = paint.NewImageOp(backdrop(480, 300))
	a.text.WrapPolicy = text.WrapWords
	a.phone.SingleLine = true
	a.iconList.Axis = layout.Vertical
	a.applyConfig()

	// Тяжёлая загрузка (эмодзи, шрифты) идёт, пока играет заставка.
	go func() {
		wk, err := newWorker(a.w.Invalidate)
		a.post(func() {
			a.wk, a.loadErr = wk, err
			if err == nil {
				a.submit()
			}
		})
	}()

	var ops op.Ops
	for {
		switch e := a.w.Event().(type) {
		case app.DestroyEvent:
			a.cfg.Draft = a.text.Text()
			config.Save(a.cfg)
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) post(f func()) {
	a.ui <- f
	a.w.Invalidate()
}

func (a *App) applyConfig() {
	c := a.cfg
	a.format, a.bgKind, a.overflow, a.phoneAt = c.Format, c.Background, c.Overflow, c.PhoneAt
	if a.bgKind == 3 { // фото не сохраняем между запусками
		a.bgKind = 0
	}
	a.count = c.Variants - 1
	a.phone.SetText(c.Phone)
	if c.Phone == "" {
		a.phone.SetText(c.UserPhone)
	}
	a.settings.address.SingleLine = true
	a.settings.address.SetText(c.Address)
	a.loadLogo()
	if demo := os.Getenv("POSTHEADER_DEMO"); demo != "" {
		if b, err := os.ReadFile(demo); err == nil {
			a.text.SetText(string(b))
		}
	} else {
		a.text.SetText(c.Draft)
	}
	if p := os.Getenv("POSTHEADER_PHOTO"); p != "" && os.Getenv("POSTHEADER_DEMO") != "" {
		a.loadPhoto(p)
	}
}

func (a *App) loadLogo() {
	a.logo = nil
	if p := a.cfg.LogoPath; p != "" {
		if img, err := decodeFile(p); err == nil {
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".jpg" || ext == ".jpeg" {
				img = render.TrimTransparent(render.KeyDarkBackground(img, 45, 110))
			}
			a.logo = img
			return
		}
	}
	a.logo, _, _ = image.Decode(bytes.NewReader(assets.LogoAdresa))
}

func (a *App) loadPhoto(path string) {
	img, err := decodeFile(path)
	if err != nil {
		a.notify("Не удалось открыть фото: "+err.Error(), true)
		return
	}
	a.photo, a.photoName, a.bgKind = img, filepath.Base(path), 3
	a.photoAdj = render.PhotoAdjust{}
	a.submit()
}

func decodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// submit отправляет текущие настройки в рендер.
func (a *App) submit() {
	a.dirty = false
	if a.wk == nil {
		return
	}
	txt := a.text.Text()
	a.blocks = render.Blocks(txt, a.wk.emoji)
	for i := range a.icons {
		if i >= len(a.blocks) {
			delete(a.icons, i)
		}
	}
	if strings.TrimSpace(txt) == "" {
		a.variants, a.cards, a.rendering = nil, nil, false
		return
	}
	o := render.Options{
		Format:   render.Formats[a.format],
		Overflow: render.Overflow(a.overflow),
		Phone:    a.phone.Text(),
		PhoneAt:  render.PhonePlace(a.phoneAt),
		Brand:    render.Brand{Logo: a.logo, Address: a.cfg.Address},
		Icons:    map[int]render.IconKind{},
	}
	for k, v := range a.icons {
		o.Icons[k] = v
	}
	switch a.bgKind {
	case 1:
		o.Background = render.BgWhite
	case 2:
		o.Background = render.BgGraphite
	case 3:
		o.Background, o.Photo, o.PhotoAdj = render.BgPhoto, a.photo, a.photoAdj
	}
	a.gen++
	a.rendering = true
	a.wk.submit(job{gen: a.gen, text: txt, opts: o, variants: a.count + 1,
		font: fontChoice{Regular: a.cfg.FontPath, Bold: a.cfg.FontBold}})
	a.saveConfig()
}

func (a *App) saveConfig() {
	a.cfg.Format, a.cfg.Background, a.cfg.Overflow = a.format, a.bgKind, a.overflow
	a.cfg.Variants, a.cfg.PhoneAt, a.cfg.Phone = a.count+1, a.phoneAt, a.phone.Text()
	a.cfg.Draft = a.text.Text()
	go config.Save(a.cfg)
}

func (a *App) notify(msg string, isErr bool) {
	a.toast, a.toastErr, a.toastAt = msg, isErr, time.Now()
	a.w.Invalidate()
}

// --- кадр ---

func (a *App) frame(gtx C) {
	for {
		select {
		case f := <-a.ui:
			f()
			continue
		default:
		}
		break
	}
	if a.wk != nil {
		a.drainResults(gtx)
	}

	// фон окна
	func() {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		widget.Image{Src: a.bg, Fit: widget.Fill}.Layout(gtx)
	}()

	if !a.splashDone {
		a.layoutSplash(gtx)
		if gtx.Now.Sub(a.splash.start) > 2100*time.Millisecond && (a.wk != nil || a.loadErr != nil) {
			a.splashDone = true
			a.mainFade.restart(gtx.Now)
			a.upd.check(a, false)
			if !a.cfg.Welcomed {
				a.welcome.show(a)
			}
		}
		gtx.Execute(op.InvalidateCmd{})
		return
	}

	// Появление главного экрана: лёгкий сдвиг вверх и «растворение» тёмной вуали.
	// Вуаль вместо paint.PushOpacity: слои прозрачности на старых видеокартах
	// рисуют текст с артефактами.
	p := a.mainFade.at(gtx, 0, 450*time.Millisecond)
	st := op.Offset(image.Pt(0, int(float32(gtx.Dp(14))*(1-p)))).Push(gtx.Ops)
	a.layoutMain(gtx)
	st.Pop()
	if p < 1 {
		paint.FillShape(gtx.Ops, alpha(cBg, 1-p), clip.Rect{Max: gtx.Constraints.Max}.Op())
	}
	a.layoutZoom(gtx)
	a.settings.layout(gtx, a)
	a.welcome.layout(gtx, a)
}

// setProfile сохраняет имя и телефон сотрудника; телефон сразу идёт в пост,
// если там пусто или стоял прежний «свой» номер.
func (a *App) setProfile(name, phone string) {
	name, phone = strings.TrimSpace(name), strings.TrimSpace(phone)
	cur := strings.TrimSpace(a.phone.Text())
	if cur == "" || cur == a.cfg.UserPhone {
		a.phone.SetText(phone)
	}
	a.cfg.UserName, a.cfg.UserPhone = name, phone
	a.dirty, a.dirtyAt = true, time.Now()
	a.w.Invalidate()
}

func (a *App) drainResults(gtx C) {
	for {
		select {
		case r := <-a.wk.results:
			if r.gen != a.gen {
				continue
			}
			a.rendering = false
			if r.err != nil {
				a.notify("Ошибка рендера: "+r.err.Error(), true)
				continue
			}
			old := a.cards
			a.variants = r.variants
			a.cards = make([]variantCard, len(r.variants))
			for i, v := range r.variants {
				c := &a.cards[i]
				if i < len(old) {
					c.page = old[i].page
				}
				if c.page >= len(v.pages) {
					c.page = 0
				}
				for _, p := range v.previews {
					c.ops = append(c.ops, paint.NewImageOp(p))
				}
				c.appear.restart(gtx.Now.Add(time.Duration(i) * 70 * time.Millisecond))
			}
		default:
			return
		}
	}
}

func (a *App) handleInput(gtx C) {
	for {
		ev, ok := a.text.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			a.dirty, a.dirtyAt = true, gtx.Now
		}
	}
	for {
		ev, ok := a.phone.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			a.dirty, a.dirtyAt = true, gtx.Now
		}
	}
	// Перерисовываем через 350 мс после последнего нажатия, а не на каждую букву.
	if a.dirty {
		at := a.dirtyAt.Add(350 * time.Millisecond)
		if !gtx.Now.Before(at) {
			a.submit()
		} else {
			gtx.Execute(op.InvalidateCmd{At: at})
		}
	}

	a.markup.update(gtx, a)
	if a.pasteBtn.Clicked(gtx) {
		go func() {
			s, err := platform.ClipboardText()
			a.post(func() {
				if err != nil {
					a.notify(err.Error(), true)
					return
				}
				a.text.Insert(s)
				a.submit()
			})
		}()
	}
	if a.clearBtn.Clicked(gtx) {
		a.text.SetText("")
		a.icons = map[int]render.IconKind{}
		a.iconPick = -1
		a.submit()
	}
	changed := false
	pick := func(btns []Button, sel *int) {
		for i := range btns {
			if btns[i].Clicked(gtx) && *sel != i {
				*sel = i
				changed = true
			}
		}
	}
	pick(a.fmtBtns[:], &a.format)
	pick(a.ovBtns[:], &a.overflow)
	pick(a.cntBtns[:], &a.count)
	pick(a.phoneAtBtns[:], &a.phoneAt)
	for i := range a.bgBtns {
		if !a.bgBtns[i].Clicked(gtx) {
			continue
		}
		if i == 3 {
			go func() {
				p, err := platform.OpenImageFile("Фото объекта для фона")
				a.post(func() {
					if err != nil {
						a.notify(err.Error(), true)
					} else if p != "" {
						a.loadPhoto(p)
					}
				})
			}()
			continue
		}
		if a.bgKind != i {
			a.bgKind = i
			changed = true
		}
	}
	for i := range a.iconBtns {
		if i < len(a.blocks) && a.iconBtns[i].Clicked(gtx) {
			if a.iconPick == i {
				a.iconPick = -1
			} else {
				a.iconPick = i
			}
		}
	}
	if len(a.pickBtns) < len(render.IconChoices) {
		a.pickBtns = make([]Button, len(render.IconChoices))
	}
	for j, ch := range render.IconChoices {
		if a.pickBtns[j].Clicked(gtx) && a.iconPick >= 0 && a.iconPick < len(a.blocks) {
			a.icons[a.iconPick] = ch.Kind
			a.iconPick = -1
			changed = true
		}
	}
	if a.updatePhotoAdj(gtx) {
		changed = true
	}
	if changed {
		a.submit()
	}

	for i := range a.cards {
		c := &a.cards[i]
		v := a.variants[i]
		if c.prev.Clicked(gtx) && c.page > 0 {
			c.page--
		}
		if c.next.Clicked(gtx) && c.page < len(v.pages)-1 {
			c.page++
		}
		if c.save.Clicked(gtx) {
			a.saveVariant(v)
		}
		if c.copy.Clicked(gtx) {
			img := v.pages[c.page]
			go func() {
				err := platform.CopyImage(img)
				a.post(func() {
					if err != nil {
						a.notify(err.Error(), true)
					} else {
						a.notify("Картинка в буфере — вставляйте в VK или Telegram (Ctrl+V)", false)
					}
				})
			}()
		}
		if c.open.Clicked(gtx) {
			op := paint.NewImageOp(v.pages[c.page])
			a.zoom = &op
		}
	}
	if a.zoomBtn.Clicked(gtx) {
		a.zoom = nil
	}
	if a.saveAllBtn.Clicked(gtx) {
		for _, v := range a.variants {
			a.saveVariant(v)
		}
	}
	if a.openDirBtn.Clicked(gtx) {
		dir := a.outDir()
		os.MkdirAll(dir, 0o755)
		platform.OpenFolder(dir)
	}
	if a.settingsBtn.Clicked(gtx) || a.profileBtn.Clicked(gtx) {
		a.settings.show(gtx, a)
	}
	if a.myPhoneBtn.Clicked(gtx) {
		a.phone.SetText(a.cfg.UserPhone)
		a.submit()
	}
}

func (a *App) currentIcon(i int) render.IconKind {
	if k, ok := a.icons[i]; ok {
		return k
	}
	return a.blocks[i].Icon
}

func (a *App) outDir() string {
	if a.cfg.OutDir != "" {
		return a.cfg.OutDir
	}
	return platform.PicturesDir()
}

func (a *App) saveVariant(v variant) {
	dir := a.outDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.notify("Не удалось создать папку: "+err.Error(), true)
		return
	}
	stamp := time.Now().Format("2006-01-02_15-04-05")
	for i, p := range v.pages {
		name := fmt.Sprintf("%s_%s_%s", stamp, render.Formats[a.format].ID, v.style.ID)
		if len(v.pages) > 1 {
			name += fmt.Sprintf("_%d", i+1)
		}
		f, err := os.Create(filepath.Join(dir, name+".jpg"))
		if err == nil {
			err = jpeg.Encode(f, p, &jpeg.Options{Quality: 93})
			f.Close()
		}
		if err != nil {
			a.notify("Не удалось сохранить: "+err.Error(), true)
			return
		}
	}
	a.notify("Сохранено в "+dir, false)
}

// --- главный экран ---

func (a *App) layoutMain(gtx C) D {
	a.handleInput(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.layoutTopBar),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Left: 20, Right: 20, Bottom: 6}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						w := gtx.Dp(400)
						gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
						return a.layoutLeft(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: 16}.Layout),
					layout.Flexed(1, a.layoutRight),
				)
			})
		}),
		layout.Rigid(a.layoutFooter),
	)
}

func (a *App) layoutTopBar(gtx C) D {
	return layout.Inset{Top: 16, Bottom: 14, Left: 24, Right: 20}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				s := gtx.Dp(40)
				strokeHex(gtx, float32(s)/2, float32(s)/2, float32(s)/2-2, float32(gtx.Dp(2)), 1, cGold)
				defer op.Offset(image.Pt(0, 0)).Push(gtx.Ops).Pop()
				gtx.Constraints = layout.Exact(image.Pt(s, s))
				return layout.Center.Layout(gtx, label(a.th, 14, "PH", cGold, font.Bold).Layout)
			}),
			layout.Rigid(layout.Spacer{Width: 14}.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(label(a.th, 19, "POST HEADER", cGold, font.Bold).Layout),
					layout.Rigid(label(a.th, 12.5, "посты для соцсетей в фирменном стиле «Адреса»", cMuted, font.Normal).Layout),
				)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx C) D { return a.upd.layoutBadge(gtx, a, &a.upd.btn) }),
			layout.Rigid(func(gtx C) D {
				if a.cfg.UserName == "" {
					return D{}
				}
				return layout.Inset{Right: 10}.Layout(gtx, func(gtx C) D {
					return a.profileBtn.Layout(gtx, a.th, "Сотрудник: "+a.cfg.UserName, btnChip, false)
				})
			}),
			layout.Rigid(func(gtx C) D { return a.settingsBtn.Layout(gtx, a.th, "Настройки", btnGhost, false) }),
		)
	})
}

func (a *App) layoutLeft(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(0.64, func(gtx C) D {
			return card(gtx, a.th, "Текст поста", func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return fieldFrame(gtx, gtx.Focused(&a.text), &a.textFocus, func(gtx C) D {
							gtx.Constraints.Min = gtx.Constraints.Max
							return editorStyle(a.th, &a.text, "Напишите или вставьте текст поста…\n\nПустая строка — новый блок.\n«- » в начале строки — пункт с золотым тире, «1. » — нумерованный пункт.\nСтрока ПРОПИСНЫМИ — золотой заголовок.\n**слово** — золотое выделение.").Layout(gtx)
						})
					}),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(a.layoutMarkup),
					layout.Rigid(layout.Spacer{Height: 10}.Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return a.pasteBtn.Layout(gtx, a.th, "Вставить из буфера", btnPrimary, false)
							}),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Rigid(func(gtx C) D { return a.clearBtn.Layout(gtx, a.th, "Очистить", btnGhost, false) }),
							layout.Flexed(1, func(gtx C) D {
								n := len([]rune(a.text.Text()))
								return layout.E.Layout(gtx, label(a.th, 12, fmt.Sprintf("%d симв.", n), cMuted, font.Normal).Layout)
							}),
						)
					}),
				)
			}, true)
		}),
		layout.Rigid(layout.Spacer{Height: 12}.Layout),
		layout.Flexed(0.36, func(gtx C) D {
			return card(gtx, a.th, "Иконки блоков · нажмите, чтобы сменить", a.layoutIcons, true)
		}),
		layout.Rigid(layout.Spacer{Height: 12}.Layout),
		layout.Rigid(func(gtx C) D {
			return card(gtx, a.th, "Телефон для этого поста", func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return fieldFrame(gtx, gtx.Focused(&a.phone), &a.phoneFocus, editorStyle(a.th, &a.phone, "+7 920 000-00-00").Layout)
					}),
					layout.Rigid(layout.Spacer{Height: 10}.Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return segmented(gtx, a.th, a.phoneAtBtns[:], []string{"Не показывать", "В шапке", "Внизу"}, &a.phoneAt)
							}),
							layout.Flexed(1, func(gtx C) D {
								if a.cfg.UserPhone == "" || strings.TrimSpace(a.phone.Text()) == a.cfg.UserPhone {
									return D{}
								}
								return layout.E.Layout(gtx, func(gtx C) D { return a.myPhoneBtn.Layout(gtx, a.th, "Мой номер", btnGhost, false) })
							}),
						)
					}),
				)
			}, false)
		}),
	)
}

// layoutMarkup — ряд кнопок разметки под полем текста.
func (a *App) layoutMarkup(gtx C) D { return a.markup.layout(gtx, a) }

func (a *App) layoutIcons(gtx C) D {
	if len(a.iconBtns) < len(a.blocks) {
		a.iconBtns = append(a.iconBtns, make([]Button, len(a.blocks)-len(a.iconBtns))...)
	}
	if len(a.blocks) == 0 {
		return label(a.th, 13, "Блоки появятся, когда вы введёте текст. Иконка подбирается сама по эмодзи и словам.", cMuted, font.Normal).Layout(gtx)
	}
	return material.List(a.th, &a.iconList).Layout(gtx, len(a.blocks), func(gtx C, i int) D {
		row := func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					s := gtx.Dp(24)
					strokeHex(gtx, float32(s)/2, float32(s)/2, float32(s)/2-1, float32(gtx.Dp(1.2)), 1, alpha(cGold, 0.7))
					gtx.Constraints = layout.Exact(image.Pt(s, s))
					return layout.Center.Layout(gtx, label(a.th, 11, fmt.Sprint(i+1), cGold, font.SemiBold).Layout)
				}),
				layout.Rigid(layout.Spacer{Width: 10}.Layout),
				layout.Flexed(1, func(gtx C) D {
					l := label(a.th, 13, a.blocks[i].Title, cText, font.Normal)
					l.MaxLines = 1
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx C) D {
					_, manual := a.icons[i]
					return a.iconBtns[i].Layout(gtx, a.th, render.IconName(a.currentIcon(i)), btnChip, manual || a.iconPick == i)
				}),
			)
		}
		return layout.Inset{Bottom: 6, Right: 6}.Layout(gtx, func(gtx C) D {
			if a.iconPick != i {
				return row(gtx)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(row),
				layout.Rigid(func(gtx C) D { return layout.Inset{Top: 6, Left: 34}.Layout(gtx, a.layoutIconPicker) }),
			)
		})
	})
}

// layoutIconPicker — все варианты иконки в три ряда, текущий подсвечен.
func (a *App) layoutIconPicker(gtx C) D {
	cur := a.currentIcon(a.iconPick)
	var rows []layout.FlexChild
	for start := 0; start < len(render.IconChoices); start += 3 {
		start := start
		rows = append(rows, layout.Rigid(func(gtx C) D {
			var ch []layout.FlexChild
			for j := start; j < start+3; j++ {
				j := j
				ch = append(ch, layout.Flexed(1, func(gtx C) D {
					if j >= len(render.IconChoices) {
						return D{}
					}
					return layout.Inset{Right: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						c := render.IconChoices[j]
						return a.pickBtns[j].Layout(gtx, a.th, c.Name, btnChip, c.Kind == cur)
					})
				}))
			}
			return layout.Flex{}.Layout(gtx, ch...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func group(gtx C, th *material.Theme, title string, w layout.Widget) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(label(th, 11.5, strings.ToUpper(title), cMuted, font.SemiBold).Layout),
		layout.Rigid(layout.Spacer{Height: 7}.Layout),
		layout.Rigid(w),
	)
}

func (a *App) layoutRight(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return card(gtx, a.th, "", func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return group(gtx, a.th, "Размер", func(gtx C) D {
									return segmented(gtx, a.th, a.fmtBtns[:], []string{"9:16 сторис", "4:5 пост", "1:1 квадрат"}, &a.format)
								})
							}),
							layout.Rigid(layout.Spacer{Width: 28}.Layout),
							layout.Rigid(func(gtx C) D {
								photo := "Фото объекта…"
								if a.photoName != "" {
									photo = "Фото: " + shorten(a.photoName, 14)
								}
								return group(gtx, a.th, "Фон", func(gtx C) D {
									return segmented(gtx, a.th, a.bgBtns[:], []string{"Чёрный", "Белый", "Графит", photo}, &a.bgKind)
								})
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: 14}.Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return group(gtx, a.th, "Длинный текст", func(gtx C) D {
									return segmented(gtx, a.th, a.ovBtns[:], []string{"Уменьшить шрифт", "Разбить на слайды"}, &a.overflow)
								})
							}),
							layout.Rigid(layout.Spacer{Width: 28}.Layout),
							layout.Rigid(func(gtx C) D {
								return group(gtx, a.th, "Сколько вариантов", func(gtx C) D {
									return segmented(gtx, a.th, a.cntBtns[:], []string{"1", "2", "3", "4"}, &a.count)
								})
							}),
						)
					}),
				)
			}, false)
		}),
		layout.Rigid(func(gtx C) D {
			if a.bgKind != 3 || a.photo == nil {
				return D{}
			}
			return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D { return card(gtx, a.th, "", a.layoutPhotoAdj, false) })
		}),
		layout.Rigid(layout.Spacer{Height: 14}.Layout),
		layout.Flexed(1, a.layoutVariants),
		layout.Rigid(layout.Spacer{Height: 12}.Layout),
		layout.Rigid(a.layoutActions),
	)
}

// updatePhotoAdj — кнопки подгонки фото: сдвиг кадра, увеличение, чёткость, яркость.
func (a *App) updatePhotoAdj(gtx C) bool {
	adj := a.photoAdj
	step := 0.25
	if adj.Zoom < 1 {
		adj.Zoom = 1
	}
	clamp := func(v, lo, hi float64) float64 { return math.Max(lo, math.Min(v, hi)) }
	for i := range a.photoMove {
		if !a.photoMove[i].Clicked(gtx) {
			continue
		}
		switch i {
		case 0:
			adj.X = clamp(adj.X-step, -1, 1)
		case 1:
			adj.X = clamp(adj.X+step, -1, 1)
		case 2:
			adj.Y = clamp(adj.Y-step, -1, 1)
		case 3:
			adj.Y = clamp(adj.Y+step, -1, 1)
		case 4:
			adj.Zoom = clamp(adj.Zoom-0.25, 1, 3)
		case 5:
			adj.Zoom = clamp(adj.Zoom+0.25, 1, 3)
		case 6:
			adj.X, adj.Y, adj.Zoom = 0, 0, 1
		}
	}
	for i := range a.sharpBtns {
		if a.sharpBtns[i].Clicked(gtx) {
			adj.Sharp = i
		}
	}
	for i := range a.lightBtns {
		if a.lightBtns[i].Clicked(gtx) {
			adj.Light = i
		}
	}
	if adj == a.photoAdj {
		return false
	}
	a.photoAdj = adj
	return true
}

func (a *App) layoutPhotoAdj(gtx C) D {
	chips := func(labels []string, btns []Button) layout.Widget {
		return func(gtx C) D {
			var ch []layout.FlexChild
			for i := range labels {
				i := i
				ch = append(ch, layout.Rigid(func(gtx C) D {
					return layout.Inset{Right: 6}.Layout(gtx, func(gtx C) D { return btns[i].Layout(gtx, a.th, labels[i], btnChip, false) })
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, ch...)
		}
	}
	zoom := a.photoAdj.Zoom
	if zoom < 1 {
		zoom = 1
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return group(gtx, a.th, "Кадр фото", chips([]string{"←", "→", "↑", "↓"}, a.photoMove[0:4]))
				}),
				layout.Rigid(layout.Spacer{Width: 22}.Layout),
				layout.Rigid(func(gtx C) D {
					return group(gtx, a.th, fmt.Sprintf("Увеличение %.0f%%", zoom*100), chips([]string{"−", "+", "Сбросить"}, a.photoMove[4:7]))
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: 12}.Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return group(gtx, a.th, "Чёткость", func(gtx C) D {
						return segmented(gtx, a.th, a.sharpBtns[:], []string{"Размыто", "Слегка", "Чётко"}, &a.photoAdj.Sharp)
					})
				}),
				layout.Rigid(layout.Spacer{Width: 22}.Layout),
				layout.Rigid(func(gtx C) D {
					return group(gtx, a.th, "Яркость фото", func(gtx C) D {
						return segmented(gtx, a.th, a.lightBtns[:], []string{"Обычно", "Светлее", "Темнее"}, &a.photoAdj.Light)
					})
				}),
			)
		}),
	)
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (a *App) layoutVariants(gtx C) D {
	if len(a.cards) == 0 {
		msg := "Вставьте текст поста слева — здесь появятся готовые варианты"
		if a.loadErr != nil {
			msg = "Ошибка загрузки: " + a.loadErr.Error()
		} else if a.rendering || a.wk == nil {
			msg = "Рисую варианты…"
		}
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					s := gtx.Dp(70)
					pulse := float32(0.55)
					if a.rendering {
						t := float32(gtx.Now.UnixMilli()%1400) / 1400
						pulse = 0.35 + 0.65*easeOut(1-abs(2*t-1))
						gtx.Execute(op.InvalidateCmd{})
					}
					strokeHex(gtx, float32(s)/2, float32(s)/2, float32(s)/2-2, float32(gtx.Dp(2)), 1, alpha(cGold, pulse))
					return D{Size: image.Pt(s, s)}
				}),
				layout.Rigid(layout.Spacer{Height: 16}.Layout),
				layout.Rigid(label(a.th, 15, msg, cMuted, font.Normal).Layout),
			)
		})
	}
	children := make([]layout.FlexChild, 0, len(a.cards)*2)
	for i := range a.cards {
		i := i
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 14}.Layout))
		}
		children = append(children, layout.Flexed(1, func(gtx C) D { return a.layoutCard(gtx, i) }))
	}
	return layout.Flex{}.Layout(gtx, children...)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) layoutCard(gtx C, i int) D {
	c := &a.cards[i]
	v := a.variants[i]
	p := c.appear.at(gtx, 0, 380*time.Millisecond)
	h := c.hover.to(gtx, boolf(c.open.Hovered()), 12)

	defer op.Offset(image.Pt(0, int(float32(gtx.Dp(18))*(1-p)))).Push(gtx.Ops).Pop()

	gtx.Constraints.Min = gtx.Constraints.Max
	r := image.Rectangle{Max: gtx.Constraints.Max}
	rad := gtx.Dp(14)
	fillGrad(gtx, r, rad, cPanelHi, cPanel)
	strokeRRect(gtx, r, rad, float32(gtx.Dp(1)), mix(cLine, alpha(cGold, 0.75), h))
	if p < 1 {
		defer fillRRect(gtx, r, rad, alpha(cBg, 1-p)) // вуаль появления поверх карточки
	}

	return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return c.open.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min = gtx.Constraints.Max
					d := widget.Image{Src: c.ops[c.page], Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
					defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
					pointer.CursorPointer.Add(gtx.Ops)
					return D{Size: gtx.Constraints.Max, Baseline: d.Baseline}
				})
			}),
			layout.Rigid(layout.Spacer{Height: 10}.Layout),
			layout.Rigid(func(gtx C) D {
				info := fmt.Sprintf("кегль %.0f", v.base)
				if v.slides {
					info = fmt.Sprintf("%d слайда · кегль %.0f", len(v.pages), v.base)
					if len(v.pages) >= 5 {
						info = fmt.Sprintf("%d слайдов · кегль %.0f", len(v.pages), v.base)
					}
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						l := label(a.th, 13.5, v.style.Name, cText, font.SemiBold)
						l.MaxLines = 1
						return l.Layout(gtx)
					}),
					layout.Rigid(label(a.th, 12, info, cMuted, font.Normal).Layout),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if len(v.pages) < 2 {
					return D{}
				}
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return c.prev.Layout(gtx, a.th, "‹", btnChip, false) }),
						layout.Flexed(1, func(gtx C) D {
							return layout.Center.Layout(gtx, label(a.th, 13, fmt.Sprintf("слайд %d из %d", c.page+1, len(v.pages)), cGold, font.Medium).Layout)
						}),
						layout.Rigid(func(gtx C) D { return c.next.Layout(gtx, a.th, "›", btnChip, false) }),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Height: 10}.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return c.save.Layout(gtx, a.th, "Сохранить", btnPrimary, false)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Flexed(1, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return c.copy.Layout(gtx, a.th, "В буфер", btnGhost, false)
					}),
				)
			}),
		)
	})
}

func (a *App) layoutActions(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if len(a.variants) == 0 {
				return D{}
			}
			return a.saveAllBtn.Layout(gtx, a.th, "Сохранить все варианты", btnPrimary, false)
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx C) D {
			return a.openDirBtn.Layout(gtx, a.th, "Открыть папку с постами", btnGhost, false)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, a.layoutStatus)
		}),
	)
}

func (a *App) layoutStatus(gtx C) D {
	if a.rendering && len(a.cards) > 0 {
		dots := strings.Repeat("•", int(gtx.Now.UnixMilli()/350%3)+1)
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(350 * time.Millisecond)})
		return label(a.th, 13, "обновляю "+dots, cGold, font.Medium).Layout(gtx)
	}
	if a.toast == "" {
		return D{}
	}
	age := gtx.Now.Sub(a.toastAt)
	if age > 4*time.Second {
		a.toast = ""
		return D{}
	}
	op := float32(1)
	if age > 3*time.Second {
		op = 1 - float32(age-3*time.Second)/float32(time.Second)
	}
	gtx.Execute(opInvalidate(gtx))
	col := cGoldLight
	if a.toastErr {
		col = cDanger
	}
	return label(a.th, 13, a.toast, alpha(col, op), font.Medium).Layout(gtx)
}

func opInvalidate(gtx C) op.InvalidateCmd {
	return op.InvalidateCmd{At: gtx.Now.Add(40 * time.Millisecond)}
}

func (a *App) layoutFooter(gtx C) D {
	return layout.Inset{Left: 24, Right: 24, Bottom: 10, Top: 2}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(label(a.th, 11.5, Signature, alpha(cGold, 0.65), font.Medium).Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, label(a.th, 11.5, "версия "+Version, cMuted, font.Normal).Layout)
			}),
		)
	})
}

// layoutZoom — крупный просмотр варианта по клику; клик в любом месте закрывает.
func (a *App) layoutZoom(gtx C) {
	if a.zoom == nil {
		return
	}
	paint.FillShape(gtx.Ops, alpha(rgb(0x000000), 0.94), clip.Rect{Max: gtx.Constraints.Max}.Op())
	a.zoomBtn.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		layout.UniformInset(28).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return widget.Image{Src: *a.zoom, Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
		})
		return D{Size: gtx.Constraints.Max}
	})
}

// --- заставка ---

func (a *App) layoutSplash(gtx C) {
	t := &a.splash
	size := gtx.Constraints.Max
	cx, cy := float32(size.X)/2, float32(size.Y)/2-float32(gtx.Dp(30))
	r := float32(gtx.Dp(62))

	draw := t.at(gtx, 150*time.Millisecond, 1100*time.Millisecond)
	glow := t.at(gtx, 900*time.Millisecond, 600*time.Millisecond)
	title := t.at(gtx, 1000*time.Millisecond, 600*time.Millisecond)
	sub := t.at(gtx, 1350*time.Millisecond, 600*time.Millisecond)

	// мягкое свечение за знаком
	for i := 0; i < 6; i++ {
		rr := r * (1.25 + float32(i)*0.22)
		paint.FillShape(gtx.Ops, alpha(cGold, 0.035*glow), clip.Ellipse{
			Min: image.Pt(int(cx-rr), int(cy-rr)), Max: image.Pt(int(cx+rr), int(cy+rr))}.Op(gtx.Ops))
	}
	strokeHex(gtx, cx, cy, r, float32(gtx.Dp(3)), draw, cGold)
	strokeHex(gtx, cx, cy, r*0.78, float32(gtx.Dp(1.2)), draw*draw, alpha(cGold, 0.5))

	centered := func(y int, w layout.Widget, lift float32) {
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints.Min = image.Point{}
		d := w(g)
		call := m.Stop()
		st := op.Offset(image.Pt((size.X-d.Size.X)/2, y+int(lift))).Push(gtx.Ops)
		call.Add(gtx.Ops)
		st.Pop()
	}
	mark := label(a.th, 30, "PH", alpha(cGold, glow), font.Bold)
	centered(int(cy)-gtx.Dp(20), mark.Layout, 0)
	base := int(cy + r + float32(gtx.Dp(34)))
	centered(base, label(a.th, 34, "POST HEADER", alpha(cGold, title), font.Bold).Layout, float32(gtx.Dp(10))*(1-title))
	centered(base+gtx.Dp(52), label(a.th, 15, "посты в фирменном стиле «Адреса»", alpha(cMuted, sub), font.Normal).Layout, float32(gtx.Dp(8))*(1-sub))
	centered(base+gtx.Dp(80), label(a.th, 13, Signature, alpha(cGold, 0.7*sub), font.Medium).Layout, float32(gtx.Dp(8))*(1-sub))

	// тонкая полоса загрузки
	load := t.at(gtx, 0, 2000*time.Millisecond)
	w := float32(gtx.Dp(180))
	y := size.Y - gtx.Dp(70)
	x0 := int(float32(size.X)/2 - w/2)
	fillRRect(gtx, image.Rect(x0, y, x0+int(w), y+gtx.Dp(2)), 1, alpha(cGold, 0.15))
	fillRRect(gtx, image.Rect(x0, y, x0+int(w*load), y+gtx.Dp(2)), 1, cGold)
}
