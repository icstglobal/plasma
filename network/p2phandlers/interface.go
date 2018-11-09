package p2phandlers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/plasma/core/types"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	ipeer "github.com/libp2p/go-libp2p-peer"
	// "io"
)

type Host interface {
	host.Host
	SendMsg(proto string, peerid ipeer.ID, data interface{}) error
	Request(proto string, peerid ipeer.ID, data interface{}) ([]byte, error)
	Response(s inet.Stream, data interface{}) error
	Decode(data []byte, out interface{}) error
	PeerIDs() []ipeer.ID
	PeerIDsWithoutTx(hash common.Hash) []ipeer.ID
	PeerIDsWithoutBlock(hash common.Hash) []ipeer.ID
	MarkTxs(peerid ipeer.ID, txs types.Transactions)
	MarkBlock(peerid ipeer.ID, block *types.Block)
}
