package p2phandlers

import (
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/plasma"
	// host "github.com/libp2p/go-libp2p-host"
	// "bufio"
	inet "github.com/libp2p/go-libp2p-net"
	msgio "github.com/libp2p/go-msgio"
	log "github.com/sirupsen/logrus"
)

const (
	blockProto = "block/0.0.1"
)

// BlockHandler is a container for tx related handlers
type BlockHandler struct {
	host Host
	pls  *plasma.Plasma
}

func NewBlockHandler(host Host, pls *plasma.Plasma) *BlockHandler {
	blockHandler := &BlockHandler{host: host, pls: pls}
	host.SetStreamHandler(blockProto, blockHandler.recvBlock)
	go blockHandler.broadcastBlock()
	return blockHandler
}

// recevBlock handles received transactions
func (handler *BlockHandler) recvBlock(s inet.Stream) {
	log.Debug("recvBlock")
	var block *types.Block
	reader := msgio.NewReader(s)
	bytes, err := reader.ReadMsg()
	log.Debugf("recv msg: %v", bytes)
	err = handler.host.Decode(bytes, &block)
	if err != nil {
		log.WithError(err).Error("recvBlocks Decode Error!")
		return
	}

	log.Debugf("recv block: %v", block)
	// mark known block
	handler.host.MarkBlock(s.Conn().RemotePeer(), block)
	// write block to chain, cache the block if it's not sequential.
}

// broadcastBlocks broadcast received valid tx
func (handler *BlockHandler) broadcastBlock() {
	log.Debug("broadcastBlocks")
	// loop peers to send txs
	newBlockCh := handler.pls.GetNewBlockChannel()
	for {
		select {
		case newBlock := <-newBlockCh:

			// Broadcast block to a batch of peers not knowing about it
			for _, peerid := range handler.host.PeerIDsWithoutBlock(newBlock.Hash()) {
				handler.host.SendMsg(blockProto, peerid, newBlock)
			}
		}
	}
}