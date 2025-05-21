package main

import "strings"

const root = ""

const environmentKey = "env"

func keyOfCoordinates(coord *Coordinate) string {
	return strings.Join([]string{
		root,
		environmentKey,
		"coordinates",
		coord.String(),
	}, "/")
}

func keyOfAdjustHp(coord *Coordinate) string {
	return strings.Join([]string{
		keyOfCoordinates(coord),
		"ops",
		"hp",
		"adjust",
	}, "/")
}
