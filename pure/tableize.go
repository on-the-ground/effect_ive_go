package pure

import (
	"fmt"
)

type ComparableOrStringer any
type ComparableOrString any

func TableizeI1O1[I1 ComparableOrStringer, O1 any](
	pureFn func(I1) O1,
	maxTableSize uint32,
) func(I1) O1 {
	tableized := tableize(
		func(args ...ComparableOrStringer) O1 {
			return pureFn(args[0].(I1))
		},
		maxTableSize,
	)
	return func(i1 I1) O1 {
		return tableized(i1)
	}
}

func TableizeI2O1[I1, I2 ComparableOrStringer, O1 any](
	pureFn func(I1, I2) O1,
	maxTableSize uint32,
) func(I1, I2) O1 {
	tableized := tableize(
		func(args ...ComparableOrStringer) O1 {
			return pureFn(args[0].(I1), args[1].(I2))
		},
		maxTableSize,
	)
	return func(i1 I1, i2 I2) O1 {
		return tableized(i1, i2)
	}
}

func TableizeI3O1[I1, I2, I3 ComparableOrStringer, O1 any](
	pureFn func(I1, I2, I3) O1,
	maxTableSize uint32,
) func(I1, I2, I3) O1 {
	tableized := tableize(
		func(args ...ComparableOrStringer) O1 {
			return pureFn(args[0].(I1), args[1].(I2), args[2].(I3))
		},
		maxTableSize,
	)
	return func(i1 I1, i2 I2, i3 I3) O1 {
		return tableized(i1, i2, i3)
	}
}

func TableizeI4O1[I1, I2, I3, I4 ComparableOrStringer, O1 any](
	pureFn func(I1, I2, I3, I4) O1,
	maxTableSize uint32,
) func(I1, I2, I3, I4) O1 {
	tableized := tableize(
		func(args ...ComparableOrStringer) O1 {
			return pureFn(args[0].(I1), args[1].(I2), args[2].(I3), args[3].(I4))
		},
		maxTableSize,
	)
	return func(i1 I1, i2 I2, i3 I3, i4 I4) O1 {
		return tableized(i1, i2, i3, i4)
	}
}

func tableKey(i ComparableOrStringer) ComparableOrString {
	if stringer, ok := i.(fmt.Stringer); ok {
		return stringer.String()
	}
	return i
}

func tableize[O any](
	pureFn func(...ComparableOrStringer) O,
	maxTableSize uint32,
) func(...ComparableOrStringer) O {
	memo := NewTrie[O](maxTableSize)
	return func(args ...ComparableOrStringer) O {
		keys := make([]ComparableOrString, len(args))
		for i, arg := range args {
			keys[i] = tableKey(arg)
		}
		v, ok := memo.Load(keys)
		if !ok {
			v = pureFn(args...)
			memo.Store(keys, v)
		}
		return v
	}
}
