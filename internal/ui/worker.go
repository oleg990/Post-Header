package ui

import (
	"image"
	"os"

	xdraw "golang.org/x/image/draw"

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
}

type variant struct {
	style    render.Style
	pages    []*image.RGBA
	previews []image.Image // уменьшенные копии для показа в окне
	slides   bool          // текст не влез и разбит на слайды
	base     float64       // кегль основного текста, px на 1080
}

type result struct {
	gen      int
	variants []variant
	err      error
}

type worker struct {
	jobs    chan job
	results chan result
	wake    func()

	emoji    *render.EmojiSet
	renderer *render.Renderer
	font     fontChoice
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
		jobs:     make(chan job, 1),
		results:  make(chan result, 4),
		wake:     wake,
		emoji:    emoji,
		renderer: render.NewRenderer(fonts, emoji),
	}
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
		res := result{gen: j.gen}
		if err := w.useFont(j.font); err != nil {
			res.err = err
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
	w.renderer = render.NewRenderer(fonts, w.emoji)
	w.font = f
	return nil
}

func (w *worker) render(j job) ([]variant, error) {
	var out []variant
	for i, st := range render.Styles {
		if i >= j.variants {
			break
		}
		// Новое задание пришло — эту работу дальше не делаем.
		if len(w.jobs) > 0 {
			return out, nil
		}
		o := j.opts
		o.Style = st
		res, err := w.renderer.Render(j.text, o)
		if err != nil {
			return nil, err
		}
		v := variant{style: st, pages: res.Pages, base: res.BaseSize}
		v.slides = len(res.Pages) > 1
		for _, p := range res.Pages {
			v.previews = append(v.previews, preview(p, 620))
		}
		out = append(out, v)
	}
	return out, nil
}

// preview уменьшает картинку качественным фильтром: в окне она выглядит
// чётко, а не «лесенкой», как при масштабировании видеокартой.
func preview(src *image.RGBA, width int) image.Image {
	b := src.Bounds()
	h := b.Dy() * width / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, width, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}
