package types

import (
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type UTXOIn struct {
	BlockNum uint64 `json:"blockNum"`
	TxIndex  uint32 `json:"txIndex"`
	OutIndex byte   `json:"outIndex"`
	Sig      []byte `json:"sig"`
}

type UTXOOut struct {
	Owner  common.Address `json:"owner"`
	Amount *big.Int       `json:"amount"`
}

type txdata struct {
	Ins  [2]*UTXOIn  `json:"ins"`
	Outs [2]*UTXOOut `json:"outs"`
	Fee  *big.Int    `json:"fee"`

	// Signature values
	V *big.Int `json:"v"`
	R *big.Int `json:"r"`
	S *big.Int `json:"s"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func NewTransaction(in1, in2 *UTXOIn, out1, out2 *UTXOOut, fee *big.Int) *Transaction {
	tx := Transaction{}
	tx.data.Ins[0] = in1
	tx.data.Ins[1] = in2
	tx.data.Outs[0] = out1
	tx.data.Outs[1] = out2
	tx.data.Fee = fee

	return &tx
}

func (tx *Transaction) ChainId() *big.Int {
	return deriveChainId(tx.data.V)
}

//GetInsCopy returns a copy of the tx ins
func (tx *Transaction) GetInsCopy() []*UTXOIn {
	copy := make([]*UTXOIn, len(tx.data.Ins))
	if err := copier.Copy(copy, tx.data.Ins); err != nil {
		log.WithError(err).Error("failed to copy tx.data.Ins")
		return nil
	}
	return copy
}

//GetOutsCopy returns a copy of the tx outs
func (tx *Transaction) GetOutsCopy() []*UTXOOut {
	copy := make([]*UTXOOut, len(tx.data.Outs))
	if err := copier.Copy(copy, tx.data.Outs); err != nil {
		log.WithError(err).Error("failed to copy tx.data.Outs")
		return nil
	}
	return copy
}

// Fee returns a copy of the tx fee
func (tx *Transaction) Fee() *big.Int {
	fcopy := new(big.Int) // fcopy = 0
	// new = old = old + 0
	fcopy.Add(tx.data.Fee, fcopy)
	return fcopy
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *Transaction) Protected() bool {
	return isProtectedV(tx.data.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 are considered unprotected
	return true
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON(indent bool) ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	if indent {
		return json.MarshalIndent(&data, "", "\t")
	}

	return json.Marshal(&data)
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	var V byte
	if isProtectedV(dec.V) {
		chainID := deriveChainId(dec.V).Uint64()
		V = byte(dec.V.Uint64() - 35 - 2*chainID)
	} else {
		V = byte(dec.V.Uint64() - 27)
	}
	if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
		return ErrInvalidSig
	}
	*tx = Transaction{data: dec}
	return nil
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

func (tx *Transaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// Remove delete the i'th element from s
func (s *Transactions) Remove(i int) {
	// remove tx by swap it to the last pos in slice and then truncate the slice
	s.Swap(i, s.Len()-1)
	*s = (*s)[:s.Len()-1]
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}
