package store

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	writePauseWarningThrottler = 1 * time.Minute
)

var OpenFileLimit = 64
var defaultBatch = &leveldb.Batch{}

type LDBDatabase struct {
	fn string      // filename for reporting
	db *leveldb.DB // LevelDB instance
	// tx    *leveldb.Transaction // only one per connection
	batch *leveldb.Batch // use batch to simulate db tx

	quitLock sync.Mutex      // Mutex protecting the quit channel access
	quitChan chan chan error // Quit channel to stop the metrics collection before closing the database
}

// NewLDBDatabase returns a LevelDB wrapped object.
func NewLDBDatabase(file string, cache int, handles int) (*LDBDatabase, error) {
	// Ensure we have some minimal caching and file guarantees
	if cache < 16 {
		cache = 16
	}
	if handles < 16 {
		handles = 16
	}
	log.Info("Allocated cache and file handles", "cache", cache, "handles", handles)

	// Open the db and recover any potential corruptions
	db, err := leveldb.OpenFile(file, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}
	// (Re)check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}
	return &LDBDatabase{
		fn: file,
		db: db,
	}, nil
}

// Path returns the path to the database directory.
func (db *LDBDatabase) Path() string {
	return db.fn
}

// Put puts the given key / value to the queue
func (db *LDBDatabase) Put(key []byte, value []byte) error {
	if db.batch != nil {
		db.batch.Put(key, value)
		return nil
	}
	return db.db.Put(key, value, nil)
}

func (db *LDBDatabase) Has(key []byte) (bool, error) {
	return db.db.Has(key, nil)
}

// Get returns the given key if it's present.
func (db *LDBDatabase) Get(key []byte) ([]byte, error) {
	dat, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

// Delete deletes the key from the queue and database
func (db *LDBDatabase) Delete(key []byte) error {
	if db.batch != nil {
		db.batch.Delete(key)
		return nil
	}
	return db.db.Delete(key, nil)
}

func (db *LDBDatabase) Close() {
	// Stop the metrics collection to avoid internal database races
	db.quitLock.Lock()
	defer db.quitLock.Unlock()

	if db.quitChan != nil {
		errc := make(chan error)
		db.quitChan <- errc
		if err := <-errc; err != nil {
			log.Error("Metrics collection failed", "err", err)
		}
		db.quitChan = nil
	}
	// rollback uncommitted tx
	db.RollbackTx()
	err := db.db.Close()
	if err == nil {
		log.Info("Database closed")
	} else {
		log.Error("Failed to close database", "err", err)
	}
}

func (db *LDBDatabase) LDB() *leveldb.DB {
	return db.db
}

// BeginTx starts a db tx, there can be only one tx per connection.
// After a tx begins, all later operation will be managed by the tx.
//
// Returns whether a new tx created, "false" if old tx being used.
//
// not thread-safe.
func (db *LDBDatabase) BeginTx() bool {
	if db.batch != nil {
		return false
	}

	db.batch = defaultBatch
	return true

}

func (db *LDBDatabase) CommitTx() error {
	if db.batch == nil {
		return errors.New("there is no open db tx")
	}
	return db.db.Write(db.batch, nil)
}

func (db *LDBDatabase) RollbackTx() error {
	if db.batch == nil {
		return errors.New("there is no open db tx")
	}

	db.batch.Reset()
	db.batch = nil

	return nil
}
