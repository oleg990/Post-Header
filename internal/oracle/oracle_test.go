package oracle

import (
	"math/rand"
	"testing"
	"time"
)

func TestDayAndNext(t *testing.T) {
	loc := time.Local
	before := time.Date(2026, 10, 23, 8, 59, 0, 0, loc)
	after := time.Date(2026, 10, 23, 9, 0, 0, 0, loc)
	if Day(before) != "2026-10-22" || Day(after) != "2026-10-23" {
		t.Fatalf("day: %s %s", Day(before), Day(after))
	}
	if !Next(before).Equal(after) {
		t.Fatalf("next before 9: %v", Next(before))
	}
	if !Next(after).Equal(after.AddDate(0, 0, 1)) {
		t.Fatalf("next after 9: %v", Next(after))
	}
}

func TestPickNoRepeats(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	var seen []int
	got := map[int]bool{}
	for i := 0; i < len(Phrases); i++ {
		var p int
		p, seen = Pick(seen, r)
		if got[p] {
			t.Fatalf("повтор %d на шаге %d", p, i)
		}
		got[p] = true
	}
	if _, seen = Pick(seen, r); len(seen) != 1 {
		t.Fatalf("после полного круга список должен начаться заново, а в нём %d", len(seen))
	}
}
