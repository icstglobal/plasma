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

func (self *Operator) SetOperbase(operbase common.Address) {
	self.Addr = operbase
}

func (self *Operator) Start() {
	go self.start()
}

func (self *Operator) start() {
	ticker := time.NewTicker(time.Second * rate)
	defer ticker.Stop()
	for {
		select {
		case <-self.quit:
			return
		case <-ticker.C:
			if txs := self.txPool.Content(); len(txs) == 0 {
				continue
			}
			fmt.Printf("%v\n", "operator txs..")
			self.Seal()
		}
	}
}

func (self *Operator) Stop() {
	close(self.quit)
}

// Seal get txs from txpool and construct block, then seal the block
func (self *Operator) Seal() error {
	block := self.constructBlock()
	if err := self.chain.WriteBlock(block); err != nil {
		return err
	}
	self.chain.ReplaceHead(block)
	for txIdx, v := range block.Transactions() {
		for _, in := range v.GetInsCopy() {
			if err := self.utxoRD.Del(in.ID()); err != nil {
				// TODO:need recover here
				// log error and stop processing
				log.WithField("utxo", *in).Fatal("failed to delete utxo")
			}
		}
		for outIdx, out := range v.GetOutsCopy() {
			utxo := types.UTXO{
				UTXOID: types.UTXOID{
					BlockNum: block.NumberU64(), TxIndex: uint32(txIdx), OutIndex: byte(outIdx),
				},
				Amount: out.Amount,
				Owner:  out.Owner,
			}
			if err := self.utxoRD.Put(&utxo); err != nil {
				//TODO: need recover here
				// log err and stop processing
				log.WithField("utxo", utxo).Fatal("failed to write utxo")
			}
		}
		hash := v.Hash()
		self.txPool.removeTx(hash, true)
	}

	return nil
}

func (self *Operator) constructBlock() *types.Block {
	// header
	tstart := time.Now()
	parent := self.chain.CurrentBlock()

	tstamp := tstart.Unix()
	if parent.Time().Cmp(new(big.Int).SetInt64(tstamp)) >= 0 {
		tstamp = parent.Time().Int64() + 1
	}
	// this will ensure we're not going off too far in the future
	if now := time.Now().Unix(); tstamp > now+1 {
		wait := time.Duration(tstamp-now) * time.Second
		log.Info("Mining too far in the future", "wait", common.PrettyDuration(wait))
		time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Coinbase:   self.Addr,
		Number:     num.Add(num, common.Big1),
		Time:       big.NewInt(tstamp),
	}
	// txs
	txs := self.txPool.Content()

	return types.NewBlock(header, txs)
}

// SubmitBlock write block hash to root chain
func (self *Operator) SubmitBlock() error {
	// todo
	return nil
}
