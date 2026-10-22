// Package update проверяет новые версии на GitHub Releases и скачивает
// установщик. Установщик сам заменяет программу и запускает её снова.
package update

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo — откуда берутся обновления.
const Repo = "oleg990/Post-Header"

// Release — свежая версия, которую можно поставить.
type Release struct {
	Tag   string // "v0.2.0"
	Notes string // описание релиза
	URL   string // ссылка на установщик
	Name  string // имя файла установщика
	Size  int64
}

var client = &http.Client{Timeout: 30 * time.Second}

// updateAsset — имя облегчённого установщика для этой сборки.
func updateAsset() string {
	if runtime.GOARCH == "386" {
		return "PostHeader-Update-win32.exe"
	}
	return "PostHeader-Update-win64.exe"
}

// Check возвращает новый релиз, если он новее current, иначе nil.
// Сборки не из релиза (dev-…) не обновляются.
func Check(current string) (*Release, error) {
	if _, ok := parse(current); !ok {
		return nil, nil
	}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PostHeader/"+current)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("GitHub ответил " + resp.Status)
	}
	var r struct {
		Tag    string `json:"tag_name"`
		Body   string `json:"body"`
		Draft  bool   `json:"draft"`
		Pre    bool   `json:"prerelease"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if r.Draft || r.Pre || !Newer(r.Tag, current) {
		return nil, nil
	}
	// Сначала облегчённый установщик под свою разрядность (он вдвое меньше),
	// если его нет — полный.
	var rel *Release
	for _, a := range r.Assets {
		if a.Name == updateAsset() || rel == nil && strings.HasPrefix(a.Name, "PostHeader-Setup") && strings.HasSuffix(a.Name, ".exe") {
			rel = &Release{Tag: r.Tag, Notes: r.Body, URL: a.URL, Name: a.Name, Size: a.Size}
			if a.Name == updateAsset() {
				break
			}
		}
	}
	return rel, nil
}

// Download сохраняет установщик во временную папку; progress получает 0..1.
func Download(rel *Release, progress func(float32)) (string, error) {
	dl := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest("GET", rel.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PostHeader")
	resp, err := dl.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("не удалось скачать: " + resp.Status)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = rel.Size
	}
	dir := filepath.Join(os.TempDir(), "PostHeader-update")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(rel.Name))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return "", err
			}
			done += int64(n)
			if total > 0 && progress != nil {
				progress(float32(done) / float32(total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if rel.Size > 0 && done != rel.Size {
		return "", errors.New("файл скачался не полностью, попробуйте ещё раз")
	}
	return path, nil
}

// Newer сообщает, что версия a новее b ("v1.2.10" > "v1.2.9").
func Newer(a, b string) bool {
	va, ok1 := parse(a)
	vb, ok2 := parse(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	if !strings.HasPrefix(v, "v") {
		return out, false
	}
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
