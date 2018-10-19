package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/icstglobal/plasma/store"
	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/common"
	// "github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/icstglobal/go-icst/chain"
	"github.com/icstglobal/go-icst/chain/eth"
	"github.com/icstglobal/plasma/core/types"

	"github.com/spf13/viper"
	"reflect"
	"time"
)

const (
	DepositEventName         = "Deposit"
	ExitedStartEventName     = "ExistedStart"
	SubmitBlockMethodName    = "submitBlock"
	FromBlockKey             = "RootChain.FromBlock"
	CurrentBlockEventDataKey = "RootChain.CurrentBlockEventData"
)

type RootChain struct {
	chain     chain.Chain
	sub       map[string]func(event *chain.ContractEvent) // map topic0 to name
	abiStr    string
	cxAddr    string
	txsCh     chan types.Transactions
	chainDb   store.Database // Block chain database
	fromBlock int64
}

type CurrentBlockEventData struct {
	blockNum uint64
	hashList [][]byte
}

func (self *CurrentBlockEventData) DelLastBlockEventHash(chainDb store.Database) {
	newDbTx := chainDb.BeginTx()
	if !newDbTx {
		log.Error("db.BeginTx Error")
		return
	}
	for _, v := range self.hashList {
		err := chainDb.Delete(v)
		if err != nil {
			log.WithError(err).Error("chainDb.Delete Error")
			continue
		}
	}
	if err := chainDb.CommitTx(); err != nil {
		log.WithError(err).Error("failed to commit db tx")
		_err := chainDb.RollbackTx()
		if _err != nil {
			log.Error("db.RollbackTx Error:", _err.Error())
		}
		return
	}
	self.hashList = [][]byte{}
}

type DepositEvent struct {
	Depositor    common.Address
	DepositBlock *big.Int
	Token        common.Address
	Amount       *big.Int
	Raw          ethtypes.Log // Blockchain specific contextual infos
}

// chain
func NewRootChain(url string, abiStr string, cxAddr string, chainDb store.Database) (*RootChain, error) {

	fromBlock := viper.GetInt64("rootchain.fromBlock")
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect eth rpc endpoint {%v}, err is:%v", url, err)
	}
	blc := eth.NewChainEthereum(client)
	chain.Set(chain.Eth, blc)
	rc := &RootChain{
		chain:     blc,
		sub:       make(map[string]func(event *chain.ContractEvent)),
		abiStr:    abiStr,
		cxAddr:    cxAddr,
		chainDb:   chainDb,
		fromBlock: fromBlock,
	}
	// register dealing func
	rc.sub[DepositEventName] = rc.dealWithDepositEvent
	rc.sub[ExitedStartEventName] = rc.dealWithExitStartedEvent

	var depositEvent DepositEvent
	// start loop to sync root chain event
	go rc.loopEvent(DepositEventName, depositEvent)
	// go rc.loopEvent(ExitedStartEventName)

	return rc, nil
}

func (rc *RootChain) SetTxsCh(txsCh chan types.Transactions) {
	rc.txsCh = txsCh
}

// GetFromBlock get fromBlock from db. if not exist, get it from config.
func (rc *RootChain) GetFromBlock() (*big.Int, error) {
	var fromBlock *big.Int
	hasFromBlock, err := rc.chainDb.Has([]byte(FromBlockKey))
	if err != nil {
		log.WithError(err).Error("chainDb Has key Error.")
		return nil, err
	}
	if hasFromBlock {
		fromBlockBytes, err := rc.chainDb.Get([]byte(FromBlockKey))
		if err != nil {
			log.WithError(err).Error("chainDb Get FromBlock Error.")
			return nil, err
		}
		fromBlock = new(big.Int).SetBytes(fromBlockBytes)

	} else {
		fromBlock = big.NewInt(rc.fromBlock)
	}
	return fromBlock, nil
}

func (rc *RootChain) PutFromBlock(fromBlock *big.Int) error {
	//save fromBlock to db
	return rc.chainDb.Put([]byte(FromBlockKey), fromBlock.Bytes())
}

func (rc *RootChain) PutCurrentBlockEventData(eventData *CurrentBlockEventData) error {
	jsonBytes, err := json.Marshal(eventData)
	if err != nil {
		log.WithError(err).Error("json.Marshal Error.")
		return err
	}

	return rc.chainDb.Put([]byte(CurrentBlockEventDataKey), jsonBytes)
}

func (rc *RootChain) loopEvent(eventName string, event interface{}) {

	fromBlock, err := rc.GetFromBlock()
	if err != nil {
		return
	}

	log.Debugf("fromBlock:%v", fromBlock)

	cxAddrBytes, err := hex.DecodeString(rc.cxAddr)
	if err != nil {
		log.Error("Decode cxAddr Error:", err)
		return
	}
	currentBlockEventData := new(CurrentBlockEventData)
	if err != nil {
		log.Errorf("chain.GetCurrentBlockEventData: %v", err)
		return
	}

	for {
		events, err := rc.chain.GetContractEvents(context.Background(), cxAddrBytes, fromBlock, nil, rc.abiStr, eventName, reflect.TypeOf(event))
		if err != nil {
			log.Errorf("chain.GetEvents: %v", err)
			return
		}

		for i, event := range events {
			// get event hash
			key, value := rc.hashEvent(event)
			// save event hash to currentdata
			currentBlockEventData.blockNum = event.BlockNum
			currentBlockEventData.hashList = append(currentBlockEventData.hashList, key)
			log.Debugf("event key hex: %v blockNumber:%v", hex.EncodeToString(key), event.BlockNum)
			// filter repeat block
			hasKey, err := rc.chainDb.Has(key)
			if err != nil {
				log.WithError(err).Error("chainDb Has key Error.")
				return
			}
			if hasKey {
				log.Warn("repeat event occured.")
				continue
			}
			rc.sub[eventName](event)
			// save event to db
			err = rc.chainDb.Put(key, value)
			if err != nil {
				log.WithError(err).Error("chainDb Put eventKey Error.")
				return
			}
			isLastEvent := (i+1 == len(events))
			if isLastEvent || currentBlockEventData.blockNum != events[i+1].BlockNum {
				fromBlock = big.NewInt(int64(event.BlockNum + 1))
				// save fromblock after finish one Block
				err = rc.PutFromBlock(fromBlock)
				if err != nil {
					log.WithError(err).Error("chainDb Has key Error.")
					return
				}
				currentBlockEventData.DelLastBlockEventHash(rc.chainDb)
			}
		}
		time.Sleep(time.Second * 2)
	}
}

func (rc *RootChain) dealWithDepositEvent(event *chain.ContractEvent) {

	out := event.V.(*DepositEvent)

	// construct tx
	txOut1 := &types.TxOut{Owner: out.Depositor, Amount: out.Amount}
	txOut2 := &types.TxOut{Owner: common.Address{}, Amount: big.NewInt(0)}
	txIn1 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 0, TxIndex: 0, OutIndex: 0},
		TxOut:  types.TxOut{Owner: common.Address{}, Amount: big.NewInt(0)},
	}
	txIn2 := &types.UTXO{
		UTXOID: types.UTXOID{BlockNum: 0, TxIndex: 0, OutIndex: 0},
		TxOut:  types.TxOut{Owner: common.Address{}, Amount: big.NewInt(0)},
	}

	fee := big.NewInt(1) // todo:fee
	tx := types.NewTransaction(txIn1, txIn2, txOut1, txOut2, fee)

	txs := make(types.Transactions, 0)
	txs = append(txs, tx)
	rc.txsCh <- txs

	log.Debugf("dealWithDepositEvent: %v blockNumber: %v", out.Depositor.Hex(), event.BlockNum)
}

func (rc *RootChain) hashEvent(event *chain.ContractEvent) ([]byte, []byte) {
	jsonBytes, err := json.Marshal(&event)
	if err != nil {
		log.WithError(err).Error("json.Marshal Error.")
		return nil, nil
	}

	// log.Debugf("event: %#v\n, %#v", event, string(jsonBytes))
	md5Bytes := md5.Sum(jsonBytes)
	return md5Bytes[:], jsonBytes
}

func (rc *RootChain) dealWithExitStartedEvent(event *chain.ContractEvent) {
}

func (rc *RootChain) PubKeyToAddress(privateKey *ecdsa.PrivateKey) []byte {
	return rc.chain.PubKeyToAddress(privateKey.Public().(*ecdsa.PublicKey))
}

func (rc *RootChain) SubmitBlock(block *types.Block, privateKey *ecdsa.PrivateKey) error {
	from := rc.PubKeyToAddress(privateKey)
	var root [32]byte
	copy(root[:], block.Header().TxHash.Bytes())
	callData := map[string]interface{}{
		"_root":    root,
		"blockNum": block.Number(),
	}
	tx, err := rc.chain.CallWithAbi(context.Background(), from, common.Hex2Bytes(rc.cxAddr), SubmitBlockMethodName, big.NewInt(0), callData, rc.abiStr)
	if err != nil {
		log.WithError(err).Error("submitblock callData", callData)
		return err
	}
	// sig tx
	sigBytes, err := ethcrypto.Sign(tx.Hash(), privateKey)
	if err != nil {
		log.WithError(err).Error("sign tx")
		return err
	}
	return rc.chain.ConfirmTrans(context.Background(), tx, sigBytes)
}
