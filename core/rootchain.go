package core

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

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
)

const (
	DepositEventName        = "Deposit"
	ExitedStartEventName    = "ExistedStart"
	BlockSubmittedEventName = "BlockSubmitted"
	SubmitBlockMethodName   = "submitBlock"
)

type RootChain struct {
	chain  chain.Chain
	sub    map[string]func(event *chain.ContractEvent) error // map topic0 to name
	abiStr string
	cxAddr string
	txsCh  chan types.Transactions

	blockSubmiittedEventHandler BlockSubmiittedEventHandler
	depositEventHandler         DepositEventHandler
}

type DepositEvent struct {
	Depositor    common.Address
	DepositBlock *big.Int
	Token        common.Address
	Amount       *big.Int
	Raw          ethtypes.Log // Blockchain specific contextual infos
}

type BlockSubmittedEvent struct {
	Root                []byte
	LastDepositBlockNum *big.Int
	SubmittedBlockNum   *big.Int
	Time                *big.Int
}

// chain
func NewRootChain(url string, abiStr string, cxAddr string) (*RootChain, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect eth rpc endpoint {%v}, err is:%v", url, err)
	}
	log.WithFields(log.Fields{"url": url, "abi": abiStr, "contract addr": cxAddr}).Info("connected to root chain")
	blc := eth.NewChainEthereum(client)
	chain.Set(chain.Eth, blc)
	rc := &RootChain{
		chain:  blc,
		sub:    make(map[string]func(event *chain.ContractEvent) error),
		abiStr: abiStr,
		cxAddr: cxAddr,
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
	// go rc.loopEvent(DepositEventName, depositEvent)
	// go rc.loopEvent(ExitedStartEventName)

	return rc, nil
}

func (rc *RootChain) loopEvents(eventTypes map[string]reflect.Type) {
	log.Debug("rootchain:loop events")
	fromBlock := big.NewInt(100)

	cxAddrBytes, err := hex.DecodeString(rc.cxAddr)
	if err != nil {
		log.Error("Decode cxAddr Error:", err)
		return
	}

	for {
		log.WithField("fromBlock", fromBlock).Debug("rootchain:try to get contract events")
		events, err := rc.chain.GetContractEvents(context.Background(), cxAddrBytes, fromBlock, nil, rc.abiStr, eventTypes)
		if err != nil {
			log.Errorf("chain.GetEvents: %v", err)
			return
		}
		log.WithField("count", len(events)).Debug("contract events returned from root chain")
		for _, event := range events {
			rc.sub[event.Name](event)
		}
		time.Sleep(time.Second * 2)
		if len(events) > 0 {
			fromBlock = big.NewInt(int64(events[len(events)-1].BlockNum) + 1)
			log.WithField("fromBlock", fromBlock).Debug("loop events for next block")
		}
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
