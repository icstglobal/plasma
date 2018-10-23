package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type DepositEventHandler func(depositBlockNum *big.Int, depositor, token common.Address, amount *big.Int) error

type BlockSubmiittedEventHandler func(lastDepositBlockNum, currentBlockNum *big.Int) error
