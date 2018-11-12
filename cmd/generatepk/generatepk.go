package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

func main() {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		fmt.Errorf("%v\n", err)
		return
	}

	privkeyb, err := priv.Bytes()
	if err != nil {
		fmt.Errorf("%v\n", err)
		return
	}
	fmt.Printf("privkey: %v\n", base64.StdEncoding.EncodeToString(privkeyb))
	peerid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		fmt.Errorf("%v\n", err)
		return
	}
	fmt.Printf("peerid: %v\n", peerid.Pretty())

}
