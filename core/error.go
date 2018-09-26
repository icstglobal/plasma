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

import "errors"

var (
	// ErrKnownBlock is returned when a block to import is already known locally.
	ErrKnownBlock = errors.New("block already known")

	// ErrBlacklistedHash is returned if a block to import is on the blacklist.
	ErrBlacklistedHash = errors.New("blacklisted hash")

	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = errors.New("nonce too high")

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

	// ErrorAlreadySpent returns if the utxo as tx in has been spent already
	ErrorAlreadySpent = errors.New("utxo already spent")
	// ErrNotEnoughTxFee is returned if the tx fee is not bigger than zero
	ErrNotEnoughTxFee = errors.New("not enough tx fee")

	// ErrDuplicateSpent returns if the duplicate spent is detected
	ErrDuplicateSpent = errors.New("duplicate spent")
)
