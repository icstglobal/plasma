package core

import (
	"context"
	"fmt"
	"math/big"

	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	// "github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/icstglobal/go-icst/chain"
	"github.com/icstglobal/go-icst/chain/eth"
	"github.com/icstglobal/plasma/core/types"

	"strings"
	"time"
)

const (
	DepositEventName     = "Deposit"
	ExitedStartEventName = "ExistedStart"
)

type RootChain struct {
	chain    chain.Chain
	sub      map[string]func(eventName string, log ethtypes.Log) // map topic0 to name
	cxAbi    abi.ABI
	operator *Operator
}

type DepositEvent struct {
	Depositor    common.Address
	DepositBlock *big.Int
	Token        common.Address
	Amount       *big.Int
	Raw          ethtypes.Log // Blockchain specific contextual infos
}

// chain
func NewRootChain(url string, abiStr string, operator *Operator) (*RootChain, error) {

	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect eth rpc endpoint {%v}, err is:%v", url, err)
	}
	blc := eth.NewChainEthereum(client)
	chain.Set(chain.Eth, blc)
	// parse abi
	abiParsed, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, err
	}
	rc := &RootChain{
		chain:    blc,
		sub:      make(map[string]func(eventName string, log ethtypes.Log)),
		cxAbi:    abiParsed,
		operator: operator,
	}
	// register dealing func
	rc.sub[DepositEventName] = rc.dealWithDepositEvent
	rc.sub[ExitedStartEventName] = rc.dealWithExitStartedEvent

	// start loop to sync root chain event
	go rc.loopEvent(DepositEventName)
	go rc.loopEvent(ExitedStartEventName)

	return rc, nil
}

func (rc *RootChain) loopEvent(eventName string) {
	fromBlock := big.NewInt(100)
	topic := rc.cxAbi.Events[eventName].Id()
	topics := make([][]common.Hash, 1)
	topics[0] = append(topics[0], topic)
	for {
		logs, err := rc.chain.GetEvents(context.Background(), topics, fromBlock)
		if err != nil {
			log.Errorf("chain.GetEvents: %v", err)
			return
		}

		for _, log := range logs {
			rc.sub[eventName](eventName, log)
		}
		time.Sleep(time.Second * 2)
		if len(logs) > 0 {
			fromBlock = big.NewInt(int64(logs[len(logs)-1].BlockNumber) + 1)
		}
	}
}

func (rc *RootChain) dealWithDepositEvent(eventName string, _log ethtypes.Log) {
	// unpack log
	out := new(DepositEvent)
	err := rc.chain.UnpackLog(rc.cxAbi, out, eventName, _log)
	if err != nil {
		log.Errorf("UnpackLog Error: %v", err)
		return
	}
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
	rc.operator.AddTxs(txs)

	log.Debugf("dealWithDepositEvent: %v blockNumber: %v", out.Depositor.Hex(), _log.BlockNumber)
}

func (rc *RootChain) dealWithExitStartedEvent(eventName string, _log ethtypes.Log) {
}
