package store

// Putter wraps the database write operation supported by both batches and regular databases.
type Putter interface {
	Put(key []byte, value []byte) error
}

// Deleter wraps the database delete operation supported by both batches and regular databases.
type Deleter interface {
	Delete(key []byte) error
}

// Database wraps all database operations. All methods are safe for concurrent use.
type Database interface {
	Putter
	Deleter
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Close()
	// BeginTx starts a db tx, there can be only one tx per connection.
	// After a tx begins, all later operation will be managed by the tx.
	//
	//  Returns false if old tx being used.
	//
	// not thread-safe.
	BeginTx() bool
	RollbackTx() error
	CommitTx() error
}
