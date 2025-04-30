package purefn_test

import (
	"fmt"
	"testing"

	"github.com/on-the-ground/effect_ive_go/purefn"
)

func naiveFib(n int) int {
	if n <= 1 {
		return n
	}
	return naiveFib(n-1) + naiveFib(n-2)
}

func BenchmarkNaiveFib20(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = naiveFib(20)
	}
}

func BenchmarkTableizedFib20(b *testing.B) {
	var tableFib func(int) int
	tableFib = purefn.TableizeI1O1(func(n int) int {
		if n <= 1 {
			return n
		}
		return tableFib(n-1) + tableFib(n-2)
	}, 32)

	for i := 0; i < b.N; i++ {
		_ = tableFib(20)
	}
}

func naiveLevenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	if a[0] == b[0] {
		return naiveLevenshtein(a[1:], b[1:])
	}
	return 1 + min(
		naiveLevenshtein(a[1:], b),
		naiveLevenshtein(a, b[1:]),
		naiveLevenshtein(a[1:], b[1:]),
	)
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func BenchmarkNaiveLevenshtein(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = naiveLevenshtein("kitten", "sitting")
	}
}

func BenchmarkTableizedLevenshtein(b *testing.B) {
	sizes := []uint32{2, 8, 32}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("TrieSize_%d", size), func(b *testing.B) {
			var lev func(string, string) int
			lev = purefn.TableizeI2O1(func(a, b string) int {
				if len(a) == 0 {
					return len(b)
				}
				if len(b) == 0 {
					return len(a)
				}
				if a[0] == b[0] {
					return lev(a[1:], b[1:])
				}
				return 1 + min(
					lev(a[1:], b),
					lev(a, b[1:]),
					lev(a[1:], b[1:]),
				)
			}, size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = lev("kitten", "sitting")
			}
		})
	}
}

type Point struct {
	X, Y float64
}

func naiveDist(p1, p2 Point) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return dx*dx + dy*dy
}

func BenchmarkNaiveDist(b *testing.B) {
	p1 := Point{1.5, 2.5}
	p2 := Point{3.0, 4.0}
	for i := 0; i < b.N; i++ {
		_ = naiveDist(p1, p2)
	}
}

func BenchmarkTableizedDist(b *testing.B) {
	var dist func(Point, Point) float64
	dist = purefn.TableizeI2O1(func(p1, p2 Point) float64 {
		dx := p1.X - p2.X
		dy := p1.Y - p2.Y
		return dx*dx + dy*dy
	}, 32)

	p1 := Point{1.5, 2.5}
	p2 := Point{3.0, 4.0}
	for i := 0; i < b.N; i++ {
		_ = dist(p1, p2)
	}
}
