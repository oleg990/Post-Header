// Package assets встраивает шрифты и эмодзи прямо в .exe,
// чтобы рендер не зависел от того, что установлено в Windows.
package assets

import _ "embed"

//go:embed fonts/Inter-Regular.ttf
var InterRegular []byte

//go:embed fonts/Inter-Medium.ttf
var InterMedium []byte

//go:embed fonts/Inter-SemiBold.ttf
var InterSemiBold []byte

//go:embed fonts/InterDisplay-Bold.ttf
var InterDisplayBold []byte

// Cormorant Garamond (OFL) — только для «Предсказания дня», чтобы блок
// выглядел отдельно от остальной программы. Оставлены латиница и кириллица.
//
//go:embed fonts/CormorantGaramond-SemiBoldItalic.ttf
var CormorantItalic []byte

// Twemoji (графика CC-BY 4.0) в SVG, упакованные в zip: ~4 МБ вместо ~17 МБ.
//
//go:embed emoji/twemoji.zip
var TwemojiZip []byte

// NotoColorEmoji — цветные эмодзи для поля ввода в окне программы
// (в Windows 7 своего цветного шрифта эмодзи нет). OFL 1.1.
//
//go:embed fonts/NotoColorEmoji.ttf
var NotoColorEmoji []byte

// LogoAdresa — логотип по умолчанию (вырезан из присланного JPG, PNG с прозрачностью).
//
//go:embed brand/logo-adresa.png
var LogoAdresa []byte

// Русский словарь hunspell (А. Лебедев, BSD) для проверки орфографии.
//
//go:embed spell/ru_RU.dic.gz
var SpellDic []byte

//go:embed spell/ru_RU.aff.gz
var SpellAff []byte

// Что появилось в программе: показывается один раз тем, кто обновился
// со старой версии.
//
//go:embed whatsnew.md
var WhatsNew string
