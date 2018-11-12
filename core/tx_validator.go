package core

import (
	"math/big"
	"reflect"

	"github.com/icstglobal/plasma/core/types"
	log "github.com/sirupsen/logrus"
)

// UtxoTxValidator is a validator for transaction including utxo inputs
type UtxoTxValidator struct {
	signer     types.Signer
	utxoReader UtxoReader
}

// NewUtxoTxValidator creates a new TxValidator
func NewUtxoTxValidator(signer types.Signer, ur UtxoReader) *UtxoTxValidator {
	return &UtxoTxValidator{signer: signer, utxoReader: ur}
}

// Validate checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (v *UtxoTxValidator) Validate(tx *types.Transaction) error {
	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}

	if tx.Fee().Cmp(zeroFee) <= 0 {
		return ErrNotEnoughTxFee
	}

	// Make sure the transaction is signed properly
	senders, err := types.Sender(v.signer, tx)
	if err != nil {
		log.Error("types.Sender Error:", err)
		return ErrInvalidSender
	}

	ins := tx.GetInsCopy()
	// check duplicate utxo inputs in the same tx
	if ins[0].Equals(*ins[1]) {
		return ErrDuplicateSpent
	}

	totalInAmount := new(big.Int)
	for i, in := range ins {
		if in.ID().BlockNum == 0 {
			continue
		}
		log.WithFields(log.Fields{"blockNum": in.BlockNum, "txIndex": in.TxIndex, "outIndex": in.OutIndex}).Debug("validate tx in with utxo set")
		utxo := v.utxoReader.Get(in.ID())
		// make sure utxo exists and is unspent
		if utxo == nil {
			return ErrAlreadySpent
		}
		log.Debugf("utxo.Owner %v %v", utxo.Owner, senders[i].Hex())
		// the signer should own the unspent tx out
		if !reflect.DeepEqual(utxo.Owner, senders[i]) {
			return ErrTxOutNotOwned
		}
		totalInAmount = totalInAmount.Add(totalInAmount, utxo.Amount)
	}

	newOuts := tx.GetOutsCopy()
	totalOutAmount := new(big.Int)
	for _, out := range newOuts {
		if out.Amount == nil || out.Amount.Cmp(big0) == 0 {
			continue
		}
		if out.Amount.Cmp(big0) <= 0 {
			return ErrInvalidOutputAmount
		}
		totalOutAmount = totalOutAmount.Add(totalOutAmount, out.Amount)
	}
	// totalInAmount = totalOutAmount + fee
	log.Info("totalInAmount, totalOutAmount, fee", totalInAmount, totalOutAmount, tx.Fee())
	if totalInAmount.Cmp(totalOutAmount.Add(totalOutAmount, tx.Fee())) != 0 {
		return ErrTxTotalAmountNotEqual
	}

	return nil
}
