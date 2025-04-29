package pure_test

import (
	"fmt"
	"testing"

	"github.com/on-the-ground/effect_ive_go/pure"

	"github.com/stretchr/testify/assert"
)

func TestTableizeI1O1(t *testing.T) {
	count := 0
	fn := pure.TableizeI1O1(func(i int) int {
		count++
		return i * 2
	}, 2)

	assert.Equal(t, 4, fn(2))
	assert.Equal(t, 4, fn(2)) // cached
	assert.Equal(t, 1, count)
}

func TestTableizeI2O1(t *testing.T) {
	count := 0
	fn := pure.TableizeI2O1(func(a, b int) int {
		count++
		return a + b
	}, 2)

	assert.Equal(t, 5, fn(2, 3))
	assert.Equal(t, 5, fn(2, 3))
	assert.Equal(t, 1, count)
}

func TestTableizeI3O1(t *testing.T) {
	count := 0
	fn := pure.TableizeI3O1(func(a, b, c int) int {
		count++
		return a * b * c
	}, 2)

	assert.Equal(t, 24, fn(2, 3, 4))
	assert.Equal(t, 24, fn(2, 3, 4))
	assert.Equal(t, 1, count)
}

func TestTableizeI4O1(t *testing.T) {
	count := 0
	fn := pure.TableizeI4O1(func(a, b, c, d int) int {
		count++
		return a + b + c + d
	}, 2)

	assert.Equal(t, 10, fn(1, 2, 3, 4))
	assert.Equal(t, 10, fn(1, 2, 3, 4))
	assert.Equal(t, 1, count)
}

func TestTableizeI1O2(t *testing.T) {
	count := 0
	fn := pure.TableizeI1O2(func(i int) (int, string) {
		count++
		return i, "val"
	}, 2)

	a, b := fn(10)
	assert.Equal(t, 10, a)
	assert.Equal(t, "val", b)
	a2, b2 := fn(10)
	assert.Equal(t, 10, a2)
	assert.Equal(t, "val", b2)
	assert.Equal(t, 1, count)
}

func TestTableizeI2O2(t *testing.T) {
	count := 0
	fn := pure.TableizeI2O2(func(a, b int) (int, string) {
		count++
		return a * b, "mul"
	}, 2)

	x, y := fn(3, 4)
	assert.Equal(t, 12, x)
	assert.Equal(t, "mul", y)
	_, _ = fn(3, 4)
	assert.Equal(t, 1, count)
}

func TestTableizeI3O2(t *testing.T) {
	count := 0
	fn := pure.TableizeI3O2(func(a, b, c int) (int, string) {
		count++
		return a + b + c, "sum"
	}, 2)

	x, y := fn(1, 2, 3)
	assert.Equal(t, 6, x)
	assert.Equal(t, "sum", y)
	_, _ = fn(1, 2, 3)
	assert.Equal(t, 1, count)
}

func TestTableizeI4O2(t *testing.T) {
	count := 0
	fn := pure.TableizeI4O2(func(a, b, c, d int) (int, string) {
		count++
		return a * b * c * d, "product"
	}, 2)

	x, y := fn(1, 2, 3, 4)
	assert.Equal(t, 24, x)
	assert.Equal(t, "product", y)
	_, _ = fn(1, 2, 3, 4)
	assert.Equal(t, 1, count)
}

type NonComparable struct {
	Field []int // slices are not comparable
}

func (n NonComparable) String() string {
	return fmt.Sprintf("NonComparable%v", n.Field)
}

func TestTableizeWithStringerFallback(t *testing.T) {
	count := 0
	fn := pure.TableizeI1O1(func(n NonComparable) int {
		count++
		return len(n.Field)
	}, 2)

	val := fn(NonComparable{Field: []int{1, 2, 3}})
	val2 := fn(NonComparable{Field: []int{1, 2, 3}})

	assert.Equal(t, 3, val)
	assert.Equal(t, 3, val2)
	assert.Equal(t, 1, count) // 캐시 확인
}

type TotallyInvalid struct {
	Field []int
}

func TestTableizeWithPanicIfNoComparableOrStringer(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic due to missing Stringer and non-comparable type")
		}
	}()
	fn := pure.TableizeI1O1(func(t TotallyInvalid) int {
		return len(t.Field)
	}, 2)

	_ = fn(TotallyInvalid{Field: []int{1}})
}
