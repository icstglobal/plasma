package p2phandlers

import (
	"math/big"
	"time"

	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/plasma"
	// host "github.com/libp2p/go-libp2p-host"
	// "bufio"
	inet "github.com/libp2p/go-libp2p-net"
	msgio "github.com/libp2p/go-msgio"
	log "github.com/sirupsen/logrus"
)

const (
	blockProto    = "block/0.0.1"
	getBlockProto = "getblock/0.0.1"
)

// BlockHandler is a container for tx related handlers
type BlockHandler struct {
	host       Host
	pls        *plasma.Plasma
	newBlockCh chan *types.Block
}

func NewBlockHandler(host Host, pls *plasma.Plasma) *BlockHandler {
	blockHandler := &BlockHandler{host: host, pls: pls, newBlockCh: make(chan *types.Block, 10)}
	host.SetStreamHandler(blockProto, blockHandler.recvBlock)
	host.SetStreamHandler(getBlockProto, blockHandler.getBlock)
	blockHandler.pls.SubscribeNewBlockCh(blockHandler.newBlockCh)
	go blockHandler.broadcastBlock()
	if !pls.Config().IsOperator {
		go blockHandler.downloadBlocks()
	}
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

	log.Debugf("recv block: %v TxHash: %v Number:%v", block, block.Header().TxHash.Hex(), block.Header().Number.Int64())
	// mark known block
	handler.host.MarkBlock(s.Conn().RemotePeer(), block)
	// write block to chain, cache the block if it's not sequential.
	err = handler.pls.WriteBlock(block)
	if err != nil {
		log.WithError(err).Error("recvBlock WriteBlock Error")
		return
	}
	// transmit to other peers
	handler.newBlockCh <- block
}

// broadcastBlocks broadcast received valid tx
func (handler *BlockHandler) broadcastBlock() {
	log.Debug("broadcastBlocks")
	// loop peers to send txs
	for {
		select {
		case newBlock := <-handler.newBlockCh:

			// Broadcast block to a batch of peers not knowing about it
			for _, peerid := range handler.host.PeerIDsWithoutBlock(newBlock.Hash()) {
				handler.host.SendMsg(blockProto, peerid, newBlock)
			}
		}
	}
}

// download block from remote peer
func (handler *BlockHandler) downloadBlocks() {
	log.Debug("downloadBlocks")

	var currentIndex int64 = 0

	for {
		blockNums, err := handler.pls.RootChain().RootChainBlockNums()
		if currentIndex >= blockNums.Int64() {
			time.Sleep(time.Second * 10)
		}
		if err != nil {
			log.WithError(err).Error("RootChainBlockNums Error")
			return
		}

		currentBlockNum := handler.pls.BlockChain().CurrentBlock().Number().Int64()

		blockNum, err := handler.pls.RootChain().GetRootChainBlockNumByIndex(currentIndex)
		if err != nil {
			log.WithError(err).Error("GetRootChainBlockNumByIndex Error")
			return
		}
		currentIndex++
		if currentBlockNum >= blockNum.Int64() {
			continue
		}
		log.Debugf("blockNum: %v currentBlockNum: %v", blockNum.Int64(), currentBlockNum)
		if blockNum.Int64() <= currentBlockNum {
			continue
		}
		peerids := handler.host.PeerIDs()
		if len(peerids) == 0 {
			continue
		}
		targetPeerId := peerids[0]
		resp, err := handler.host.Request(getBlockProto, targetPeerId, big.NewInt(blockNum.Int64()))
		var block *types.Block
		log.Debugf("response msg: %v", resp)
		err = handler.host.Decode(resp, &block)
		if err != nil {
			log.WithError(err).Error("recvBlocks Decode Error!")
			return
		}
		handler.host.MarkBlock(targetPeerId, block)
		// write block to chain, cache the block if it's not sequential.
		err = handler.pls.WriteBlock(block)
		if err != nil {
			log.WithError(err).Error("recvBlock WriteBlock Error")
			return
		}
		time.Sleep(time.Second * 2)
	}
}

// getBlock get block from chain
func (handler *BlockHandler) getBlock(s inet.Stream) {
	reader := msgio.NewReader(s)
	bytes, err := reader.ReadMsg()
	if err != nil {
		log.WithError(err).Error("getBlock ReadMsg Error!")
		return
	}
	blockNum := new(big.Int)
	err = handler.host.Decode(bytes, blockNum)
	if err != nil {
		log.WithError(err).Error("getBlock Decode Error!")
		return
	}
	log.Debug("getBlock blockNum:", blockNum)
	block := handler.pls.BlockChain().GetBlockByNumber(blockNum.Uint64())
	if block == nil {
		return
	}
	err = handler.host.Response(s, block)
	if err != nil {
		log.WithError(err).Error("getBlock Response Error!")
		return
	}
	log.Debug("getBlock done")
}
