package purefn

func TableizeI1O2[I1 ComparableOrStringer, O1, O2 any](
	pureFn func(I1) (O1, O2),
	maxTableSize uint32,
) func(I1) (O1, O2) {
	tableized := tableize_dual_output(
		func(args ...ComparableOrStringer) (O1, O2) {
			return pureFn(args[0].(I1))
		},
		maxTableSize,
	)
	return func(i1 I1) (O1, O2) {
		return tableized(i1)
	}
}

func TableizeI2O2[I1, I2 ComparableOrStringer, O1, O2 any](
	pureFn func(I1, I2) (O1, O2),
	maxTableSize uint32,
) func(I1, I2) (O1, O2) {
	tableized := tableize_dual_output(
		func(args ...ComparableOrStringer) (O1, O2) {
			return pureFn(args[0].(I1), args[1].(I2))
		},
		maxTableSize,
	)
	return func(i1 I1, i2 I2) (O1, O2) {
		return tableized(i1, i2)
	}
}

func TableizeI3O2[I1, I2, I3 ComparableOrStringer, O1, O2 any](
	pureFn func(I1, I2, I3) (O1, O2),
	maxTableSize uint32,
) func(I1, I2, I3) (O1, O2) {
	tableized := tableize_dual_output(
		func(args ...ComparableOrStringer) (O1, O2) {
			return pureFn(args[0].(I1), args[1].(I2), args[2].(I3))
		},
		maxTableSize,
	)
	return func(i1 I1, i2 I2, i3 I3) (O1, O2) {
		return tableized(i1, i2, i3)
	}
}

func TableizeI4O2[I1, I2, I3, I4 ComparableOrStringer, O1, O2 any](
	pureFn func(I1, I2, I3, I4) (O1, O2),
	maxTableSize uint32,
) func(I1, I2, I3, I4) (O1, O2) {
	tableized := tableize_dual_output(
		func(args ...ComparableOrStringer) (O1, O2) {
			return pureFn(args[0].(I1), args[1].(I2), args[2].(I3), args[3].(I4))
		},
		maxTableSize,
	)
	return func(i1 I1, i2 I2, i3 I3, i4 I4) (O1, O2) {
		return tableized(i1, i2, i3, i4)
	}
}

type result[O1 any, O2 any] struct {
	O1 O1
	O2 O2
}

func tableize_dual_output[O1, O2 any](
	pureFn func(...ComparableOrStringer) (O1, O2),
	maxTableSize uint32,
) func(...ComparableOrStringer) (O1, O2) {
	memo := NewTrie[result[O1, O2]](maxTableSize)
	return func(args ...ComparableOrStringer) (O1, O2) {
		keys := make([]ComparableOrString, len(args))
		for i, arg := range args {
			keys[i] = tableKey(arg)
		}
		res, ok := memo.Load(keys)
		if !ok {
			v1, v2 := pureFn(args...)
			res = result[O1, O2]{O1: v1, O2: v2}
			memo.Store(keys, res)
		}
		return res.O1, res.O2
	}
}
