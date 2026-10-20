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

// Twemoji (графика CC-BY 4.0) в SVG, упакованные в zip: ~4 МБ вместо ~17 МБ.
//
//go:embed emoji/twemoji.zip
var TwemojiZip []byte
