package spell

import (
	"os"
	"testing"
	"time"

	"postheader/assets"
)

func TestCheck(t *testing.T) {
	s := time.Now()
	c, err := Load(assets.SpellDic, assets.SpellAff)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("load %v, %d words", time.Since(s), len(c.words))
	for _, w := range []string{"квартир", "новостройках", "Эксклюзивное", "военнослужащих", "демобилизованных", "БЕСПЛАТНО", "сопровождению", "застройщиками", "дистанционно", "благодарными", "Риелтор", "ипотечному", "по-новому", "ещё", "передачи"} {
		if !c.Correct(w) {
			t.Errorf("%q должно быть верным", w)
		}
	}
	for _, w := range []string{"кваритра", "новостройкох", "сапровождение"} {
		if c.Correct(w) {
			t.Errorf("%q должно быть ошибкой", w)
		}
		t.Logf("%s → %v", w, c.Suggest(w, 3))
	}
	if b, err := os.ReadFile("/tmp/claude-0/t/long.txt"); err == nil {
		s := time.Now()
		ms := c.Check(string(b), 20)
		t.Logf("check %v", time.Since(s))
		for _, m := range ms {
			t.Logf("ошибка: %s %v", m.Word, m.Suggest)
		}
	}
}
