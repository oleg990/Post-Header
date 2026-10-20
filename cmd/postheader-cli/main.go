// postheader-cli — консольный прототип рендера: текст → N вариантов JPG.
// В десктоп-приложении тот же пакет render вызывается из GUI.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"postheader/assets"
	"postheader/internal/render"
)

func main() {
	in := flag.String("in", "-", "файл с текстом поста (- = stdin)")
	out := flag.String("out", "out", "папка для JPG")
	n := flag.Int("n", 4, "сколько вариантов оформления получить")
	formats := flag.String("format", "story", "форматы через запятую: story, portrait, square, all")
	bg := flag.String("bg", "black", "фон: black, white, graphite, photo")
	photo := flag.String("photo", "", "фото объекта для -bg photo")
	logo := flag.String("logo", "", "логотип (PNG с прозрачностью или JPG на тёмном фоне)")
	address := flag.String("address", "мкр. Степной 18", "адрес в шапке")
	phone := flag.String("phone", "", "телефон для этого поста")
	phoneAt := flag.String("phone-at", "footer", "где телефон: header или footer")
	slides := flag.Bool("slides", false, "длинный текст делить на слайды, а не уменьшать шрифт")
	quality := flag.Int("q", 92, "качество JPG")
	fontRegular := flag.String("font", "", "свой шрифт .ttf (обычный), например C:\\Windows\\Fonts\\arial.ttf")
	fontBold := flag.String("font-bold", "", "свой шрифт .ttf (жирный)")
	icons := flag.String("icons", "", "иконки вручную: номер_блока=иконка через запятую, например 1=gift,4=house")
	flag.Parse()

	text, err := readText(*in)
	check(err)
	var fs render.FontSet
	if *fontRegular != "" {
		fs.Regular, err = os.ReadFile(*fontRegular)
		check(err)
	}
	if *fontBold != "" {
		fs.Bold, err = os.ReadFile(*fontBold)
		check(err)
	}
	fonts, err := render.LoadFontSet(fs)
	check(err)
	emoji, err := render.LoadEmoji(assets.TwemojiZip)
	check(err)
	r := render.NewRenderer(fonts, emoji)

	o := render.Options{Brand: render.Brand{Address: *address}, Phone: *phone}
	if *phone != "" {
		o.PhoneAt = render.PhoneFooter
		if *phoneAt == "header" {
			o.PhoneAt = render.PhoneHeader
		}
	}
	if *slides {
		o.Overflow = render.OverflowSlides
	}
	o.Icons, err = parseIcons(*icons)
	check(err)
	switch *bg {
	case "white":
		o.Background = render.BgWhite
	case "graphite":
		o.Background = render.BgGraphite
	case "photo":
		o.Background = render.BgPhoto
		o.Photo, err = loadImage(*photo)
		check(err)
	}
	if *logo != "" {
		img, err := loadImage(*logo)
		check(err)
		if strings.HasSuffix(strings.ToLower(*logo), ".jpg") || strings.HasSuffix(strings.ToLower(*logo), ".jpeg") {
			img = render.TrimTransparent(render.KeyDarkBackground(img, 45, 110))
		}
		o.Brand.Logo = img
	}

	check(os.MkdirAll(*out, 0o755))
	styles := render.Styles
	if *n < len(styles) {
		styles = styles[:*n]
	}
	for _, f := range pickFormats(*formats) {
		for _, st := range styles {
			o.Format, o.Style = f, st
			res, err := r.Render(text, o)
			check(err)
			for i, page := range res.Pages {
				name := fmt.Sprintf("%s_%s_%s", f.ID, st.ID, *bg)
				if len(res.Pages) > 1 {
					name += fmt.Sprintf("_%d", i+1)
				}
				path := filepath.Join(*out, name+".jpg")
				check(saveJPEG(path, page, *quality))
				fmt.Printf("%s  (кегль %.0f px)\n", path, res.BaseSize)
			}
		}
	}
}

func pickFormats(s string) []render.Format {
	if s == "all" {
		return render.Formats
	}
	var out []render.Format
	for _, id := range strings.Split(s, ",") {
		for _, f := range render.Formats {
			if f.ID == strings.TrimSpace(id) {
				out = append(out, f)
			}
		}
	}
	return out
}

func parseIcons(s string) (map[int]render.IconKind, error) {
	out := map[int]render.IconKind{}
	for _, pair := range strings.Split(s, ",") {
		if strings.TrimSpace(pair) == "" {
			continue
		}
		var n int
		var id string
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("иконка %q: нужен вид номер=иконка", pair)
		}
		if _, err := fmt.Sscan(parts[0], &n); err != nil {
			return nil, fmt.Errorf("иконка %q: %w", pair, err)
		}
		id = strings.TrimSpace(parts[1])
		found := false
		for _, ic := range render.IconChoices {
			if ic.ID == id {
				out[n-1] = ic.Kind // в CLI блоки считаем с 1
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("неизвестная иконка %q", id)
		}
	}
	return out, nil
}

func readText(path string) (string, error) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	return string(b), err
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func saveJPEG(path string, img image.Image, q int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: q})
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
