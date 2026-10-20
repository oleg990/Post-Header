package render

import (
	"os"
	"testing"

	"postheader/assets"
)

func newTestRenderer(t *testing.T) *Renderer {
	t.Helper()
	f, err := LoadFonts()
	if err != nil {
		t.Fatal(err)
	}
	e, err := LoadEmoji(assets.TwemojiZip)
	if err != nil {
		t.Fatal(err)
	}
	return NewRenderer(f, e)
}

func TestRenderAllCombinations(t *testing.T) {
	text, err := os.ReadFile("../../examples/post1.txt")
	if err != nil {
		t.Fatal(err)
	}
	r := newTestRenderer(t)
	for _, f := range Formats {
		for _, st := range Styles {
			for _, bg := range []BackgroundKind{BgBlack, BgWhite, BgGraphite} {
				for _, ov := range []Overflow{OverflowFit, OverflowSlides} {
					res, err := r.Render(string(text), Options{Format: f, Style: st, Background: bg, Overflow: ov, Phone: "89205952888", PhoneAt: PhoneFooter})
					if err != nil {
						t.Fatalf("%s/%s: %v", f.ID, st.ID, err)
					}
					if len(res.Pages) == 0 {
						t.Fatalf("%s/%s: no pages", f.ID, st.ID)
					}
				}
			}
		}
	}
}

func TestEmojiMatch(t *testing.T) {
	r := newTestRenderer(t)
	cases := map[string]string{"✅": "2705", "❤️": "2764", "1️⃣": "31-20e3", "👨‍👩‍👧": "1f468-200d-1f469-200d-1f467"}
	for in, want := range cases {
		if got, _ := r.emoji.Match([]rune(in), 0); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	if got, n := r.emoji.Match([]rune("1 комната"), 0); n != 0 {
		t.Errorf("plain digit matched as emoji %q", got)
	}
}

func TestFormatPhone(t *testing.T) {
	for _, in := range []string{"89205952888", "+7 920 595 28 88", "9205952888"} {
		if got := FormatPhone(in); got != "+7 (920) 595-28-88" {
			t.Errorf("%q → %q", in, got)
		}
	}
}
