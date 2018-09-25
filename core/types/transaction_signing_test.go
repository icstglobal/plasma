package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestEIP155Signing(t *testing.T) {
	senderKey, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(senderKey.PublicKey)

	receiverKey, _ := crypto.GenerateKey()
	receiver := crypto.PubkeyToAddress(receiverKey.PublicKey)

	signer := NewEIP155Signer(big.NewInt(1))

	tx1 := &Transaction{}
	in1 := &UTXO{UTXOID: UTXOID{BlockNum: 1, TxIndex: 0, OutIndex: 0}, Owner: sender, Amount: big.NewInt(50)}
	in2 := &UTXO{UTXOID: UTXOID{BlockNum: 1, TxIndex: 1, OutIndex: 0}, Owner: sender, Amount: big.NewInt(50)}
	tx1.data.Ins = [2]*UTXO{in1, in2}

	out1 := &TxOut{Owner: receiver, Amount: big.NewInt(90)}
	out2 := &TxOut{Owner: sender, Amount: big.NewInt(0)} //zero output
	tx1.data.Outs = [2]*TxOut{out1, out2}
	tx1.data.Fee = big.NewInt(10)

	t.Log("sign tx")
	tx1, err := SignTx(tx1, signer, senderKey, senderKey)
	if err != nil {
		t.Fatal("failed to sign tx", err)
	}

	buf, err := tx1.marshalJSON(true)
	if err != nil {
		t.Fatal("unable to marshal tx")
	}
	t.Logf("tx json:%v", string(buf))

	t.Log("recover senders")
	sendersRecovered, err := Sender(signer, tx1)
	if err != nil {
		t.Fatal(err)
	}
	for _, senderRecovered := range sendersRecovered {
		if senderRecovered.String() != sender.String() {
			t.Errorf("exected from and address to be equal. Got %x want %x", senderRecovered, sender)
		}
	}

}
