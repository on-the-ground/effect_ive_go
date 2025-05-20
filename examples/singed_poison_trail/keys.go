package main

const environmentKey = "/env"

func keyOfCoordinates(x int32) string {
	return environmentKey + "/coordinates/" + string(x)
}

func keyOfRebalanceHp(x int32) string {
	return keyOfCoordinates(x) + "/ops/hp/rebalance"
}
