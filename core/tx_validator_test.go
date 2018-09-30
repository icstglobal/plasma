package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/icstglobal/plasma/core/types"
)

func TestTxValidatorDuplicatSpent(t *testing.T) {
	signer := types.NewEIP155Signer(big.NewInt(1))

	senderKey, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(senderKey.PublicKey)

	receiverKey, _ := crypto.GenerateKey()
	receiver := crypto.PubkeyToAddress(receiverKey.PublicKey)

	in1 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 0, OutIndex: 0},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}
	in2 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 1, OutIndex: 0},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}
	in3 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 2, OutIndex: 0},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}

	var ur UtxoReader = &DummyUtxoReader{
		utxoset: map[types.UTXOID]*types.UTXO{in1.ID(): in1, in2.ID(): in2, in3.ID(): in3},
	}
	validator := NewUtxoTxValidator(signer, ur)

	out1 := &types.TxOut{Owner: receiver, Amount: big.NewInt(90)}
	out2 := &types.TxOut{Owner: sender, Amount: big.NewInt(0)} //zero output
	fee := big.NewInt(10)

	// use "in1" the second time in a new tx
	tx1 := types.NewTransaction(in1, in1, out1, out2, fee)
	tx1, _ = types.SignTx(tx1, signer, senderKey, senderKey)

	if err := validator.Validate(tx1); err != ErrDuplicateSpent {
		t.Error(err)
		t.Fatal("should report duplicate spent error")
	}
}

func TestTxValidatorInvalidOutputAmount(t *testing.T) {
	signer := types.NewEIP155Signer(big.NewInt(1))

	senderKey, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(senderKey.PublicKey)

	receiverKey, _ := crypto.GenerateKey()
	receiver := crypto.PubkeyToAddress(receiverKey.PublicKey)

	in1 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 0, OutIndex: 0},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}
	in2 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 1, OutIndex: 0},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}
	in3 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 2, OutIndex: 0},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}

	var ur UtxoReader = &DummyUtxoReader{
		utxoset: map[types.UTXOID]*types.UTXO{in1.ID(): in1, in2.ID(): in2, in3.ID(): in3},
	}
	validator := NewUtxoTxValidator(signer, ur)

	out1 := &types.TxOut{Owner: receiver, Amount: big.NewInt(90)}
	out2 := &types.TxOut{Owner: sender, Amount: big.NewInt(0)} //zero output
	fee := big.NewInt(10)

	// use "in1" the second time in a new tx
	tx1 := types.NewTransaction(in1, in2, out1, out2, fee)
	tx1, _ = types.SignTx(tx1, signer, senderKey, senderKey)

	if err := validator.Validate(tx1); err != ErrInvalidOutputAmount {
		t.Error(err)
		t.Fatal("should report ErrInvalidOutputAmount")
	}
}
