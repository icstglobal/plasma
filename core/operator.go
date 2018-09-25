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
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/icstglobal/plasma/core/types"
)

const (
	rate = 2
)

type Plasma interface {
	BlockChain() *BlockChain
	TxPool() *TxPool
	ChainDb() ethdb.Database
	Operbase() common.Address
}

// Operator a key element in plasma
// 1. seal a block
// 2. commit a block hash
// 3.
type Operator struct {
	Addr       common.Address
	PrivateKey *ecdsa.PrivateKey
	quit       chan struct{} // quit channel
	plasma     Plasma
}

// NewOperator creates a new operator
func NewOperator(plasma Plasma, privateKey *ecdsa.PrivateKey) *Operator {
	oper := &Operator{
		plasma:     plasma,
		Addr:       plasma.Operbase(),
		PrivateKey: privateKey,
		quit:       make(chan struct{}),
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
			if txs := self.plasma.TxPool().Content(); len(txs) == 0 {
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
	self.plasma.BlockChain().WriteBlock(block)
	self.plasma.BlockChain().ReplaceHead(block)
	for _, v := range block.Transactions() {
		fmt.Printf("%v\n", v)
		hash := v.Hash()
		self.plasma.TxPool().removeTx(hash, true)
	}

	return nil
}

func (self *Operator) constructBlock() *types.Block {
	// header
	tstart := time.Now()
	parent := self.plasma.BlockChain().CurrentBlock()

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
	txs := self.plasma.TxPool().Content()

	return types.NewBlock(header, txs)
}

// SubmitBlock write block hash to root chain
func (self *Operator) SubmitBlock() error {
	// todo
	return nil
}
