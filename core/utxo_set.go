package core

import (
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/icstglobal/plasma/core/rawdb"
	"github.com/icstglobal/plasma/core/types"
)

type UTXOSet struct {
	db ethdb.Database
}

func NewUTXOSet(db ethdb.Database) *UTXOSet {
	return &UTXOSet{db: db}
}

func (us *UTXOSet) Get(id types.UTXOID) *types.UTXO {
	return rawdb.ReadUTXO(us.db, id.BlockNum, id.TxIndex, id.OutIndex)
}

func (us *UTXOSet) Del(id types.UTXOID) error {
	return rawdb.DeleteUTXO(us.db, id.BlockNum, id.TxIndex, id.OutIndex)
}

func (us *UTXOSet) Write(v *types.UTXO) error {
	return rawdb.WriteUTXO(us.db, v)
}
