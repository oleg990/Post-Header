package render

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// --- публичные настройки ---

type Format struct {
	ID, Name string
	W, H     int
}

var Formats = []Format{
	{"story", "Сторис / вертикальный 9:16 (VK, Telegram, Instagram)", 1080, 1920},
	{"portrait", "Пост 4:5 (Instagram, VK лента)", 1080, 1350},
	{"square", "Квадрат 1:1 (VK, Telegram, Instagram)", 1080, 1080},
}

type Style struct {
	ID, Name       string
	Frame          IconFrame
	StripLeadEmoji bool // эмодзи в начале блока превращается в иконку
	Separators     bool // золотые разделители с точкой между блоками
	Cards          bool // каждый блок в тёмной карточке
	Bar            bool // золотая вертикальная черта вместо иконки
	Wave           bool // золотая волна внизу
}

var Styles = []Style{
	{ID: "classic", Name: "Классика", Frame: FrameRoundSquare, Wave: true},
	{ID: "hex", Name: "Гексагоны", Frame: FrameHexagon, StripLeadEmoji: true, Separators: true},
	{ID: "cards", Name: "Карточки", Frame: FrameHexagon, StripLeadEmoji: true, Cards: true, Wave: true},
	{ID: "minimal", Name: "Минимал", Bar: true, Separators: true},
}

type BackgroundKind int

const (
	BgBlack BackgroundKind = iota
	BgGraphite
	BgPhoto
	BgWhite
)

type Overflow int

const (
	OverflowFit    Overflow = iota // уменьшать шрифт
	OverflowSlides                 // делить на слайды-карусель
)

type PhonePlace int

const (
	PhoneNone PhonePlace = iota
	PhoneHeader
	PhoneFooter
)

type Brand struct {
	Logo    image.Image // PNG с прозрачностью; JPG на тёмном фоне пропусти через KeyDarkBackground
	Address string
}

type Options struct {
	Format     Format
	Style      Style
	Background BackgroundKind
	Photo      image.Image // для BgPhoto
	Overflow   Overflow
	Phone      string
	PhoneAt    PhonePlace
	Brand      Brand
	Icons      map[int]IconKind // ручной выбор иконки: номер блока (с 0) → иконка
}

// Result — готовые слайды и пометка, если «ужать» не хватило и текст разбит.
type Result struct {
	Pages      []*image.RGBA
	FellBackTo Overflow
	BaseSize   float64
}

type Renderer struct {
	fonts *Fonts
	emoji *EmojiSet
}

func NewRenderer(f *Fonts, e *EmojiSet) *Renderer { return &Renderer{fonts: f, emoji: e} }

// palette — цвета, зависящие от фона: на белом золото темнее, текст тёмный.
type palette struct {
	Gold, Accent, Text, Card, Cutout, OnGold color.RGBA
	Glow                                     float64 // сила свечений: на белом слабее, иначе «грязные» пятна
	// Шапка: на белом светлое золото логотипа и адреса теряется, поэтому
	// лого перекрашивается по маске в градиент HeadTop→HeadBottom, адрес — HeadTop.
	TintLogo            bool
	Shine               color.RGBA // цвет бликов и свечений
	HeadTop, HeadBottom color.RGBA
}

var (
	black    = rgb(0x080808)
	darkPal  = palette{Gold: rgb(0xD9A85C), Accent: rgb(0xF0C77E), Text: rgb(0xEFEDEA), Card: rgb(0x121110), Cutout: black, OnGold: rgb(0x111111), Glow: 1, HeadTop: rgb(0xD9A85C), HeadBottom: rgb(0xD9A85C), Shine: rgb(0xF0C77E)}
	lightPal = palette{Gold: rgb(0xB8893F), Accent: rgb(0xA36F22), Text: rgb(0x1D1C1A), Card: rgb(0xFBF8F2), Cutout: rgb(0xFFFFFF), OnGold: rgb(0x111111), Glow: 0.35, TintLogo: true, HeadTop: rgb(0xA9782C), HeadBottom: rgb(0x7E5A1E), Shine: rgb(0xF2DDB4)}
)

// Render собирает пост: фон, шапка, текстовые блоки, низ.
func (r *Renderer) Render(text string, o Options) (*Result, error) {
	if o.Format.W == 0 {
		return nil, errors.New("format not set")
	}
	probe := r.newCanvas(o)
	k := probe.k
	margin := 46 * k
	headerBottom := r.headerHeight(o.Format, k)
	footerH := 70 * k
	if o.Style.Wave {
		footerH = 110 * k
	}
	if o.PhoneAt == PhoneFooter && o.Phone != "" {
		footerH += 70 * k
	}
	top := headerBottom + 34*k
	avail := float64(o.Format.H) - footerH - top
	contentW := float64(o.Format.W) - 2*margin

	blocks := parseText(text, r.emoji, o.Style.StripLeadEmoji)
	for i, ic := range o.Icons {
		if i >= 0 && i < len(blocks) {
			blocks[i].icon = ic
		}
	}
	if len(blocks) == 0 {
		return nil, errors.New("пустой текст")
	}

	mk := func(s float64) metrics {
		m := metrics{s: s, blockGap: s * 1.0}
		if o.Style.Frame != FrameNone {
			m.iconCol = s * 2.3
			m.textW = contentW - m.iconCol - s*0.75
		} else if o.Style.Bar {
			m.textW = contentW - s*0.9
		} else {
			m.textW = contentW
		}
		if o.Style.Cards {
			m.textW -= 2 * s * 0.7
			m.blockGap = s * 0.6
		}
		if o.Style.Separators {
			m.blockGap = s * 1.5
		}
		return m
	}
	lay := func(s float64) ([]blockOut, metrics) {
		m := mk(s)
		bs := probe.layoutBlocks(blocks, m)
		if o.Style.Cards {
			pad := s * 0.7
			for i := range bs {
				for j := range bs[i].lines {
					bs[i].lines[j].y += pad
				}
				bs[i].h += 2 * pad
			}
		}
		return bs, m
	}

	minS, maxS, slideS := 23*k, 44*k, 34*k
	res := &Result{FellBackTo: o.Overflow}

	// Самый крупный кегль, при котором всё влезает на одну картинку.
	fitS := 0.0
	if bs, m := lay(minS); stackHeight(bs, m.blockGap) <= avail {
		lo, hi := minS, maxS
		for hi-lo > 0.25 {
			mid := (lo + hi) / 2
			if bs, m := lay(mid); stackHeight(bs, m.blockGap) <= avail {
				lo = mid
			} else {
				hi = mid
			}
		}
		fitS = lo
	}

	var pages [][]blockOut
	var m metrics
	switch {
	case fitS > 0 && (o.Overflow == OverflowFit || fitS >= slideS):
		var bs []blockOut
		bs, m = lay(fitS)
		pages = [][]blockOut{bs}
	default:
		if o.Overflow == OverflowFit {
			res.FellBackTo = OverflowSlides // даже минимальный кегль не влез
		}
		var bs []blockOut
		bs, m = lay(slideS)
		pages = paginate(bs, avail, m.blockGap)
	}
	res.BaseSize = m.s / k

	for pi, page := range pages {
		c := r.newCanvas(o)
		r.background(c, o)
		r.header(c, o, margin, headerBottom)
		gap := m.blockGap
		h := stackHeight(page, gap)
		// Свободное место отдаём сначала интервалам между блоками, потом отступу сверху.
		if extra := avail - h; extra > 0 && len(page) > 1 {
			add := math.Min(extra/float64(len(page)-1), m.s*1.1)
			gap += add
			h += add * float64(len(page)-1)
		}
		y := top + math.Max(0, avail-h)*0.3
		for bi, b := range page {
			if bi > 0 {
				if o.Style.Separators {
					r.separator(c, margin, y+gap/2, contentW, m)
				}
				y += gap
			}
			r.drawBlock(c, o.Style, b, margin, y, contentW, m)
			y += b.h
		}
		r.footer(c, o, margin, footerH, pi, len(pages))
		res.Pages = append(res.Pages, c.img)
	}
	return res, nil
}

func (r *Renderer) newCanvas(o Options) *canvas {
	f := o.Format
	pal := darkPal
	if o.Background == BgWhite {
		pal = lightPal
	}
	return &canvas{
		pal:   pal,
		img:   image.NewRGBA(image.Rect(0, 0, f.W, f.H)),
		k:     float64(f.W) / 1080,
		fonts: r.fonts,
		emoji: r.emoji,
	}
}

func (r *Renderer) headerHeight(f Format, k float64) float64 {
	if float64(f.H)/float64(f.W) > 1.5 {
		return 210 * k
	}
	return 180 * k
}

// --- фон ---

func (r *Renderer) background(c *canvas, o Options) {
	b := c.img.Bounds()
	switch o.Background {
	case BgWhite:
		c.vGradient(b, rgb(0xFFFFFF), rgb(0xF3EEE5))
		c.glow(c.W()*0.5, 0, c.W()*0.9, rgb(0xE9D3A8), 0.35)
	case BgGraphite:
		c.vGradient(b, rgb(0x211E1A), rgb(0x0B0B0B))
		for x := -c.H(); x < c.W(); x += 46 * c.k {
			c.polyline([]pt{{x, c.H()}, {x + c.H(), 0}}, 1*c.k, withAlpha(c.pal.Gold, 0.035))
		}
		c.glow(c.W()*0.5, 0, c.W()*0.9, rgb(0x5A4320), 0.25)
	case BgPhoto:
		if o.Photo != nil {
			c.photoBackground(o.Photo)
			break
		}
		fallthrough
	default:
		c.vGradient(b, rgb(0x0E0D0B), black)
		c.glow(c.W()*0.5, c.H()*0.08, c.W()*0.95, rgb(0x4A3518), 0.28)
		c.glow(c.W()*0.15, c.H()*0.7, c.W()*0.6, rgb(0x2A1E0E), 0.25)
	}
}

// photoBackground — фото объекта на весь кадр, размытое и затемнённое,
// чтобы золотой текст читался. Размытие = уменьшить в 12 раз и растянуть обратно.
func (c *canvas) photoBackground(photo image.Image) {
	W, H := c.img.Bounds().Dx(), c.img.Bounds().Dy()
	pb := photo.Bounds()
	scale := math.Max(float64(W)/float64(pb.Dx()), float64(H)/float64(pb.Dy()))
	cw, ch := int(float64(W)/scale), int(float64(H)/scale)
	src := image.Rect(pb.Min.X+(pb.Dx()-cw)/2, pb.Min.Y+(pb.Dy()-ch)/2, 0, 0)
	src.Max = src.Min.Add(image.Pt(cw, ch))
	small := image.NewRGBA(image.Rect(0, 0, W/12+1, H/12+1))
	xdraw.ApproxBiLinear.Scale(small, small.Bounds(), photo, src, xdraw.Src, nil)
	xdraw.BiLinear.Scale(c.img, c.img.Bounds(), small, small.Bounds(), xdraw.Src, nil)
	for y := 0; y < H; y++ {
		t := float64(y) / float64(H)
		a := 0.78 - 0.12*math.Sin(t*math.Pi) // сверху и снизу темнее
		for x := 0; x < W; x++ {
			i := c.img.PixOffset(x, y)
			for ch := 0; ch < 3; ch++ {
				c.img.Pix[i+ch] = uint8(float64(c.img.Pix[i+ch]) * (1 - a))
			}
		}
	}
}

// --- шапка ---

func (r *Renderer) header(c *canvas, o Options, margin, bottom float64) {
	k := c.k
	W := c.W()
	logoH := bottom * 0.62
	cy := bottom*0.5 + 4*k

	logoRight := margin
	if o.Brand.Logo != nil {
		lb := o.Brand.Logo.Bounds()
		lw := logoH * float64(lb.Dx()) / float64(lb.Dy())
		if lw > W*0.52 {
			lw = W * 0.52
			logoH = lw * float64(lb.Dy()) / float64(lb.Dx())
		}
		dst := image.Rect(int(margin), int(cy-logoH/2), int(margin+lw), int(cy+logoH/2))
		if c.pal.TintLogo {
			c.tintedLogo(o.Brand.Logo, dst)
		} else {
			xdraw.CatmullRom.Scale(c.img, dst, o.Brand.Logo, lb, xdraw.Over, nil)
		}
		logoRight = margin + lw
	}

	// Адрес справа: гексагон с меткой + текст. Длинный адрес переносится
	// на вторую строку и при необходимости чуть уменьшается.
	phone := ""
	if o.PhoneAt == PhoneHeader {
		phone = FormatPhone(o.Phone)
	}
	hex := 92 * k
	gap := 24 * k
	avail := W - margin - (logoRight + 36*k) - hex - gap
	as, ps := 40*k, 30*k
	lines := []string{o.Brand.Address}
	if c.textWidth(o.Brand.Address, Bold, as) > avail {
		lines = splitBalanced(o.Brand.Address, func(s string) float64 { return c.textWidth(s, Bold, as) })
	}
	maxLines := bottom * 0.78
	for as > 24*k {
		fits := len(lines)*int(as*1.15) <= int(maxLines)
		for _, l := range lines {
			if c.textWidth(l, Bold, as) > avail {
				fits = false
			}
		}
		if phone != "" && len(lines) > 1 {
			fits = fits && float64(len(lines))*as*1.15+ps*1.3 <= maxLines
		}
		if fits {
			break
		}
		as -= 2 * k
		ps = math.Min(ps, as*0.8)
	}
	tw := 0.0
	for _, l := range lines {
		tw = math.Max(tw, c.textWidth(l, Bold, as))
	}
	if pw := c.textWidth(phone, SemiBold, ps); pw > tw {
		tw = pw
	}
	tx := W - margin - tw
	hx := tx - gap - hex
	head := c.pal.HeadTop
	c.glow(hx+hex/2, cy, hex*0.8, c.pal.Gold, 0.10)
	c.polyline(hexagon(hx+hex/2, cy, hex/2), 3.2*k, head)
	c.pin(hx+hex/2, cy, hex*0.5, 3.6*k, head)
	lh := as * 1.15
	total := float64(len(lines)) * lh
	if phone != "" {
		total += ps * 1.3
	}
	y := cy - total/2
	for _, l := range lines {
		c.text(l, tx, y+lh*0.5+as*0.36, Bold, as, head)
		y += lh
	}
	if phone != "" {
		c.text(phone, tx, y+ps*0.65+ps*0.36, SemiBold, ps, c.pal.Accent)
	}

	// Тонкая золотая линия со вспышкой в центре.
	ly := bottom + 6*k
	c.goldRule(margin, W-margin, ly, 2.2*k, 1)
	c.glow(W/2, ly, 150*k, c.pal.Shine, 0.35)
}

// tintedLogo рисует логотип одной заливкой-градиентом по его прозрачности:
// чётко и контрастно на светлом фоне, без «выцветших» краёв от JPG.
func (c *canvas) tintedLogo(logo image.Image, dst image.Rectangle) {
	tmp := image.NewNRGBA(image.Rect(0, 0, dst.Dx(), dst.Dy()))
	xdraw.CatmullRom.Scale(tmp, tmp.Bounds(), logo, logo.Bounds(), xdraw.Src, nil)
	mask := image.NewAlpha(tmp.Bounds())
	for i := 0; i < len(mask.Pix); i++ {
		// чуть поднимаем непрозрачность, чтобы тонкие линии не терялись
		mask.Pix[i] = uint8(math.Min(255, float64(tmp.Pix[i*4+3])*1.25))
	}
	fill := image.NewRGBA(tmp.Bounds())
	(&canvas{img: fill}).vGradient(fill.Bounds(), c.pal.HeadTop, c.pal.HeadBottom)
	draw.DrawMask(c.img, dst, fill, image.Point{}, mask, image.Point{}, draw.Over)
}

// goldRule — горизонтальная линия, к краям гаснет.
func (c *canvas) goldRule(x0, x1, y, h, strength float64) {
	for x := x0; x < x1; x++ {
		t := math.Abs((x-x0)/(x1-x0)*2 - 1)
		a := strength * (0.15 + 0.85*math.Pow(1-t, 1.4))
		c.rect(x, y-h/2, x+1, y+h/2, withAlpha(c.pal.Gold, a))
	}
}

func (r *Renderer) separator(c *canvas, margin, y, contentW float64, m metrics) {
	x0 := margin + m.iconCol + m.s*0.75
	if m.iconCol == 0 {
		x0 = margin + contentW*0.15
	}
	x1 := margin + contentW
	c.goldRule(x0, x1, y, 1.4*c.k, 0.6)
	c.circle((x0+x1)/2, y, 5*c.k, c.pal.Gold)
}

// --- блок текста ---

func (r *Renderer) drawBlock(c *canvas, st Style, b blockOut, margin, y, contentW float64, m metrics) {
	s := m.s
	x := margin
	if st.Cards {
		c.roundRect(margin, y, margin+contentW, y+b.h, s*0.6, withAlpha(c.pal.Gold, 0.30))
		c.roundRect(margin+1.5*c.k, y+1.5*c.k, margin+contentW-1.5*c.k, y+b.h-1.5*c.k, s*0.55, c.pal.Card)
		x += s * 0.5
	}
	if m.iconCol > 0 {
		if b.icon != IconAuto {
			iy := y + (s * 0.15)
			if st.Cards {
				iy = y + s*0.6
			}
			c.icon(b.icon, st.Frame, x, iy, m.iconCol, c.pal.Gold)
		}
		x += m.iconCol + s*0.75
		if st.Cards {
			x += s * 0.2
		}
	} else if st.Bar {
		c.roundRect(x, y+s*0.2, x+5*c.k, y+b.h-s*0.2, 2.5*c.k, c.pal.Gold)
		x += s * 0.9
	}
	for _, l := range b.lines {
		c.drawLine(l, x, y)
	}
}

func (c *canvas) drawLine(l lineOut, x0, y0 float64) {
	top := y0 + l.y
	base := top + l.h/2 + l.size*0.36
	x := x0 + l.x
	if l.marker {
		c.text("–", x0+l.x-l.size*0.85, base, Bold, l.size, c.pal.Gold)
	}
	for _, sp := range l.spans {
		switch sp.kind {
		case spEmoji:
			es := l.size * 1.15
			c.emojiAt(sp.emoji, x+l.size*0.04, top+l.h/2-es/2, es)
		case spWord:
			col, w := c.pal.Text, l.weight
			if l.heading {
				col = c.pal.Gold
			}
			if sp.accent {
				col = c.pal.Accent
				if w == Regular {
					w = SemiBold
				}
			}
			c.text(sp.text, x, base, w, l.size, col)
		}
		x += sp.w
	}
}

// --- низ ---

func (r *Renderer) footer(c *canvas, o Options, margin, footerH float64, page, pages int) {
	k := c.k
	W, H := c.W(), c.H()
	if o.Style.Wave {
		c.wave(H - 60*k)
	}
	if o.PhoneAt == PhoneFooter && o.Phone != "" {
		ph := FormatPhone(o.Phone)
		fs := 38 * k
		tw := c.textWidth(ph, Bold, fs)
		pw, phh := tw+90*k, 74*k
		py := H - footerH + 18*k
		px := (W - pw) / 2
		c.glow(W/2, py+phh/2, pw*0.7, c.pal.Gold, 0.18)
		c.roundRect(px, py, px+pw, py+phh, phh/2, c.pal.Gold)
		c.text(ph, px+45*k, py+phh/2+fs*0.36, Bold, fs, c.pal.OnGold)
	}
	if pages > 1 {
		label := itoa(page+1) + " / " + itoa(pages)
		c.text(label, W-margin-c.textWidth(label, SemiBold, 28*k), H-24*k, SemiBold, 28*k, c.pal.Gold)
	}
}

// wave — золотые волны с бликами, как внизу первого образца.
func (c *canvas) wave(cy float64) {
	W := c.W()
	k := c.k
	for i := 0; i < 5; i++ {
		var p []pt
		amp := (14 + float64(i)*5) * k
		ph := float64(i) * 0.5
		for x := 0.0; x <= W; x += 6 * k {
			t := x / W
			env := math.Sin(t * math.Pi) // к краям волна затухает
			p = append(p, pt{x, cy + amp*math.Sin(t*math.Pi*2.2+ph)*env})
		}
		a := 0.55 - float64(i)*0.09
		c.stroke(p, (1.1+0.4*float64(4-i))*k, withAlpha(c.pal.Gold, a))
	}
	c.glow(W*0.42, cy-4*k, 140*k, c.pal.Shine, 0.45)
	c.glow(W*0.72, cy+6*k, 90*k, c.pal.Shine, 0.3)
}

// --- утилиты ---

// FormatPhone приводит российский номер к виду +7 (920) 595-28-88.
func FormatPhone(s string) string {
	var d []rune
	for _, r := range s {
		if r >= '0' && r <= '9' {
			d = append(d, r)
		}
	}
	if len(d) == 11 && (d[0] == '8' || d[0] == '7') {
		d = d[1:]
	}
	if len(d) != 10 {
		return strings.TrimSpace(s)
	}
	n := string(d)
	return "+7 (" + n[0:3] + ") " + n[3:6] + "-" + n[6:8] + "-" + n[8:10]
}

// KeyDarkBackground делает тёмный фон логотипа прозрачным (для JPG без альфы).
func KeyDarkBackground(src image.Image, lo, hi float64) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 257
			a := (lum - lo) / (hi - lo)
			a = math.Max(0, math.Min(1, a))
			// цвет «как если бы» лого лежало на чёрном: наблюдаемое = a·цвет
			cl := func(v uint32) uint8 {
				if a < 0.01 {
					return 0
				}
				return uint8(math.Min(255, float64(v)/257/a))
			}
			out.SetNRGBA(x-b.Min.X, y-b.Min.Y, color.NRGBA{cl(r), cl(g), cl(bl), uint8(a * 255)})
		}
	}
	return out
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

// TrimTransparent обрезает прозрачные поля вокруг логотипа.
func TrimTransparent(img *image.NRGBA) image.Image {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A > 40 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX {
		return img
	}
	return img.SubImage(image.Rect(minX, minY, maxX+1, maxY+1))
}

// splitBalanced делит строку на две по пробелу так, чтобы половины были
// как можно ближе по ширине.
func splitBalanced(s string, width func(string) float64) []string {
	words := strings.Fields(s)
	if len(words) < 2 {
		return []string{s}
	}
	best, bestD := 1, math.Inf(1)
	for i := 1; i < len(words); i++ {
		a := strings.Join(words[:i], " ")
		b := strings.Join(words[i:], " ")
		if d := math.Abs(width(a) - width(b)); d < bestD {
			best, bestD = i, d
		}
	}
	return []string{strings.Join(words[:best], " "), strings.Join(words[best:], " ")}
}
