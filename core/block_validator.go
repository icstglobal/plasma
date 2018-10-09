package core

import (
	"fmt"

	"github.com/icstglobal/plasma/consensus"
	"github.com/icstglobal/plasma/core/types"
	log "github.com/sirupsen/logrus"
)

// UtxoBlockValidator is responsible for validating block /
// UtxoBlockValidator implements BlockValidator.
type UtxoBlockValidator struct {
	blcReader   BlockReader
	utxoReader  UtxoReader
	txValidator TxValidator
}

// NewUtxoBlockValidator returns a new block validator which is safe for re-use
func NewUtxoBlockValidator(s types.Signer, br BlockReader, ur UtxoReader) *UtxoBlockValidator {
	validator := &UtxoBlockValidator{
		blcReader:   br,
		utxoReader:  ur,
		txValidator: NewUtxoTxValidator(s, ur),
	}
	return validator
}

// ValidateBody validates the given block's uncles and verifies the the block
// header's transaction. The headers are assumed to be already
// validated at this point.
func (v *UtxoBlockValidator) ValidateBody(block *types.Block) error {
	// Check whether the block's known, and if not, that it's linkable
	if v.blcReader.HasBlock(block.Hash(), block.NumberU64()) {
		return ErrKnownBlock
	}
	//different with Ethereum here, "fork" will never happen
	if !v.blcReader.HasBlock(block.ParentHash(), block.NumberU64()-1) {
		return consensus.ErrUnknownAncestor
	}
	// Header validity is known at this point, check transactions
	header := block.Header()
	if hash := types.DeriveSha(block.Transactions()); hash != header.TxHash {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxHash)
	}

	return v.validateTx(block.Transactions())
}

func (v *UtxoBlockValidator) validateTx(trans types.Transactions) error {
	spent := make(map[types.UTXOID]*types.UTXO)
	var err error
	for _, tx := range trans {
		if err = v.txValidator.Validate(tx); err != nil {
			return err
		}
		ins := tx.GetInsCopy()
		for _, in := range ins {
			if in == nil {
				continue
			}
			if _, exist := spent[in.ID()]; exist {
				return ErrDuplicateSpent
			}
			// utxo should exists in utxo set, after passing the tx validation
			utxo := v.utxoReader.Get(in.ID())
			if utxo == nil {
				// fatal error, log and exit
				log.Fatal("UtxoValidator.ValidateBody:utxo not exist, it should not pass the tx validation")
			}
			// mark it as spent, so won't be spent later in other tx
			spent[in.ID()] = utxo
		}
	}
	return nil
}
