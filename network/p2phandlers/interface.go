package p2phandlers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/plasma/core/types"
	host "github.com/libp2p/go-libp2p-host"
	ipeer "github.com/libp2p/go-libp2p-peer"
	"io"
)

type Peer interface {
	host.Host
	SendMsg(proto string, peerid ipeer.ID, data interface{}) error
	Decode(r io.Reader, out interface{}) error
	PeerIDsWithoutTx(hash common.Hash) []ipeer.ID
	MarkTxs(peerid ipeer.ID, txs types.Transactions)
}
