// Package history хранит последние готовые посты: текст, все настройки и
// миниатюру, чтобы пост можно было открыть и доработать.
package history

import (
	"encoding/json"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Max — сколько постов помнить.
const Max = 10

type Entry struct {
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
	Text string    `json:"text"`

	Phone     string `json:"phone,omitempty"`
	PhoneName string `json:"phone_name,omitempty"`
	PhoneAt   int    `json:"phone_at"`

	Format     int         `json:"format"`
	Background int         `json:"background"`
	Overflow   int         `json:"overflow"`
	Align      int         `json:"align"`
	TextSize   float64     `json:"text_size,omitempty"`
	Icons      map[int]int `json:"icons,omitempty"`

	PhotoPath    string  `json:"photo_path,omitempty"` // фон
	PhotoX       float64 `json:"photo_x,omitempty"`
	PhotoY       float64 `json:"photo_y,omitempty"`
	PhotoZoom    float64 `json:"photo_zoom,omitempty"`
	PhotoSharp   int     `json:"photo_sharp,omitempty"`
	PhotoLight   int     `json:"photo_light,omitempty"`
	InlinePath   string  `json:"inline_path,omitempty"` // фото в посте
	InlineSize   int     `json:"inline_size,omitempty"`
	InlineBottom bool    `json:"inline_bottom,omitempty"`
	InlineOver   bool    `json:"inline_over,omitempty"`
	InlineY      float64 `json:"inline_y,omitempty"`
	InlineX      float64 `json:"inline_x,omitempty"`
}

// Title — первая строка текста для списка.
func (e Entry) Title() string {
	for _, l := range strings.Split(e.Text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "Без текста"
}

func dir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "PostHeader", "history")
}

// ThumbPath — миниатюра поста.
func ThumbPath(id string) string { return filepath.Join(dir(), id+".jpg") }

func Load() []Entry {
	var es []Entry
	if b, err := os.ReadFile(filepath.Join(dir(), "history.json")); err == nil {
		json.Unmarshal(b, &es)
	}
	return es
}

func save(es []Entry) error {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(es, "", "  ")
	p := filepath.Join(dir(), "history.json")
	if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

// Add кладёт пост в начало истории. Тот же текст не дублируется — запись
// обновляется и поднимается наверх. Лишние старые записи удаляются.
func Add(e Entry, thumb image.Image) ([]Entry, error) {
	es := Load()
	key := strings.TrimSpace(e.Text)
	for i := 0; i < len(es); i++ {
		if strings.TrimSpace(es[i].Text) == key {
			os.Remove(ThumbPath(es[i].ID))
			es = append(es[:i], es[i+1:]...)
			i--
		}
	}
	e.Time = time.Now()
	e.ID = strconv.FormatInt(e.Time.UnixNano(), 36)
	es = append([]Entry{e}, es...)
	for len(es) > Max {
		os.Remove(ThumbPath(es[len(es)-1].ID))
		es = es[:len(es)-1]
	}
	if err := save(es); err != nil {
		return es, err
	}
	if thumb != nil {
		if f, err := os.Create(ThumbPath(e.ID)); err == nil {
			jpeg.Encode(f, thumb, &jpeg.Options{Quality: 85})
			f.Close()
		}
	}
	return es, nil
}

// Remove удаляет запись.
func Remove(id string) ([]Entry, error) {
	es := Load()
	for i := range es {
		if es[i].ID == id {
			os.Remove(ThumbPath(id))
			es = append(es[:i], es[i+1:]...)
			break
		}
	}
	return es, save(es)
}
