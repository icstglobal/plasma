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
	"crypto/ecdsa"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/icstglobal/plasma/core/types"
	log "github.com/sirupsen/logrus"
)

const (
	rate                = 2
	submitBlockInterval = 3
)

// Operator a key element in plasma
// 1. seal a block
// 2. commit a block hash
type Operator struct {
	Addr       common.Address
	privateKey *ecdsa.PrivateKey

	chain     *BlockChain
	txPool    *TxPool
	rootchain *RootChain
	utxoRD    UtxoReaderWriter

	TxsCh chan types.Transactions
	quit  chan struct{} // quit channel
}

// NewOperator creates a new operator
func NewOperator(chain *BlockChain, pool *TxPool, privateKey *ecdsa.PrivateKey, utxoRD UtxoReaderWriter, rootchain *RootChain) *Operator {
	from := rootchain.PubKeyToAddress(privateKey)
	oper := &Operator{
		Addr:       common.BytesToAddress(from),
		privateKey: privateKey,
		chain:      chain,
		rootchain:  rootchain,
		txPool:     pool,
		TxsCh:      make(chan types.Transactions, 10),
		quit:       make(chan struct{}),
		utxoRD:     utxoRD,
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
	go o.processTxs()
}

func (o *Operator) processTxs() {
	for txs := range o.TxsCh {
		if txs[0].IsDepositTx() {
			err := o.SealDeposit(txs)
			if err != nil {
				log.Error("operator seal deposit block error:", err.Error())
			}
		} else {
			log.Debug("processTxs Seal", txs)
			err := o.Seal(txs)
			if err != nil {
				log.Error("operator seal block error:", err.Error())
			}
		}
	}

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
			} else {
				log.Debugf("%v\n", "operator txs..")
				o.TxsCh <- txs
			}
		}
	}
}

func (o *Operator) AddTxs(txs types.Transactions) {
	if len(txs) == 0 {
		return
	}
	o.TxsCh <- txs
}

// Stop sends a sigal to stop the operator
func (o *Operator) Stop() {
	close(o.quit)
}

// Seal get txs from txpool and construct block, then seal the block
func (o *Operator) Seal(txs types.Transactions) error {
	block := o.constructBlock(txs)
	newDbTx := o.chain.db.BeginTx()
	if !newDbTx {
		return errors.New("database race detected, there should be no tx pending when plasma operator commit a block")
	}
	// append block to chain and update chain head
	if err := o.chain.WriteBlock(block); err != nil {
		_err := o.chain.db.RollbackTx()
		if _err != nil {
			log.Error("db.RollbackTx Error:", _err.Error())
		}

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
				TxOut: types.TxOut{
					Amount: out.Amount,
					Owner:  out.Owner,
				},
			}
			if err := o.utxoRD.Put(&utxo); err != nil {
				log.WithError(err).WithField("utxo", utxo).Error("failed to write utxo")
				_err := o.chain.db.RollbackTx()
				if _err != nil {
					log.Error("db.RollbackTx Error:", _err.Error())
				}
				return err
			}
		}

	}
	if err := o.chain.db.CommitTx(); err != nil {
		log.WithError(err).Error("failed to commit db tx")
		//TODO: need recover here
		_err := o.chain.db.RollbackTx()
		if _err != nil {
			log.Error("db.RollbackTx Error:", _err.Error())
		}
		return err
	}

	for _, tx := range block.Transactions() {
		hash := tx.Hash()
		o.txPool.removeTx(hash, true)
	}
	// sumbit block every n blocks
	log.Debug("block.NumberU64()%submitBlockInterval:", block.NumberU64()%submitBlockInterval)
	if block.NumberU64()%submitBlockInterval == 0 {
		err := o.SubmitBlock(block)
		if err != nil {
			return err
		}
	}

	return nil
}

// SealDeposit get txs from txpool and construct block, then seal the block
func (o *Operator) SealDeposit(txs types.Transactions) error {
	block := o.constructBlock(txs)
	newDbTx := o.chain.db.BeginTx()
	if !newDbTx {
		return errors.New("database race detected, there should be no tx pending when plasma operator commit a block")
	}
	// append block to chain and update chain head
	if err := o.chain.WriteDepositBlock(block); err != nil {
		_err := o.chain.db.RollbackTx()
		if _err != nil {
			log.Error("db.RollbackTx Error:", _err.Error())
		}

		return err
	}
	o.chain.ReplaceHead(block)
	// add deposit utxo to set
	for txIdx, tx := range block.Transactions() {
		for outIdx, out := range tx.GetOutsCopy() {
			utxo := types.UTXO{
				UTXOID: types.UTXOID{
					BlockNum: block.NumberU64(), TxIndex: uint32(txIdx), OutIndex: byte(outIdx),
				},
				TxOut: types.TxOut{
					Amount: out.Amount,
					Owner:  out.Owner,
				},
			}
			if err := o.utxoRD.Put(&utxo); err != nil {
				log.WithError(err).WithField("utxo", utxo).Error("failed to write utxo")
				_err := o.chain.db.RollbackTx()
				if _err != nil {
					log.Error("db.RollbackTx Error:", _err.Error())
				}
				return err
			}
		}

	}
	if err := o.chain.db.CommitTx(); err != nil {
		log.WithError(err).Error("failed to commit db tx")
		_err := o.chain.db.RollbackTx()
		if _err != nil {
			log.Error("db.RollbackTx Error:", _err.Error())
		}
		return err
	}
	return nil
}

func (o *Operator) constructBlock(txs types.Transactions) *types.Block {
	// header
	parent := o.chain.CurrentBlock()
	num := parent.Number()
	log.Debug("parent.Number:", num)
	header := &types.Header{
		ParentHash: parent.Hash(),
		Coinbase:   o.Addr,
		Number:     num.Add(num, common.Big1),
		Time:       big.NewInt(time.Now().Unix()),
	}

	return types.NewBlock(header, txs)
}

// SubmitBlock write block hash to root chain
func (o *Operator) SubmitBlock(block *types.Block) error {
	return o.rootchain.SubmitBlock(block, o.privateKey)
}
