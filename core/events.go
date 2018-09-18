package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/plasma/core/types"
)

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*types.Transaction }

// NewMinedBlockEvent is posted when a block has been imported.
type NewMinedBlockEvent struct{ Block *types.Block }

type ChainEvent struct {
	Block *types.Block
	Hash  common.Hash
}

type ChainSideEvent struct {
	Block *types.Block
}

type ChainHeadEvent struct{ Block *types.Block }
