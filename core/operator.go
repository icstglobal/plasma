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
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/icstglobal/plasma/core/types"
	log "github.com/sirupsen/logrus"
)

const (
	rate = 2
)

// Operator a key element in plasma
// 1. seal a block
// 2. commit a block hash
// 3.
type Operator struct {
	Addr common.Address
	// PrivateKey *ecdsa.PrivateKey

	chain  *BlockChain
	txPool *TxPool
	utxoRD UtxoReaderWriter

	quit chan struct{} // quit channel
}

// NewOperator creates a new operator
func NewOperator(chain *BlockChain, pool *TxPool, opAddr common.Address) *Operator {
	oper := &Operator{
		Addr: opAddr,
		// PrivateKey: privateKey,
		chain:  chain,
		txPool: pool,
		quit:   make(chan struct{}),
	}
	return oper
}

// SetOperbase changes operator's current address
func (o *Operator) SetOperbase(operbase common.Address) {
	o.Addr = operbase
}

// Start the operator to do mining
func (o *Operator) Start() {
	go o.start()
}

func (o *Operator) start() {
	ticker := time.NewTicker(time.Second * rate)
	defer ticker.Stop()
	for {
		select {
		case <-o.quit:
			return
		case <-ticker.C:
			if txs := o.txPool.Content(); len(txs) == 0 {
				continue
			}
			fmt.Printf("%v\n", "operator txs..")
			o.Seal()
		}
	}
}

// Stop sends a sigal to stop the operator
func (o *Operator) Stop() {
	close(o.quit)
}

// Seal get txs from txpool and construct block, then seal the block
func (o *Operator) Seal() error {
	block := o.constructBlock()
	newDbTx := o.chain.db.BeginTx()
	if !newDbTx {
		return errors.New("database race detected, there should be no tx pending when plasma operator commit a block")
	}
	// append block to chain and update chain head
	if err := o.chain.WriteBlock(block); err != nil {
		return err
	}
	o.chain.ReplaceHead(block)
	// remove used utxo
	for txIdx, tx := range block.Transactions() {
		for _, in := range tx.GetInsCopy() {
			if err := o.utxoRD.Del(in.ID()); err != nil {
				log.WithError(err).WithField("utxo", *in).Error("failed to delete utxo")
			}
		}
		for outIdx, out := range tx.GetOutsCopy() {
			utxo := types.UTXO{
				UTXOID: types.UTXOID{
					BlockNum: block.NumberU64(), TxIndex: uint32(txIdx), OutIndex: byte(outIdx),
				},
				Amount: out.Amount,
				Owner:  out.Owner,
			}
			if err := o.utxoRD.Put(&utxo); err != nil {
				log.WithError(err).WithField("utxo", utxo).Error("failed to write utxo")
			}
		}

	}
	if err := o.chain.db.CommitTx(); err != nil {
		log.WithError(err).Error("failed to commit db tx")
		//TODO: need recover here
		return err
	}

	for _, tx := range block.Transactions() {
		hash := tx.Hash()
		o.txPool.removeTx(hash, true)
	}

	return nil
}

func (o *Operator) constructBlock() *types.Block {
	// header
	parent := o.chain.CurrentBlock()
	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Coinbase:   o.Addr,
		Number:     num.Add(num, common.Big1),
		Time:       big.NewInt(time.Now().Unix()),
	}
	// txs
	txs := o.txPool.Content()

	return types.NewBlock(header, txs)
}

// SubmitBlock write block hash to root chain
func (o *Operator) SubmitBlock() error {
	// todo
	return nil
}
