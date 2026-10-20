package render

import (
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

	"postheader/assets"
)

type Weight int

const (
	Regular Weight = iota
	Medium
	SemiBold
	Bold
)

// Fonts — встроенные Inter (кириллица есть), фейсы кэшируются по размеру.
type Fonts struct {
	byWeight [4]*opentype.Font
	inter    [4]*opentype.Font // запасной шрифт для символов, которых нет в своём (₽, стрелки)
	custom   bool
	buf      sfnt.Buffer
	hasGlyph map[glyphKey]bool

	mu    sync.Mutex
	faces map[faceKey]font.Face
}

type faceKey struct {
	w        Weight
	size     int // px * 4, чтобы автоподбор мог идти шагом 0.25 px
	fallback bool
}

type glyphKey struct {
	w Weight
	r rune
}

func LoadFonts() (*Fonts, error) {
	f := &Fonts{faces: make(map[faceKey]font.Face), hasGlyph: make(map[glyphKey]bool)}
	for i, data := range [][]byte{assets.InterRegular, assets.InterMedium, assets.InterSemiBold, assets.InterDisplayBold} {
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, err
		}
		f.byWeight[i] = parsed
		f.inter[i] = parsed
	}
	return f, nil
}

func (f *Fonts) Face(w Weight, px float64) font.Face {
	return f.face(w, px, false)
}

// Has сообщает, есть ли символ в основном шрифте этого начертания.
func (f *Fonts) Has(w Weight, r rune) bool {
	if !f.custom || r == ' ' {
		return true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := glyphKey{w, r}
	if v, ok := f.hasGlyph[key]; ok {
		return v
	}
	gi, err := f.byWeight[w].GlyphIndex(&f.buf, r)
	v := err == nil && gi != 0
	f.hasGlyph[key] = v
	return v
}

func (f *Fonts) face(w Weight, px float64, fallback bool) font.Face {
	key := faceKey{w, int(px*4 + 0.5), fallback}
	f.mu.Lock()
	defer f.mu.Unlock()
	if face, ok := f.faces[key]; ok {
		return face
	}
	src := f.byWeight[w]
	if fallback {
		src = f.inter[w]
	}
	face, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    float64(key.size) / 4,
		DPI:     72, // при 72 DPI размер в пунктах равен размеру в пикселях
		Hinting: font.HintingNone,
	})
	if err != nil {
		panic(err) // шрифты встроены и проверены при LoadFonts
	}
	f.faces[key] = face
	return face
}

// FontSet — свой шрифт вместо Inter. Пустые начертания берутся из ближайшего
// заданного, поэтому достаточно обычного и жирного файла (например, из C:\Windows\Fonts).
type FontSet struct {
	Regular, Medium, SemiBold, Bold []byte
}

func LoadFontSet(fs FontSet) (*Fonts, error) {
	base, err := LoadFonts()
	if err != nil {
		return nil, err
	}
	pick := func(list ...[]byte) []byte {
		for _, b := range list {
			if len(b) > 0 {
				return b
			}
		}
		return nil
	}
	data := [4][]byte{
		pick(fs.Regular, fs.Medium),
		pick(fs.Medium, fs.Regular),
		pick(fs.SemiBold, fs.Bold, fs.Medium),
		pick(fs.Bold, fs.SemiBold),
	}
	for i, d := range data {
		if d == nil {
			continue
		}
		parsed, err := opentype.Parse(d)
		if err != nil {
			return nil, err
		}
		base.byWeight[i] = parsed
		base.custom = true
	}
	return base, nil
}

// Clone — копия для другой горутины: разобранные шрифты общие (они
// потокобезопасны), а фейсы и их кэш свои.
func (f *Fonts) Clone() *Fonts {
	return &Fonts{
		byWeight: f.byWeight,
		inter:    f.inter,
		custom:   f.custom,
		hasGlyph: make(map[glyphKey]bool),
		faces:    make(map[faceKey]font.Face),
	}
}

// HasCyrillic — для списка шрифтов в интерфейсе: показываем только те, где есть кириллица.
func HasCyrillic(data []byte) bool {
	f, err := opentype.Parse(data)
	if err != nil {
		return false
	}
	var buf sfnt.Buffer
	for _, r := range "АЖЯжё" {
		if gi, err := f.GlyphIndex(&buf, r); err != nil || gi == 0 {
			return false
		}
	}
	return true
}
