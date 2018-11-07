// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	// "crypto/ecdsa"
	// "github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/plasma/core/types"
	log "github.com/sirupsen/logrus"
)

// Operator a key element in plasma
// 1. seal a block
// 2. commit a block hash
type Validator struct {
	chain     *BlockChain
	rootchain *RootChain
	utxoRD    UtxoReaderWriter

	NewTxsCh chan types.Transactions
	quit     chan struct{} // quit channel
	// the block number of non-deposit block on plasma, increased by "childBlockInterval"
	currentChildBlock uint64
	newBlockCh        chan *types.Block
}

// NewOperator creates a new operator
func NewValidator(chain *BlockChain, utxoRD UtxoReaderWriter, rootchain *RootChain) *Validator {
	v := &Validator{
		chain:     chain,
		rootchain: rootchain,
		NewTxsCh:  make(chan types.Transactions, 10),
		quit:      make(chan struct{}),
		utxoRD:    utxoRD,
	}
	currentBlockNum := chain.CurrentHeader().Number.Uint64()
	// head bock is non-deposit block
	if currentBlockNum%childBlockInterval == 0 {
		v.currentChildBlock = currentBlockNum
	} else {
		// head block is deposit block
		v.currentChildBlock = ((currentBlockNum / childBlockInterval) + 1) * childBlockInterval
	}
	return v
}

// Start the operator to do mining
func (v *Validator) Start() {

}

func (v *Validator) ProcessRemoteTxs(txs types.Transactions) {
	v.NewTxsCh <- txs
}

// ProcessRemoteBlock write block to chain
// cache the block if block is not sequential
func (v *Validator) ProcessRemoteBlock(block *types.Block) {
	newDbTx := v.chain.db.BeginTx()
	if !newDbTx {
		log.Error("database race detected, there should be no tx pending when plasma operator commit a block")
		return
	}
	if err := v.chain.WriteBlock(block); err != nil {
		_err := v.chain.db.RollbackTx()
		if _err != nil {
			log.Error("db.RollbackTx Error:", _err.Error())
		}
		return
	}
	if v.newBlockCh != nil {
		v.newBlockCh <- block
	}
}

func (v *Validator) SubscribeNewBlockCh(ch chan *types.Block) {
	v.newBlockCh = ch
}
