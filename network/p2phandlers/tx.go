package p2phandlers

import (
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/plasma"
	// host "github.com/libp2p/go-libp2p-host"
	// "bufio"
	inet "github.com/libp2p/go-libp2p-net"
	ipeer "github.com/libp2p/go-libp2p-peer"
	msgio "github.com/libp2p/go-msgio"
	log "github.com/sirupsen/logrus"
)

const (
	txProto = "tx/0.0.1"
)

// TxHandler is a container for tx related handlers
type TxHandler struct {
	host Host
	pls  *plasma.Plasma
}

func NewTxHandler(host Host, pls *plasma.Plasma) *TxHandler {
	txHandler := &TxHandler{host: host, pls: pls}
	host.SetStreamHandler(txProto, txHandler.recvTxs)
	go txHandler.broadcastTxs()
	return txHandler
}

// recevTx handles received transactions
func (handler *TxHandler) recvTxs(s inet.Stream) {
	log.Debug("recvTxs")
	var txs []*types.Transaction
	reader := msgio.NewReader(s)
	bytes, err := reader.ReadMsg()
	log.Debugf("recv msg: %v", bytes)
	err = handler.host.Decode(bytes, &txs)
	if err != nil {
		log.WithError(err).Error("recvTxs Decode Error!")
		return
	}

	log.Debugf("recv txs: %v", txs)
	// mark known tx
	handler.host.MarkTxs(s.Conn().RemotePeer(), txs)

	if handler.pls.Config().IsOperator {
		handler.pls.TxPool().AddRemotes(txs)
	} else {
		handler.pls.TxPool().NewTxsChannel() <- txs
	}
}

// broadcastTxs broadcast received valid tx
func (handler *TxHandler) broadcastTxs() {
	log.Debug("broadcastTxs")
	// loop peers to send txs
	txsCh := handler.pls.TxPool().NewTxsChannel()
	for {
		select {
		case txs := <-txsCh:
			var txset = make(map[ipeer.ID]types.Transactions)

			// Broadcast transactions to a batch of peers not knowing about it
			for _, tx := range txs {
				peerids := handler.host.PeerIDsWithoutTx(tx.Hash())
				for _, peerid := range peerids {
					txset[peerid] = append(txset[peerid], tx)
				}
				log.Debug("Broadcast transaction", "hash", tx.Hash(), "recipients", len(peerids))
			}
			for peerid, txs := range txset {
				handler.host.SendMsg(txProto, peerid, txs)
			}
		}
	}
}
