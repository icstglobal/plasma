// Copyright 2016 The go-ethereum Authors
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

package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	// log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidChainId = errors.New("invalid chain id for signer")
	emptySignature    = make([]byte, 65)
)

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   common.Address
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(config *params.ChainConfig, blockNumber *big.Int) Signer {
	// Plasma chain supports only the latest singer of Ethereum
	return NewEIP155Signer(config.ChainID)
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *Transaction, s Signer, prv1 *ecdsa.PrivateKey, prv2 *ecdsa.PrivateKey) (*Transaction, error) {
	h := s.Hash(tx)
	var err error
	sig1, sig2 := emptySignature, emptySignature
	if sig1, err = crypto.Sign(h[:], prv1); err != nil {
		return nil, err
	}
	if prv2 != nil {
		if sig2, err = crypto.Sign(h[:], prv2); err != nil {
			return nil, err
		}
	}
	return tx.WithSignature(s, sig1, sig2)
}

// Sender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx *Transaction) ([2]common.Address, error) {
	// if sc := tx.from.Load(); sc != nil {
	// 	sigCache := sc.(sigCache)
	// 	// If the signer used to derive from in a previous
	// 	// call is not the same as used current, invalidate
	// 	// the cache.
	// 	if sigCache.signer.Equal(signer) {
	// 		return sigCache.from, nil
	// 	}
	// }

	// addr, err := signer.Sender(tx)
	// if err != nil {
	// 	return common.Address{}, err
	// }
	// tx.from.Store(sigCache{signer: signer, from: addr})
	// return addr, nil
	return signer.Sender(tx)
}

// Signer encapsulates transaction signature handling. Note that this interface is not a
// stable API and may change at any time to accommodate new protocol rules.
type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *Transaction) ([2]common.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(sig []byte) (r, s, v *big.Int)
	// Hash returns the hash to be signed.
	Hash(tx *Transaction) common.Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

// EIP155Signer implements Signer using the EIP155 rules.
type EIP155Signer struct {
	chainId, chainIdMul *big.Int
}

func NewEIP155Signer(chainId *big.Int) EIP155Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return EIP155Signer{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
}

func (s EIP155Signer) Equal(s2 Signer) bool {
	eip155, ok := s2.(EIP155Signer)
	return ok && eip155.chainId.Cmp(s.chainId) == 0
}

var big8 = big.NewInt(8)

func (s EIP155Signer) Sender(tx *Transaction) (senders [2]common.Address, err error) {
	txHash := s.Hash(tx)
	for i, sig := range tx.data.Sigs {
		chainId := deriveChainId(sig.V)
		if chainId.Cmp(s.chainId) != 0 {
			return senders, ErrInvalidChainId
		}
		V := sig.V.Sub(sig.V, s.chainIdMul)
		V.Sub(V, big8)
		if senders[i], err = recoverPlain(txHash, sig.R, sig.S, V); err != nil {
			return senders, err
		}
	}
	return senders, nil
}

// SignatureValues returns signature values. This signature
func (s EIP155Signer) SignatureValues(sig []byte) (R, S, V *big.Int) {
	R, S, V = SignatureValues(sig)
	if s.chainId.Sign() != 0 {
		V = big.NewInt(int64(sig[64] + 35))
		V.Add(V, s.chainIdMul)
	}
	return R, S, V
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) Hash(tx *Transaction) common.Hash {
	return rlpHash([]interface{}{
		tx.data.Ins,
		tx.data.Outs,
		tx.data.Fee,
		s.chainId, uint(0), uint(0),
	})
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func SignatureValues(sig []byte) (r, s, v *big.Int) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, false) {
		return common.Address{}, ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

// deriveChainId derives the chain id from the given v parameter
func deriveChainId(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}
