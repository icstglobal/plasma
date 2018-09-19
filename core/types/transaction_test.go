package types

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestFee(t *testing.T) {
	tx := new(Transaction)
	tx.data.Fee = big.NewInt(10)

	fcopy := tx.Fee()
	fcopy = fcopy.Add(fcopy, big.NewInt(1))

	if fcopy.Cmp(tx.data.Fee) == 0 {
		t.Logf("fee:%v, fcopy:%v", tx.data.Fee, fcopy)
		t.Fail()
	}
}

func BenchmarkBigIntCopyByAdd(b *testing.B) {
	tx := new(Transaction)
	tx.data.Fee, _ = new(big.Int).SetString("100000000222222222222222222220000000000000000000", 10)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tx.Fee()
	}
}

func TestMarshalTx(t *testing.T) {
	tx1 := &Transaction{}
	in1 := &UTXOIn{BlockNum: 1, TxIndex: 0, OutIndex: 0, Sig: common.Hex2Bytes("53e44dece9d583ab238ded76636e2cc3b16d2ddd48f19c22545bf9c4878539732fcf6612069f082c96ab159c42de5dbd64a6e4088dbe1756d116a253ea532b6501")}
	in2 := &UTXOIn{BlockNum: 1, TxIndex: 1, OutIndex: 0, Sig: common.Hex2Bytes("53e44dece9d583ab238ded76636e2cc3b16d2ddd48f19c22545bf9c4878539732fcf6612069f082c96ab159c42de5dbd64a6e4088dbe1756d116a253ea532b6501")}
	tx1.data.Ins = [2]*UTXOIn{in1, in2}

	out1 := &UTXOOut{Owner: common.HexToAddress(""), Amount: big.NewInt(100)}
	out2 := &UTXOOut{Owner: common.HexToAddress(""), Amount: big.NewInt(0)} //zero output
	tx1.data.Outs = [2]*UTXOOut{out1, out2}
	tx1.data.Fee = big.NewInt(10)
	tx1, _ = tx1.WithSignature(HomesteadSigner{}, common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100"))

	buf, err := tx1.MarshalJSON(true)
	if err != nil {
		t.Fatal("unable to marshal tx")
	}

	tx2 := new(Transaction)
	err = tx2.UnmarshalJSON(buf)
	if err != nil {
		t.Logf("json: %v", string(buf))
		t.Fatal("unable to unmarshal tx")
	}
	if reflect.DeepEqual(tx1, tx2) {
		t.Logf("json: %v", string(buf))
		t.Fatal("tx not correctly marshaled")
	}
}
