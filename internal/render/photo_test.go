package render

import (
	"image"
	"image/color"
	"testing"
)

func testPhoto() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 4), uint8(y / 3), 140, 255})
		}
	}
	return img
}

// Пост из одного фото без текста и текст поверх фото — во всех стилях.
func TestPhotoPosts(t *testing.T) {
	r := newTestRenderer(t)
	ph := testPhoto()
	for _, st := range Styles {
		res, err := r.Render("", Options{Format: Formats[1], Style: st, Inline: InlinePhoto{Image: ph}})
		if err != nil || len(res.Pages) != 1 || res.Inline.Empty() {
			t.Fatalf("%s без текста: %v", st.ID, err)
		}
		res, err = r.Render("   ", Options{Format: Formats[1], Style: st, Background: BgPhoto, Photo: ph})
		if err != nil || len(res.Pages) != 1 {
			t.Fatalf("%s фон без текста: %v", st.ID, err)
		}
		res, err = r.Render("{++Продаётся} квартира\nУютная и светлая", Options{Format: Formats[1], Style: st, Inline: InlinePhoto{Image: ph, Over: true}})
		if err != nil || len(res.Pages) != 1 {
			t.Fatalf("%s поверх: %v", st.ID, err)
		}
	}
	if _, err := r.Render("", Options{Format: Formats[1], Style: Styles[0]}); err == nil {
		t.Fatal("пустой пост без фото должен давать ошибку")
	}
}
