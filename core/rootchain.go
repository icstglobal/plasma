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

	"reflect"
	"time"

	"github.com/spf13/viper"
)

const (
	DepositEventName         = "Deposit"
	ExitedStartEventName     = "ExistedStart"
	BlockSubmittedEventName  = "BlockSubmitted"
	SubmitBlockMethodName    = "submitBlock"
	FromBlockKey             = "RootChain.FromBlock"
	CurrentBlockEventDataKey = "RootChain.CurrentBlockEventData"
)

type RootChain struct {
	chain     chain.Chain
	sub       map[string]func(event *chain.ContractEvent) error // map topic0 to name
	abiStr    string
	cxAddr    string
	txsCh     chan types.Transactions
	fromBlock int64
	chainDb   store.Database // Block chain database

	blockSubmiittedEventHandler BlockSubmiittedEventHandler
	depositEventHandler         DepositEventHandler
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

type BlockSubmittedEvent struct {
	Root                [32]byte
	LastDepositBlockNum *big.Int
	SubmittedBlockNum   *big.Int
	Time                *big.Int
}

// chain
func NewRootChain(url string, abiStr string, cxAddr string, chainDb store.Database) (*RootChain, error) {

	fromBlock := viper.GetInt64("rootchain.fromBlock")
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect eth rpc endpoint {%v}, err is:%v", url, err)
	}
	log.WithFields(log.Fields{"url": url, "abi": abiStr, "contract addr": cxAddr}).Info("connected to root chain")
	blc := eth.NewChainEthereum(client)
	chain.Set(chain.Eth, blc)
	rc := &RootChain{
		chain:     blc,
		sub:       make(map[string]func(event *chain.ContractEvent) error),
		abiStr:    abiStr,
		cxAddr:    cxAddr,
		chainDb:   chainDb,
		fromBlock: fromBlock,
	}
	// register dealing func
	rc.sub[DepositEventName] = rc.dealWithDepositEvent
	rc.sub[BlockSubmittedEventName] = rc.dealWithBlockSubmittedEvent
	rc.sub[ExitedStartEventName] = rc.dealWithExitStartedEvent

	// start loop to sync root chain event
	eventTypes := map[string]reflect.Type{
		DepositEventName:        reflect.TypeOf(DepositEvent{}),
		BlockSubmittedEventName: reflect.TypeOf(BlockSubmittedEvent{}),
	}
	go rc.loopEvents(eventTypes)
	// go rc.loopEvent(ExitedStartEventName)

	return rc, nil
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

//PutFromBlock save fromBlock to db
func (rc *RootChain) PutFromBlock(fromBlock *big.Int) error {

	return rc.chainDb.Put([]byte(FromBlockKey), fromBlock.Bytes())
}

func (rc *RootChain) loopEvents(eventTypes map[string]reflect.Type) {
	log.Debug("rootchain:loop events")
	//TODO: for local test only
	fromBlock := big.NewInt(100)
	// fromBlock, err := rc.GetFromBlock()
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
		// log.WithField("fromBlock", fromBlock).Debug("rootchain:try to get contract events")
		events, err := rc.chain.GetContractEvents(context.Background(), cxAddrBytes, fromBlock, nil, rc.abiStr, eventTypes)
		if err != nil {
			log.Errorf("chain.GetEvents: %v", err)
			return
		}
		// log.WithField("count", len(events)).Debug("contract events returned from root chain")
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
			rc.sub[event.Name](event)
			// save event to db
			err = rc.chainDb.Put(key, value)
			if err != nil {
				log.WithError(err).Error("chainDb Put eventKey Error.")
				return
			}
			isLastEvent := (i+1 == len(events))
			if isLastEvent || currentBlockEventData.blockNum != events[i+1].BlockNum {
				fromBlock = big.NewInt(int64(event.BlockNum + 1))
				log.WithField("fromBlock", fromBlock).Debug("loop events for next block")
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

func (rc *RootChain) dealWithDepositEvent(event *chain.ContractEvent) error {
	if rc.depositEventHandler == nil {
		log.Warn("depositEventHandler is not registered")
		return nil
	}

	out := event.V.(*DepositEvent)
	log.Debugf("RootChain.dealWithDepositEvent: %v rootChainBlockNumber: %v, depositBlock:%v, depositAmount:%v, depositorAddr:%v", out.Depositor.Hex(), event.BlockNum, out.DepositBlock, out.Amount, out.Depositor)
	return rc.depositEventHandler(out.DepositBlock, out.Depositor, out.Token, out.Amount)
}

func (rc *RootChain) RegisterDepositEventHandler(handler DepositEventHandler) {
	rc.depositEventHandler = handler
}

func (rc *RootChain) RegisterBlockSubmittedEventHandler(handler BlockSubmiittedEventHandler) {
	rc.blockSubmiittedEventHandler = handler
}

func (rc *RootChain) dealWithBlockSubmittedEvent(event *chain.ContractEvent) error {
	if rc.blockSubmiittedEventHandler == nil {
		log.Warn("no BlockSubmiittedEvent Handler registered")
		return nil
	}

	e := event.V.(*BlockSubmittedEvent)
	lastDeposiBlockNum := e.LastDepositBlockNum
	submittedBlockNum := e.SubmittedBlockNum
	rc.blockSubmiittedEventHandler(lastDeposiBlockNum, submittedBlockNum)

	return nil
}

func (rc *RootChain) dealWithExitStartedEvent(event *chain.ContractEvent) error {
	return nil
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
	log.Debugf("SubmitBlock root %v", block.Header().TxHash.Hex())
	// return nil
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
	log.Debugf("SubmitBlock:%v", tx.Hex())
	return rc.chain.ConfirmTrans(context.Background(), tx, sigBytes)
}

func (rc *RootChain) RootChainBlockNums() (*big.Int, error) {
	cxAddrBytes, err := hex.DecodeString(rc.cxAddr)
	if err != nil {
		log.Error("Decode cxAddr Error:", err)
		return nil, err
	}

	ret := new(*big.Int)
	err = rc.chain.Query(context.Background(), cxAddrBytes, rc.abiStr, "getBlockCount", ret)
	log.Debugf("RootChainBlockNums ret:%v, err:%v", *ret, err)
	return *ret, err
}

func (rc *RootChain) GetRootChainBlockNumByIndex(index int64) (*big.Int, error) {
	cxAddrBytes, err := hex.DecodeString(rc.cxAddr)
	if err != nil {
		log.Error("Decode cxAddr Error:", err)
		return nil, err
	}
	arg := big.NewInt(index)

	ret := new(*big.Int)
	err = rc.chain.Query(context.Background(), cxAddrBytes, rc.abiStr, "blockNums", ret, arg)
	return *ret, err
}
