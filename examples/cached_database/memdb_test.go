package main_test

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	cacheddatabase "github.com/on-the-ground/effect_ive_go/examples/cached_database"
	"github.com/stretchr/testify/assert"
)

type User struct {
	ID   string
	Name string
}

func (u *User) Equals(other any) bool {
	if ou, ok := other.(*User); ok {
		return u.ID == ou.ID && u.Name == ou.Name
	}
	return false
}

func schema() *memdb.DBSchema {
	return &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"user": {
				Name: "user",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "ID"},
					},
				},
			},
		},
	}
}

func TestMemDBStore_BasicOperations(t *testing.T) {
	store, err := cacheddatabase.NewMemDBStore[string]("user", "id", schema())
	assert.NoError(t, err)

	user1 := &User{ID: "u1", Name: "Alice"}
	user2 := &User{ID: "u1", Name: "Bob"}
	user3 := &User{ID: "u1", Name: "Alice"}

	// InsertIfAbsent (new)
	inserted, err := store.InsertIfAbsent("u1", user1)
	assert.NoError(t, err)
	assert.True(t, inserted)

	// InsertIfAbsent (existing)
	inserted, err = store.InsertIfAbsent("u1", user1)
	assert.NoError(t, err)
	assert.False(t, inserted)

	// Load (existing)
	val, ok, err := store.Load("u1")
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, user1, val)

	// CompareAndSwap (should succeed)
	swapped, err := store.CompareAndSwap("u1", user1, user2)
	assert.NoError(t, err)
	assert.True(t, swapped)

	// CompareAndSwap (should fail due to different value)
	swapped, err = store.CompareAndSwap("u1", user1, user3)
	assert.NoError(t, err)
	assert.False(t, swapped)

	// CompareAndDelete (should fail: wrong old value)
	deleted, err := store.CompareAndDelete("u1", user1)
	assert.NoError(t, err)
	assert.False(t, deleted)

	// CompareAndDelete (should succeed)
	deleted, err = store.CompareAndDelete("u1", user2)
	assert.NoError(t, err)
	assert.True(t, deleted)

	// Load (after deletion)
	_, ok, err = store.Load("u1")
	assert.NoError(t, err)
	assert.False(t, ok)
}
