package main

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/on-the-ground/effect_ive_go/effects/state"
)

type MemDBStore[K comparable] struct {
	db    *memdb.MemDB
	table string
	index string // e.g. "id"
}

func NewMemDBStore[K comparable](
	table string,
	index string,
	schema *memdb.DBSchema,
) (state.CasStore[K], error) {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	return MemDBStore[K]{db: db, table: table, index: index}, nil
}

func (m MemDBStore[K]) Load(key K) (value any, ok bool, err error) {
	txn := m.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(m.table, m.index, key)
	if err != nil || raw == nil {
		return nil, false, err
	}
	return raw, true, nil
}

func (m MemDBStore[K]) InsertIfAbsent(key K, value any) (inserted bool, err error) {
	txn := m.db.Txn(true)
	defer txn.Abort()

	old, err := txn.First(m.table, m.index, key)
	if err != nil {
		return false, err
	} else if old != nil {
		return false, nil
	}

	if err := txn.Insert(m.table, value); err != nil {
		return false, err
	}
	txn.Commit()
	return true, nil
}

func (m MemDBStore[K]) CompareAndSwap(key K, old, new any) (swapped bool, err error) {
	txn := m.db.Txn(true)
	defer txn.Abort()

	actual, err := txn.First(m.table, m.index, key)
	if err != nil {
		return false, err
	} else if actual == nil || !state.Equals(old, actual) {
		return false, nil
	}

	if err := txn.Insert(m.table, new); err != nil {
		return false, err
	}
	txn.Commit()
	return true, nil
}

func (m MemDBStore[K]) CompareAndDelete(key K, old any) (deleted bool, err error) {
	txn := m.db.Txn(true)
	defer txn.Abort()

	actual, err := txn.First(m.table, m.index, key)
	if err != nil {
		return false, err
	} else if actual == nil || !state.Equals(old, actual) {
		return false, nil
	}

	if err := txn.Delete(m.table, actual); err != nil {
		return false, err
	}
	txn.Commit()
	return true, nil
}
