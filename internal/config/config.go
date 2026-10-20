// Package config хранит настройки в %APPDATA%\PostHeader\config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	LogoPath   string `json:"logo_path,omitempty"` // пусто — встроенный логотип «Адреса»
	Address    string `json:"address"`
	Phone      string `json:"phone,omitempty"`
	PhoneAt    int    `json:"phone_at"` // 0 нет, 1 шапка, 2 низ
	Format     int    `json:"format"`
	Background int    `json:"background"`
	Overflow   int    `json:"overflow"`
	Variants   int    `json:"variants"`
	FontName   string `json:"font_name,omitempty"`
	FontPath   string `json:"font_path,omitempty"`
	FontBold   string `json:"font_bold,omitempty"`
	OutDir     string `json:"out_dir,omitempty"`
	Draft      string `json:"draft,omitempty"` // автосохранение текста
}

func Default() Config {
	return Config{Address: "мкр. Степной 18", PhoneAt: 2, Variants: 4}
}

func path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "PostHeader", "config.json")
}

func Load() Config {
	c := Default()
	data, err := os.ReadFile(path())
	if err == nil {
		json.Unmarshal(data, &c)
	}
	if c.Variants < 1 || c.Variants > 4 {
		c.Variants = 4
	}
	return c
}

func Save(c Config) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
