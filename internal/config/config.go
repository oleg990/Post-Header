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
	PhoneName  string `json:"phone_name,omitempty"`
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

	// Профиль сотрудника: спрашивается при первом запуске.
	UserName  string `json:"user_name,omitempty"`
	UserPhone string `json:"user_phone,omitempty"`
	Welcomed  bool   `json:"welcomed,omitempty"`
	NewsSeen  bool   `json:"news_seen,omitempty"` // окно «Что нового» после обновления уже показано

	UserWords []string `json:"user_words,omitempty"` // личный словарь для проверки орфографии
	Align     int      `json:"align,omitempty"`      // 0 слева, 1 по центру

	// Предсказание дня: на какой день открыто, какая фраза, какие уже были.
	OracleDay   string `json:"oracle_day,omitempty"`
	OracleIdx   int    `json:"oracle_idx,omitempty"`
	OracleSeen  []int  `json:"oracle_seen,omitempty"`
	OracleSign  int    `json:"oracle_sign,omitempty"`  // знак дня + 1; 0 — ещё не выбран
	OracleTheme int    `json:"oracle_theme,omitempty"` // оформление + 1; 0 — ещё не выбрано
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
