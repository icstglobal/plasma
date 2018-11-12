package types

import (
	"math/big"
)

// UTXOID is the identity of a UTXO obj
type UTXOID struct {
	BlockNum uint64 `json:"blockNum"`
	TxIndex  uint32 `json:"txIndex"`
	OutIndex byte   `json:"outIndex"`
}

// Equals check if two UTXOID objs are identical
func (id UTXOID) Equals(other UTXOID) bool {
	return id.BlockNum == other.BlockNum && id.TxIndex == other.TxIndex && id.OutIndex == other.OutIndex
}

// UTXO is the unspent tx output
type UTXO struct {
	UTXOID
	TxOut
}

// ID returns the identity of a UTXO obj
func (u UTXO) ID() UTXOID {
	return u.UTXOID
}

// Equals check if two UTXO objs are identical
func (u UTXO) Equals(other UTXO) bool {
	return u.UTXOID.Equals(other.UTXOID)
}

// NewNullUTXO
func NewNullUTXO() *UTXO {
	return &UTXO{UTXOID: UTXOID{BlockNum: 0}, TxOut: TxOut{Amount: big.NewInt(0)}}
}

// NewNullTxOut
func NewNullTxOut() *TxOut {
	return &TxOut{Amount: big.NewInt(0)}
}

func IsNullUTXO(utxo *UTXO) bool {
	return utxo.UTXOID.BlockNum == 0
}
