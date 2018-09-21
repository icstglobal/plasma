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
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math"
	"math/big"
	"reflect"
	"sync"
	"time"

	"github.com/icstglobal/plasma/core/types"
)

var (
	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	// ErrTxInNotFound is returned if the tx in is not found by in index
	ErrTxInNotFound = errors.New("tx in not found")

	// ErrTxOutNotFound is returned if the tx out is not found by out index
	ErrTxOutNotFound = errors.New("tx out not found")

	// ErrTxOutNotOwned is returned if the tx out to be used is not owned by the tx sender
	ErrTxOutNotOwned = errors.New("tx out not owned")

	// ErrTxTotalAmountNotEqual is returned if the total amount of tx in and out not equal
	ErrTxTotalAmountNotEqual = errors.New("total amount of tx ins and outs not equal")

	// ErrNotEnoughTxFee is returned if the tx fee is not bigger than zero
	ErrNotEnoughTxFee = errors.New("not enough tx fee")
)

const (
	rate = 2
)

type Plasma interface {
	AccountManager() *accounts.Manager
	BlockChain() *BlockChain
	TxPool() *TxPool
	ChainDb() ethdb.Database
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
		plasma:     Plasma,
		Addr:       addr,
		PrivateKey: privateKey,
		quit:       make(chan struct{}),
	}
	return oper
}

func (self *Operator) Start() {
	go self.start()
}

func (self *Operator) start() {
	ticker := time.NewTicker(time.Second * rate)
	defer ticker.Stop()
	for {
		select {
		case <-quit:
			return
		case t := <-ticker.C:
			if txs := self.plasma.TxPool().Content(); len(txs) == 0 {
				continue
			}
			self.Seal()
		}
	}
}

func (self *Operator) Stop() {
	close(quit)
}

// Seal get txs from txpool and construct block, then seal the block
func (self *Operator) Seal() error {
	block := constructBlock()
	self.plasma.BlockChain().WriteBlock(block)
	self.plasma.BlockChain().ReplaceHead(block)
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

}
