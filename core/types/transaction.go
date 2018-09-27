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
	// ErrInvalidSig returns when the signature is not valid
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// Transaction is the container of tx data
type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
}

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
	Owner  common.Address `json:"owner"`
	Amount *big.Int       `json:"amount"`
}

// ID returns the identity of a UTXO obj
func (u UTXO) ID() UTXOID {
	return u.UTXOID
}

// Equals check if two UTXO objs are identical
func (u UTXO) Equals(other UTXO) bool {
	return u.UTXOID.Equals(other.UTXOID)
}

// TxOut is the tx output
type TxOut struct {
	Owner  common.Address `json:"owner"`
	Amount *big.Int       `json:"amount"`
}

// Sig represents the signature values
type Sig struct {
	V *big.Int `json:"v"`
	R *big.Int `json:"r"`
	S *big.Int `json:"s"`
}

type txdata struct {
	Ins  [2]*UTXO  `json:"ins"`
	Outs [2]*TxOut `json:"outs"`
	Fee  *big.Int  `json:"fee"`

	Sigs [2]Sig `json:"sigs"`
	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

// NewTransaction makes a tx from given inputs and out puts
func NewTransaction(in1, in2 *UTXO, out1, out2 *TxOut, fee *big.Int) *Transaction {
	tx := Transaction{}
	tx.data.Ins[0] = in1
	tx.data.Ins[1] = in2
	tx.data.Outs[0] = out1
	tx.data.Outs[1] = out2
	tx.data.Fee = fee

	return &tx
}

//GetInsCopy returns a copy of the tx ins
func (tx *Transaction) GetInsCopy() []*UTXO {
	log.WithFields(log.Fields{"in1": tx.data.Ins[0], "in2": tx.data.Ins[1]}).Debug("Transaction.GetInsCopy")
	var copy = [2]*UTXO{&UTXO{}, &UTXO{}}
	if err := copier.Copy(&copy, tx.data.Ins); err != nil {
		log.WithError(err).Error("failed to copy tx.data.Ins")
		return nil
	}
	return copy[:]
}

//GetOutsCopy returns a copy of the tx outs
func (tx *Transaction) GetOutsCopy() []*TxOut {
	var copy = [2]*TxOut{&TxOut{}, &TxOut{}}
	if err := copier.Copy(&copy, tx.data.Outs); err != nil {
		log.WithError(err).Error("failed to copy tx.data.Outs")
		return nil
	}
	return copy[:]
}

// Fee returns a copy of the tx fee
func (tx *Transaction) Fee() *big.Int {
	fcopy := new(big.Int) // fcopy = 0
	// new = old = old + 0
	fcopy.Add(tx.data.Fee, fcopy)
	return fcopy
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
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	return tx.marshalJSON(false)
}

func (tx *Transaction) marshalJSON(indent bool) ([]byte, error) {
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
	// validate signatures after doing marshal
	sigs := dec.Sigs
	for _, s := range sigs {
		chainID := deriveChainId(s.V).Uint64()
		V := byte(s.V.Uint64() - 35 - 2*chainID)
		if !crypto.ValidateSignatureValues(V, s.R, s.S, false) {
			return ErrInvalidSig
		}
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
func (tx *Transaction) WithSignature(signer Signer, sig1, sig2 []byte) (*Transaction, error) {
	cpy := &Transaction{data: tx.data}
	cpy.data.Sigs[0].R, cpy.data.Sigs[0].S, cpy.data.Sigs[0].V = signer.SignatureValues(sig1)
	cpy.data.Sigs[1].R, cpy.data.Sigs[1].S, cpy.data.Sigs[1].V = signer.SignatureValues(sig2)
	return cpy, nil
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
