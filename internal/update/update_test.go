package update

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.2.0", "v0.1.0", true},
		{"v0.1.10", "v0.1.9", true},
		{"v1", "v0.9.9", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v0.2.0", "dev-abc1234", false},
		{"v0.2.0-beta", "v0.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestDownloadReuses(t *testing.T) {
	body := []byte("установщик")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(body)
	}))
	defer srv.Close()
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("TMP", os.Getenv("TMPDIR"))
	rel := &Release{Tag: "v9.9.9", URL: srv.URL, Name: "PostHeader-Update-win64.exe", Size: int64(len(body))}
	p1, err := Download(rel, nil)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Download(rel, nil)
	if err != nil || p1 != p2 || hits != 1 {
		t.Fatalf("второй раз должен взять готовый файл: %v %q %q hits=%d", err, p1, p2, hits)
	}
	if b, _ := os.ReadFile(p2); string(b) != string(body) {
		t.Fatalf("содержимое %q", b)
	}
}
