// Package storage provides a bbolt-backed key-value store used by metaviz
// to persist all runtime configuration (except .mrs files and uploaded
// mihomo configs which remain as plain files).
//
// Two bbolt buckets:
//
//	settings  — key → JSON blob (singleton structs: meta_settings, proxy_settings, etc.)
//	entities  — composite key "<kind>/<id>" → JSON blob (nodes, subscriptions, caches)
package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketSettings = []byte("settings")
	bucketEntities = []byte("entities")
)

// DB is the application-wide bbolt database handle.
type DB struct {
	db *bolt.DB
}

// Entity is a row from the entities bucket.
type Entity struct {
	Kind      string
	ID        string
	Data      string
	UpdatedAt time.Time
}

// Open opens (or creates) the bbolt database at path and initialises buckets.
func Open(path string) (*DB, error) {
	bdb, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt %s: %w", path, err)
	}
	err = bdb.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketSettings, bucketEntities} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		bdb.Close()
		return nil, fmt.Errorf("init buckets: %w", err)
	}
	return &DB{db: bdb}, nil
}

// Close closes the underlying database.
func (db *DB) Close() error { return db.db.Close() }

// ── Settings ────────────────────────────────────────────────────────────────

// LoadSetting reads a JSON-encoded value from the settings bucket.
// If the key does not exist v is left unchanged and nil is returned.
func (db *DB) LoadSetting(key string, v interface{}) error {
	return db.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		raw := b.Get([]byte(key))
		if raw == nil {
			return nil // not set yet
		}
		return json.Unmarshal(raw, v)
	})
}

// SaveSetting JSON-encodes v and upserts it into the settings bucket.
func (db *DB) SaveSetting(key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal setting %q: %w", key, err)
	}
	return db.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSettings).Put([]byte(key), data)
	})
}

// ── Entities ────────────────────────────────────────────────────────────────

func entityKey(kind, id string) []byte { return []byte(kind + "/" + id) }

// GetEntity fetches a single entity by (kind, id).
// Returns (nil, nil) if not found.
func (db *DB) GetEntity(kind, id string) (*Entity, error) {
	var e *Entity
	err := db.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketEntities).Get(entityKey(kind, id))
		if raw == nil {
			return nil
		}
		var row entityRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return err
		}
		e = &Entity{Kind: kind, ID: id, Data: row.Data, UpdatedAt: row.UpdatedAt}
		return nil
	})
	return e, err
}

// ListEntities returns all entities of a given kind, ordered by UpdatedAt ASC.
func (db *DB) ListEntities(kind string) ([]Entity, error) {
	prefix := []byte(kind + "/")
	var out []Entity
	err := db.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketEntities).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var row entityRow
			if err := json.Unmarshal(v, &row); err != nil {
				continue
			}
			id := strings.TrimPrefix(string(k), string(prefix))
			out = append(out, Entity{Kind: kind, ID: id, Data: row.Data, UpdatedAt: row.UpdatedAt})
		}
		return nil
	})
	// Sort by UpdatedAt ASC (bbolt cursor is lexicographic, not time-ordered)
	sortEntities(out)
	return out, err
}

// UpsertEntity inserts or replaces an entity row. v is JSON-marshalled into data.
func (db *DB) UpsertEntity(kind, id string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal entity %s/%s: %w", kind, id, err)
	}
	row := entityRow{Data: string(data), UpdatedAt: time.Now()}
	encoded, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return db.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketEntities).Put(entityKey(kind, id), encoded)
	})
}

// DeleteEntity removes a single entity. Returns nil if not found.
func (db *DB) DeleteEntity(kind, id string) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketEntities).Delete(entityKey(kind, id))
	})
}

// DeleteEntitiesByKind removes all entities of a given kind.
func (db *DB) DeleteEntitiesByKind(kind string) error {
	prefix := []byte(kind + "/")
	return db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketEntities)
		c := b.Cursor()
		var keys [][]byte
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
			cp := make([]byte, len(k))
			copy(cp, k)
			keys = append(keys, cp)
		}
		for _, k := range keys {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// ── Store (backward-compat shim) ────────────────────────────────────────────

// Store wraps DB for a single named settings key.
type Store struct {
	db  *DB
	key string
}

// NewStore returns a Store that persists to the given settings key.
func NewStore(db *DB, key string) *Store { return &Store{db: db, key: key} }

func (s *Store) Load(v interface{}) error { return s.db.LoadSetting(s.key, v) }
func (s *Store) Save(v interface{}) error { return s.db.SaveSetting(s.key, v) }

// ── Internal helpers ─────────────────────────────────────────────────────────

type entityRow struct {
	Data      string    `json:"d"`
	UpdatedAt time.Time `json:"t"`
}

func sortEntities(es []Entity) {
	// Simple insertion sort — entity lists are typically small (<1000)
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && es[j].UpdatedAt.Before(es[j-1].UpdatedAt); j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
}
