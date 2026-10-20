// Package fontscan находит установленные шрифты с кириллицей и группирует
// их по семействам (обычный + жирный файл), чтобы выбрать шрифт в настройках.
package fontscan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/image/font/sfnt"

	"postheader/internal/render"
)

type Family struct {
	Name          string
	Regular, Bold string // пути к файлам; Bold может быть пустым
}

// Scan обходит папки и возвращает семейства, отсортированные по имени.
func Scan(dirs []string) []Family {
	byName := map[string]*Family{}
	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.EqualFold(filepath.Ext(path), ".ttf") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil || !render.HasCyrillic(data) {
				return nil
			}
			f, err := sfnt.Parse(data)
			if err != nil {
				return nil
			}
			var buf sfnt.Buffer
			family, _ := f.Name(&buf, sfnt.NameIDFamily)
			sub, _ := f.Name(&buf, sfnt.NameIDSubfamily)
			if family == "" {
				return nil
			}
			fam := byName[family]
			if fam == nil {
				fam = &Family{Name: family}
				byName[family] = fam
			}
			switch strings.ToLower(sub) {
			case "regular", "normal", "book", "обычный":
				fam.Regular = path
			case "bold", "полужирный":
				fam.Bold = path
			}
			return nil
		})
	}
	var out []Family
	for _, f := range byName {
		if f.Regular != "" {
			out = append(out, *f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
