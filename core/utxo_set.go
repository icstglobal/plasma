package core

import (
	"github.com/icstglobal/plasma/core/rawdb"
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/store"
)

// UTXOSet is a utxo manager, which implements interface UtxoReaderWriter
type UTXOSet struct {
	db store.Database
}

//NewUTXOSet  connects utxoset to db
func NewUTXOSet(db store.Database) *UTXOSet {
	return &UTXOSet{db: db}
}

//Get finds a utxo by id
//nil, if not found
func (us *UTXOSet) Get(id types.UTXOID) *types.UTXO {
	return rawdb.ReadUTXO(us.db, id.BlockNum, id.TxIndex, id.OutIndex)
}

//Del remove a utxo from db
func (us *UTXOSet) Del(id types.UTXOID) error {
	return rawdb.DeleteUTXO(us.db, id.BlockNum, id.TxIndex, id.OutIndex)
}

//Put saves a utxo into db
func (us *UTXOSet) Put(v *types.UTXO) error {
	return rawdb.WriteUTXO(us.db, v)
}
