package p2phandlers

import (
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/plasma"
	// host "github.com/libp2p/go-libp2p-host"
	"bufio"
	inet "github.com/libp2p/go-libp2p-net"
	ipeer "github.com/libp2p/go-libp2p-peer"
	log "github.com/sirupsen/logrus"
)

const (
	txProto = "tx/0.0.1"
)

// TxHandler is a container for tx related handlers
type TxHandler struct {
	peer Peer
	pls  *plasma.Plasma
}

func NewTxHandler(peer Peer, pls *plasma.Plasma) *TxHandler {
	txHandler := &TxHandler{peer: peer, pls: pls}
	peer.SetStreamHandler(txProto, txHandler.recvTxs)
	go txHandler.broadcastTxs()
	return txHandler
}

// recevTx handles received transactions
func (handler *TxHandler) recvTxs(s inet.Stream) {
	log.Debug("recvTxs")
	txs := make([]*types.Transaction, 0)
	out := &types.Transactions{}
	err := handler.peer.Decode(bufio.NewReader(s), out)
	if err != nil {
		log.WithError(err).Error("recvTxs Decode Error!")
		return
	}

	// mark known tx
	handler.peer.MarkTxs(s.Conn().RemotePeer(), txs)

	handler.pls.TxPool().AddRemotes(txs)
}

// broadcastTxs broadcast received valid tx
func (handler *TxHandler) broadcastTxs() {
	log.Debug("broadcastTxs")
	// loop peers to send txs
	peers := handler.peer.Peerstore().Peers()
	log.Debug("peers:%v", peers, handler.pls)
	txsCh := handler.pls.TxPool().NewTxsChannel()
	for {
		select {
		case txs := <-txsCh:
			var txset = make(map[ipeer.ID]types.Transactions)

			// Broadcast transactions to a batch of peers not knowing about it
			for _, tx := range txs {
				peerids := handler.peer.PeerIDsWithoutTx(tx.Hash())
				for _, peerid := range peerids {
					txset[peerid] = append(txset[peerid], tx)
				}
				log.Debug("Broadcast transaction", "hash", tx.Hash(), "recipients", len(peerids))
			}
			for peerid, txs := range txset {
				handler.peer.SendMsg(txProto, peerid, txs)
			}
		}
	}
}
