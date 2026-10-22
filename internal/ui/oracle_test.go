package ui

import (
	"math/rand"
	"testing"
)

func TestNextLookAlwaysChanges(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for _, n := range []int{len(oracleSigns), len(oracleThemes)} {
		prev := 0
		hit := map[int]bool{}
		for i := 0; i < 500; i++ {
			v := nextLook(prev, n, r)
			if v == prev || v < 0 || v >= n {
				t.Fatalf("n=%d: после %d выпало %d", n, prev, v)
			}
			hit[v], prev = true, v
		}
		if len(hit) != n {
			t.Fatalf("n=%d: выпали не все варианты: %v", n, hit)
		}
	}
}
