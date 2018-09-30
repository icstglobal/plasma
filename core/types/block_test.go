package types

import (
	"bytes"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// from bcValidBlockTest.json, "SimpleTx"
func TestBlockEncoding(t *testing.T) {
	blockEnc := common.FromHex("8c4f87ba083cafc574e1f51ba9dc0568fc617a08ea2429fb384059c972f13b19fa1c8dd5594b5fc7bdf42d5f2a1ca0b1cbeda1080903bccae74a066cafc574e1f51ba9dc0568fc617a08ea2429fb384059c972f13b19fa1c8dd57018080a083cafc574e1f51ba9dc0568fc617a08ea2429fb384059c972f13b19fa1c8dd55f845f8431ba09bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094fa08a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b1")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	// check("Difficulty", block.Difficulty(), big.NewInt(131072))
	// check("Coinbase", block.Coinbase(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	// check("Hash", block.Hash(), common.HexToHash("0a5843ac1cb04865017cb35a57b50b07084e5fcee39b5acadade33149f4fff9e"))
	// check("Nonce", block.Nonce(), uint64(0xa13a5a8c8f2bb1c4))
	// check("Time", block.Time(), big.NewInt(1426516743))
	// check("Size", block.Size(), common.StorageSize(len(blockEnc)))
	// check("len(Transactions)", len(block.Transactions()), 1)

	tx1 := &Transaction{}
	in1 := &UTXO{UTXOID: UTXOID{BlockNum: 1, TxIndex: 0, OutIndex: 0}, Owner: common.HexToAddress("53e44dece9d583ab238ded76636e2cc3b16d2ddd48f19c22545bf9c4878539732fcf6612069f082c96ab159c42de5dbd64a6e4088dbe1756d116a253ea532b6501")}
	in2 := &UTXO{UTXOID: UTXOID{BlockNum: 1, TxIndex: 1, OutIndex: 0}, Owner: common.HexToAddress("53e44dece9d583ab238ded76636e2cc3b16d2ddd48f19c22545bf9c4878539732fcf6612069f082c96ab159c42de5dbd64a6e4088dbe1756d116a253ea532b6501")}
	tx1.data.Ins = [2]*UTXO{in1, in2}

	out1 := &TxOut{Owner: common.HexToAddress(""), Amount: big.NewInt(100)}
	out2 := &TxOut{Owner: common.HexToAddress(""), Amount: big.NewInt(0)} //zero output
	tx1.data.Outs = [2]*TxOut{out1, out2}
	tx1.data.Fee = big.NewInt(10)

	sig := common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100")
	tx1, _ = tx1.WithSignature(EIP155Signer{}, sig, sig)
	block.transactions = append(block.transactions, tx1)

	//inti a new block obj
	block = Block{}
	block.header = new(Header)
	block.header.ParentHash = common.HexToHash("83cafc574e1f51ba9dc0568fc617a08ea2429fb384059c972f13b19fa1c8dd55")
	block.header.Coinbase = common.HexToAddress("B5fc7BDF42D5f2a1CA0B1cbedA1080903BCCaE74")
	block.header.Number = big.NewInt(1)
	block.header.Time = big.NewInt(0)
	block.header.TxHash = common.HexToHash("66cafc574e1f51ba9dc0568fc617a08ea2429fb384059c972f13b19fa1c8dd57") //TODO: fake, should gen from transactions Merkle tree

	// fmt.Println(block.Transactions()[0].Hash().Hex())
	// fmt.Println(tx1.data)
	// fmt.Println(tx1.Hash())
	check("len(Transactions)", len(block.Transactions()), 1)
	check("Transactions[0].Hash", block.Transactions()[0].Hash(), tx1.Hash())

	ourBlockEnc, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}
	fmt.Println("block encoded:", common.ToHex(ourBlockEnc))
	if !bytes.Equal(ourBlockEnc, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", ourBlockEnc, blockEnc)
	}
}
