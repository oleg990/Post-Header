package ui

import (
	"os"

	"gioui.org/font"
	"gioui.org/layout"

	"postheader/internal/config"
	"postheader/internal/platform"
	"postheader/internal/update"
)

// updater — автообновление: при запуске тихо спрашивает GitHub о новой
// версии; если она есть, в шапке окна появляется кнопка «Обновить».
type updater struct {
	rel      *update.Release
	checking bool
	status   string // для раздела «О программе»
	loading  bool
	progress float32

	btn, btn2, checkBtn Button
}

func (u *updater) check(a *App, manual bool) {
	if u.checking || u.loading {
		return
	}
	u.checking = true
	if manual {
		u.status = "Проверяю…"
	}
	go func() {
		rel, err := update.Check(Version)
		a.post(func() {
			u.checking = false
			switch {
			case err != nil:
				u.status = "Не удалось проверить: " + err.Error()
			case rel != nil:
				u.rel = rel
				u.status = "Доступна версия " + rel.Tag
			case Version == "dev" || len(Version) > 3 && Version[:4] == "dev-":
				u.status = "Тестовая сборка: обновления приходят только в установленную версию"
			default:
				u.status = "У вас последняя версия"
			}
		})
	}()
}

func (u *updater) install(a *App) {
	if u.rel == nil || u.loading {
		return
	}
	u.loading, u.progress = true, 0
	rel := u.rel
	go func() {
		last := -1
		path, err := update.Download(rel, func(p float32) {
			if pct := int(p * 100); pct != last {
				last = pct
				a.post(func() { u.progress = p })
			}
		})
		if err == nil {
			err = platform.RunInstaller(path)
		}
		a.post(func() {
			if err != nil {
				u.loading = false
				a.notify("Обновление не удалось: "+err.Error(), true)
				return
			}
			// Установщик заменит файлы и сам запустит новую версию.
			a.cfg.Draft = a.text.Text()
			config.Save(a.cfg)
			os.Exit(0)
		})
	}()
}

// layoutBadge — кнопка обновления в шапке окна (пусто, если обновлять нечего).
func (u *updater) layoutBadge(gtx C, a *App, b *Button) D {
	if u.rel == nil {
		return D{}
	}
	if b.Clicked(gtx) {
		u.install(a)
	}
	txt := "Обновить до " + u.rel.Tag
	if u.loading {
		txt = "Загрузка " + itoa(int(u.progress*100)) + "%"
	}
	return layout.Inset{Right: 10}.Layout(gtx, func(gtx C) D {
		return b.Layout(gtx, a.th, txt, btnPrimary, false)
	})
}

// layoutAbout — строка статуса и кнопка проверки в настройках.
func (u *updater) layoutAbout(gtx C, a *App) D {
	if u.checkBtn.Clicked(gtx) {
		u.check(a, true)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if u.status == "" {
				return D{}
			}
			return layout.Inset{Bottom: 8}.Layout(gtx, label(a.th, 12.5, u.status, cText, font.Normal).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			if u.rel != nil {
				return u.layoutBadge(gtx, a, &u.btn2)
			}
			return u.checkBtn.Layout(gtx, a.th, "Проверить обновления", btnGhost, false)
		}),
	)
}
