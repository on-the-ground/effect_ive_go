package effectmodel

type Partitionable interface {
	PartitionKey() string
}
