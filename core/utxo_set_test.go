package core

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/store"
)

func TestReadWriteUtxo(t *testing.T) {
	testDbFile := fmt.Sprint(os.TempDir(), "utxo_set_test_db", time.Now().Unix())
	t.Log("test db file,", testDbFile)
	db, err := store.NewLDBDatabase(testDbFile, 0, 0)
	defer func() {
		db.Close()
		os.RemoveAll(testDbFile)
	}()

	if err != nil {
		t.Fatal(err)
	}
	us := NewUTXOSet(db)

	senderKey, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(senderKey.PublicKey)
	u1 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 1, TxIndex: 1, OutIndex: 1},
		TxOut:  types.TxOut{Owner: sender, Amount: big.NewInt(50)},
	}
	if err := us.Put(u1); err != nil {
		t.Fatal(err, "can not save utxo")
	}

	u2 := us.Get(u1.ID())
	if !reflect.DeepEqual(u1, u2) {
		t.Fatal("utxo retrieved is not correct")
	}
}
