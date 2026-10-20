package render

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"strconv"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// EmojiSet — встроенный набор Twemoji. Цветные эмодзи рисуем сами из SVG,
// потому что на Windows 7 нет цветного Segoe UI Emoji, а x/image/font
// всё равно не умеет цветные глифы.
type EmojiSet struct {
	files  map[string]*zip.File
	starts map[rune]bool
	maxLen int

	mu    sync.Mutex
	cache map[emojiKey]*image.RGBA
}

type emojiKey struct {
	name string
	size int
}

func LoadEmoji(zipData []byte) (*EmojiSet, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("twemoji zip: %w", err)
	}
	e := &EmojiSet{
		files:  make(map[string]*zip.File, len(zr.File)),
		starts: make(map[rune]bool),
		cache:  make(map[emojiKey]*image.RGBA),
	}
	for _, f := range zr.File {
		name := strings.TrimSuffix(f.Name, ".svg")
		e.files[name] = f
		parts := strings.Split(name, "-")
		if len(parts) > e.maxLen {
			e.maxLen = len(parts)
		}
		if cp, err := strconv.ParseUint(parts[0], 16, 32); err == nil {
			e.starts[rune(cp)] = true
		}
	}
	// FE0F может стоять внутри последовательности, поэтому запас по длине.
	e.maxLen += 3
	return e, nil
}

// Match ищет самую длинную эмодзи-последовательность, начинающуюся с rs[i].
// Возвращает имя файла Twemoji и число поглощённых рун (0 — не эмодзи).
func (e *EmojiSet) Match(rs []rune, i int) (string, int) {
	if !e.starts[rs[i]] {
		return "", 0
	}
	maxN := len(rs) - i
	if maxN > e.maxLen {
		maxN = e.maxLen
	}
	for n := maxN; n >= 1; n-- {
		seq := rs[i : i+n]
		// Цифры, #, *, ©, ® считаем эмодзи только с FE0F или в составе keycap,
		// иначе обычный текст «1 комната» превратится в картинки.
		if seq[0] < 0x2000 && !hasPresentation(seq) {
			continue
		}
		full := joinHex(seq, false)
		if _, ok := e.files[full]; ok {
			return full, n
		}
		short := joinHex(seq, true)
		if _, ok := e.files[short]; ok {
			return short, n
		}
	}
	return "", 0
}

func hasPresentation(seq []rune) bool {
	for _, r := range seq[1:] {
		if r == 0xFE0F || r == 0x20E3 {
			return true
		}
	}
	return false
}

func joinHex(seq []rune, dropFE0F bool) string {
	var b strings.Builder
	for _, r := range seq {
		if dropFE0F && r == 0xFE0F {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
		b.WriteString(strconv.FormatInt(int64(r), 16))
	}
	return b.String()
}

// Image растеризует SVG в нужный размер (кэшируется).
func (e *EmojiSet) Image(name string, size int) (*image.RGBA, error) {
	key := emojiKey{name, size}
	e.mu.Lock()
	defer e.mu.Unlock()
	if img, ok := e.cache[key]; ok {
		return img, nil
	}
	f, ok := e.files[name]
	if !ok {
		return nil, fmt.Errorf("emoji %q not found", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	icon, err := oksvg.ReadIconStream(rc, oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, fmt.Errorf("emoji %q: %w", name, err)
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1)
	e.cache[key] = img
	return img, nil
}
