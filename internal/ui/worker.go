package ui

import (
	"image"
	"os"
	"sync"

	"postheader/assets"
	"postheader/internal/render"
)

// Рендер идёт в отдельной горутине: окно не замирает, а при быстром наборе
// текста старые задания отбрасываются — рисуется только последнее.

type fontChoice struct {
	Regular, Bold string // пути; пусто — встроенный Inter
}

type job struct {
	gen      int
	text     string
	opts     render.Options
	variants int
	font     fontChoice
	// Быстрый кадр при перетаскивании фото: рисуется только один вариант
	// и в половинном размере. -1 — обычный полный рендер.
	only, page int
}

type variant struct {
	style    render.Style
	pages    []*image.RGBA
	previews []image.Image   // уменьшенные копии для показа в окне
	slides   bool            // текст не влез и разбит на слайды
	base     float64         // кегль основного текста, px на 1080
	inline   image.Rectangle // фото в посте на первом слайде, px картинки
}

type result struct {
	gen      int
	only     int         // быстрый кадр для одной карточки (-1 — полный рендер)
	fast     image.Image // его превью
	variants []variant
	err      error
}

type worker struct {
	jobs    chan job
	results chan result
	wake    func()

	emoji     *render.EmojiSet
	renderers []*render.Renderer // по одному на вариант: варианты рисуются параллельно
	font      fontChoice
}

func newWorker(wake func()) (*worker, error) {
	emoji, err := render.LoadEmoji(assets.TwemojiZip)
	if err != nil {
		return nil, err
	}
	fonts, err := render.LoadFonts()
	if err != nil {
		return nil, err
	}
	w := &worker{
		jobs:    make(chan job, 1),
		results: make(chan result, 4),
		wake:    wake,
		emoji:   emoji,
	}
	w.setFonts(fonts)
	go w.loop()
	return w, nil
}

// submit кладёт задание, вытесняя ещё не начатое.
func (w *worker) submit(j job) {
	select {
	case <-w.jobs:
	default:
	}
	w.jobs <- j
}

func (w *worker) loop() {
	for j := range w.jobs {
		res := result{gen: j.gen, only: j.only}
		if err := w.useFont(j.font); err != nil {
			res.err = err
		} else if j.only >= 0 {
			res.fast, res.err = w.renderFast(j)
		} else {
			res.variants, res.err = w.render(j)
		}
		w.results <- res
		w.wake()
	}
}

func (w *worker) useFont(f fontChoice) error {
	if f == w.font {
		return nil
	}
	var fs render.FontSet
	var err error
	if f.Regular != "" {
		if fs.Regular, err = os.ReadFile(f.Regular); err != nil {
			return err
		}
	}
	if f.Bold != "" {
		if fs.Bold, err = os.ReadFile(f.Bold); err != nil {
			return err
		}
	}
	fonts, err := render.LoadFontSet(fs)
	if err != nil {
		return err
	}
	w.setFonts(fonts)
	w.font = f
	return nil
}

func (w *worker) setFonts(f *render.Fonts) {
	w.renderers = w.renderers[:0]
	for range render.Styles {
		w.renderers = append(w.renderers, render.NewRenderer(f.Clone(), w.emoji))
	}
}

func (w *worker) render(j job) ([]variant, error) {
	n := j.variants
	if n > len(render.Styles) {
		n = len(render.Styles)
	}
	out := make([]variant, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o := j.opts
			o.Style = render.Styles[i]
			res, err := w.renderers[i].Render(j.text, o)
			if err != nil {
				errs[i] = err
				return
			}
			v := variant{style: o.Style, pages: res.Pages, base: res.BaseSize, inline: res.Inline}
			v.slides = len(res.Pages) > 1
			for _, p := range res.Pages {
				v.previews = append(v.previews, preview(p))
			}
			out[i] = v
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// renderFast рисует один вариант вполовину меньше: для живого превью,
// пока фото тянут мышкой. Полный рендер идёт после отпускания кнопки.
func (w *worker) renderFast(j job) (image.Image, error) {
	o := j.opts
	o.Style = render.Styles[j.only]
	o.Format.W, o.Format.H = o.Format.W/2, o.Format.H/2
	o.Fast = true
	res, err := w.renderers[j.only].Render(j.text, o)
	if err != nil {
		return nil, err
	}
	p := j.page
	if p >= len(res.Pages) {
		p = 0
	}
	return res.Pages[p], nil
}

// preview уменьшает картинку вдвое усреднением 2×2: в окне она выглядит
// чётко, а считается в разы быстрее фильтров x/image (важно для 32-битной сборки).
func preview(src *image.RGBA) image.Image {
	b := src.Bounds()
	w, h := b.Dx()/2, b.Dy()/2
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		r0 := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+2*y):]
		r1 := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+2*y+1):]
		d := dst.Pix[y*dst.Stride:]
		for x := 0; x < w; x++ {
			i, j := 8*x, 4*x
			for c := 0; c < 4; c++ {
				d[j+c] = uint8((uint32(r0[i+c]) + uint32(r0[i+4+c]) + uint32(r1[i+c]) + uint32(r1[i+4+c]) + 2) >> 2)
			}
		}
	}
	return dst
}
