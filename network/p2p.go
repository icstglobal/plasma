package network

import (
	"context"
	"encoding/base64"
	"fmt"
	// "io/ioutil"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/network/p2phandlers"
	"github.com/icstglobal/plasma/plasma"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	inet "github.com/libp2p/go-libp2p-net"
	ipeer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	msgio "github.com/libp2p/go-msgio"
	ma "github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/fatih/set.v0"
)

// LocalHost serves P2P requests
type P2PLocalHost struct {
	Port int
	// Plasma *plasma.Plasma
	// Chain  *core.BlockChain
	host.Host
	RemotePeerCaches map[ipeer.ID]*RemotePeerCache
}

var (
	localhost      *P2PLocalHost
	maxKnownTxs    = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks = 1000  // Maximum transactions hashes to keep in the known list (prevent DOS)
)

func bootstrapConnect(ctx context.Context, host host.Host) error {
	bootstrapPeers := viper.GetStringSlice("plasma.bootstrapPeers")
	if bootstrapPeers == nil {
		log.Debug("there is no bootstrapPeers.")
		return nil
	}
	log.Printf("bootstrapConnect...%#v", bootstrapPeers)
	// Let's connect to the bootstrap nodes first. They will tell us about the other nodes in the network.
	for _, peerAddr := range bootstrapPeers {
		addr, _ := ma.NewMultiaddr(peerAddr)
		peerinfo, _ := pstore.InfoFromP2pAddr(addr)

		if err := host.Connect(ctx, *peerinfo); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Connection established with bootstrap node: ", *peerinfo)
		}
	}

	log.Printf("bootstrapConnectSuccess")
	return nil
}

// makeRoutedHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeRoutedHost(listenPort int) (host.Host, error) {
	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs

	privKeyStr := viper.GetString("plasma.privKey")
	privKeyByte, err := base64.StdEncoding.DecodeString(privKeyStr)
	priv, err := crypto.UnmarshalPrivateKey(privKeyByte)

	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		libp2p.Identity(priv),
	}

	basicHost, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	// Make the DHT
	dht := dht.NewDHT(ctx, basicHost, nil)

	// Make the routed host
	routedHost := rhost.Wrap(basicHost, dht)
	routedHost.Network().SetConnHandler(connectHandler)

	// connect to the chosen ipfs nodes
	err = bootstrapConnect(ctx, routedHost)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := basicHost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	log.Printf("Now run \"./echo -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)

	return routedHost, nil
}

func startP2P(plasma *plasma.Plasma) (*P2PLocalHost, error) {
	port := viper.GetInt("p2pserver.port")

	localhost = &P2PLocalHost{Port: port, RemotePeerCaches: make(map[ipeer.ID]*RemotePeerCache)}
	// Make a host that listens on the given multiaddress
	ha, err := makeRoutedHost(port)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	localhost.Host = ha

	p2phandlers.NewTxHandler(localhost, plasma)
	p2phandlers.NewBlockHandler(localhost, plasma)

	return localhost, nil
}

func connectHandler(conn inet.Conn) {
	peerid := conn.RemotePeer()
	log.WithField("peerid", peerid).Debug("connectHandler")
	localhost.RemotePeerCaches[peerid] = &RemotePeerCache{peerid: peerid, KnownTxs: set.New(set.NonThreadSafe), KnownBlock: set.New(set.NonThreadSafe)}
}

// RemotePeerCache
type RemotePeerCache struct {
	peerid     ipeer.ID
	KnownTxs   set.Interface
	KnownBlock set.Interface
}

func (localhost P2PLocalHost) SendMsg(proto string, peerid ipeer.ID, data interface{}) error {
	log.WithField("proto", proto).Debugf("SendMsg peer:%v", peerid)
	bytes, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}
	s, err := localhost.NewStream(context.Background(), peerid, protocol.ID(proto))
	if err != nil {
		log.Fatalln(err)
	}
	writer := msgio.NewWriter(s)
	return writer.WriteMsg(bytes)
}

func (localhost P2PLocalHost) Response(s inet.Stream, data interface{}) error {
	log.Debug("Response")
	bytes, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}
	writer := msgio.NewWriter(s)
	return writer.WriteMsg(bytes)
}

func (localhost P2PLocalHost) Request(proto string, peerid ipeer.ID, data interface{}) ([]byte, error) {
	log.WithField("proto", proto).Debugf("Request peer:%v", peerid)
	bytes, err := rlp.EncodeToBytes(data)
	if err != nil {
		return nil, err
	}
	s, err := localhost.NewStream(context.Background(), peerid, protocol.ID(proto))
	if err != nil {
		log.Fatalln(err)
	}
	writer := msgio.NewWriter(s)
	err = writer.WriteMsg(bytes)

	if err != nil {
		log.WithError(err).Error("Request Error")
		return nil, err
	}
	reader := msgio.NewReader(s)
	return reader.ReadMsg()
}

func (localhost P2PLocalHost) Decode(data []byte, out interface{}) error {
	return rlp.DecodeBytes(data, out)
}

func (localhost P2PLocalHost) PeerIDsWithoutTx(hash common.Hash) []ipeer.ID {
	log.WithField("hash", hash).Debug("PeerIDsWithoutTx")
	peerIds := make([]ipeer.ID, 0)
	for _, peercache := range localhost.RemotePeerCaches {
		if !peercache.KnownTxs.Has(hash) {
			peerIds = append(peerIds, peercache.peerid)
		}
	}
	log.WithField("peerIds", peerIds).Debug("PeerIDsWithoutTx")
	return peerIds
}

func (localhost P2PLocalHost) PeerIDsWithoutBlock(hash common.Hash) []ipeer.ID {
	log.WithField("hash", hash).Debug("PeerIDsWithoutBlock")
	peerIds := make([]ipeer.ID, 0)
	for _, peercache := range localhost.RemotePeerCaches {
		if !peercache.KnownBlock.Has(hash) {
			peerIds = append(peerIds, peercache.peerid)
		}
	}
	log.WithField("peerIds", peerIds).Debug("PeerIDsWithoutBlock")
	return peerIds
}

func (localhost P2PLocalHost) PeerIDs() []ipeer.ID {
	peerIds := make([]ipeer.ID, 0)
	for _, peercache := range localhost.RemotePeerCaches {
		peerIds = append(peerIds, peercache.peerid)
	}
	// log.WithField("peerIds", peerIds).Debug("PeerIDs")
	return peerIds
}

func (localhost P2PLocalHost) MarkTxs(peerid ipeer.ID, txs types.Transactions) {
	log.WithField("peerid", peerid).Debug("MarkTxs")
	peercache, ok := localhost.RemotePeerCaches[peerid]
	if !ok {
		return
	}
	for _, tx := range txs {
		for peercache.KnownTxs.Size() >= maxKnownTxs {
			peercache.KnownTxs.Pop()
		}
		peercache.KnownTxs.Add(tx.Hash())
	}
}

func (localhost P2PLocalHost) MarkBlock(peerid ipeer.ID, block *types.Block) {
	log.WithField("block hash", block.Hash().Hex()).Debug("MarkBlock")
	peercache, ok := localhost.RemotePeerCaches[peerid]
	if !ok {
		return
	}
	for peercache.KnownBlock.Size() >= maxKnownBlocks {
		peercache.KnownBlock.Pop()
	}
	peercache.KnownBlock.Add(block.Hash())
}
