package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/plasma/core/types"
)

// BlockValidator is an interface which defines the standard for block validation. It
// is only responsible for validating block contents, as the header validation is
// done by the specific consensus engines.
//
type BlockValidator interface {
	// ValidateBody validates the given block's content.
	ValidateBody(block *types.Block) error
}

// TxValidator is an interface which defines the standard for transaction validation
type TxValidator interface {
	Validate(tx *types.Transaction) error
}

// Processor is an interface for processing blocks using a given initial state.
//
// Process takes the block to be processed and the statedb upon which the
// initial state is based. It should return the receipts generated, amount
// of gas used in the process and return an error if any of the internal rules
// failed.
type Processor interface {
	Process(block *types.Block) (uint64, error)
}

// BlockReader is an interface for block reading
type BlockReader interface {
	HasBlock(hash common.Hash, blockNum uint64) bool
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	GetBlockByNumber(number uint64) *types.Block
}

// UtxoReaderWriter combines both UtxoReader and UtxoDeleter interfaces
type UtxoReaderWriter interface {
	UtxoReader
	UtxoWriter
}

// UtxoReader defines an interface to read utxo set data
type UtxoReader interface {
	Get(id types.UTXOID) *types.UTXO
}

// UtxoWriter defines an interface to delete utxo
type UtxoWriter interface {
	Del(id types.UTXOID) error
	Put(v *types.UTXO) error
}
