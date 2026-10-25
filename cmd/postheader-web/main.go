//go:build js && wasm

// postheader-web — тот же движок постов в браузере (WebAssembly): страница
// на телефоне отдаёт сюда текст и настройки, а получает готовые JPG.
// Работает внутри Web Worker, чтобы страница не подвисала.
package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	_ "image/png"
	"strconv"
	"syscall/js"

	"postheader/assets"
	"postheader/internal/render"
)

// params — настройки поста от страницы.
type params struct {
	Text      string         `json:"text"`
	Format    int            `json:"format"`
	Style     int            `json:"style"`
	Bg        int            `json:"bg"` // 0 чёрный, 1 белый, 2 графит, 3 фото
	PhotoX    float64        `json:"photoX"`
	PhotoY    float64        `json:"photoY"`
	PhotoZoom float64        `json:"photoZoom"`
	Sharp     int            `json:"sharp"`
	Light     int            `json:"light"`
	Slides    bool           `json:"slides"`
	Phone     string         `json:"phone"`
	PhoneName string         `json:"phoneName"`
	PhoneAt   int            `json:"phoneAt"` // 0 нет, 1 шапка, 2 низ
	Align     int            `json:"align"`
	TextSize  float64        `json:"textSize"`
	Address   string         `json:"address"`
	Quality   int            `json:"quality"`
	Icons     map[string]int `json:"icons"` // ручной выбор иконки: номер блока → вид иконки
	Inline    struct {
		On     bool    `json:"on"`
		Size   int     `json:"size"`
		Bottom bool    `json:"bottom"`
		Over   bool    `json:"over"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Blur   int     `json:"blur"`
		Light  int     `json:"light"`
		Color  int     `json:"color"`
	} `json:"inline"`
}

var rend *render.Renderer
var emojiSet *render.EmojiSet
var logo image.Image

func main() {
	fonts, err := render.LoadFonts()
	if err != nil {
		panic(err)
	}
	emoji, err := render.LoadEmoji(assets.TwemojiZip)
	if err != nil {
		panic(err)
	}
	emojiSet = emoji
	rend = render.NewRenderer(fonts, emoji)
	logo, _, _ = image.Decode(bytes.NewReader(assets.LogoAdresa))

	// phRender(paramsJSON, photoBytes|null, inlineBytes|null) → {pages: [Uint8Array…], size, error}
	js.Global().Set("phRender", js.FuncOf(func(this js.Value, args []js.Value) any {
		return doRender(args)
	}))
	// phBlocks(text) → JSON: блоки поста с автоматической иконкой и список иконок на выбор
	js.Global().Set("phBlocks", js.FuncOf(func(this js.Value, args []js.Value) any {
		return blocksJSON(args[0].String())
	}))
	js.Global().Call("phReady")
	select {}
}

func doRender(args []js.Value) (out any) {
	res := map[string]any{}
	defer func() {
		if r := recover(); r != nil {
			res["error"] = "ошибка рендера"
			out = js.ValueOf(res)
		}
	}()
	var p params
	if err := json.Unmarshal([]byte(args[0].String()), &p); err != nil {
		res["error"] = err.Error()
		return js.ValueOf(res)
	}
	o := render.Options{
		Format:    render.Formats[clamp(p.Format, len(render.Formats))],
		Style:     render.Styles[clamp(p.Style, len(render.Styles))],
		Phone:     p.Phone,
		PhoneName: p.PhoneName,
		PhoneAt:   render.PhonePlace(clamp(p.PhoneAt, 3)),
		Brand:     render.Brand{Logo: logo, Address: p.Address},
		Align:     render.Align(clamp(p.Align, 2)),
		TextSize:  p.TextSize,
		Fast:      true,
		Icons:     map[int]render.IconKind{},
	}
	for k, v := range p.Icons {
		if n, err := strconv.Atoi(k); err == nil {
			o.Icons[n] = render.IconKind(v)
		}
	}
	if p.Slides {
		o.Overflow = render.OverflowSlides
	}
	switch p.Bg {
	case 1:
		o.Background = render.BgWhite
	case 2:
		o.Background = render.BgGraphite
	case 3:
		if img := decode(args, 1); img != nil {
			o.Background, o.Photo = render.BgPhoto, img
			o.PhotoAdj = render.PhotoAdjust{X: p.PhotoX, Y: p.PhotoY, Zoom: p.PhotoZoom, Sharp: p.Sharp, Light: p.Light}
		}
	}
	if p.Inline.On {
		if img := decode(args, 2); img != nil {
			o.Inline = render.InlinePhoto{Image: img, Size: p.Inline.Size, Bottom: p.Inline.Bottom, Over: p.Inline.Over,
				X: p.Inline.X, Y: p.Inline.Y, Blur: p.Inline.Blur, Light: p.Inline.Light, Color: p.Inline.Color}
		}
	}
	r, err := rend.Render(p.Text, o)
	if err != nil {
		res["error"] = err.Error()
		return js.ValueOf(res)
	}
	q := p.Quality
	if q < 40 || q > 100 {
		q = 88
	}
	pages := make([]any, 0, len(r.Pages))
	for _, pg := range r.Pages {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, pg, &jpeg.Options{Quality: q}); err != nil {
			res["error"] = err.Error()
			return js.ValueOf(res)
		}
		u8 := js.Global().Get("Uint8Array").New(buf.Len())
		js.CopyBytesToJS(u8, buf.Bytes())
		pages = append(pages, u8)
	}
	res["pages"] = pages
	res["size"] = r.BaseSize
	return js.ValueOf(res)
}

func blocksJSON(text string) string {
	type blk struct {
		Title string `json:"title"`
		Icon  int    `json:"icon"`
	}
	type choice struct {
		Kind int    `json:"kind"`
		Name string `json:"name"`
	}
	out := struct {
		Blocks  []blk    `json:"blocks"`
		Choices []choice `json:"choices"`
	}{Blocks: []blk{}}
	for _, b := range render.Blocks(text, emojiSet) {
		out.Blocks = append(out.Blocks, blk{b.Title, int(b.Icon)})
	}
	for _, c := range render.IconChoices {
		out.Choices = append(out.Choices, choice{int(c.Kind), c.Name})
	}
	data, _ := json.Marshal(out)
	return string(data)
}

func decode(args []js.Value, i int) image.Image {
	if len(args) <= i || args[i].IsNull() || args[i].IsUndefined() {
		return nil
	}
	b := make([]byte, args[i].Length())
	js.CopyBytesToGo(b, args[i])
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	return img
}

func clamp(v, n int) int {
	if v < 0 || v >= n {
		return 0
	}
	return v
}
