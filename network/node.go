package network

import (
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

//Node is a P2P network member, it can acts both as client and server
type Node struct {
	s *Server
}

//Start the node
func (n *Node) Start() error {
	proto := viper.GetString("listen.proto")
	port := viper.GetInt("listen.port")
	log.WithFields(log.Fields{"proto": proto, "port": port}).Debug("try to start node")
	s, err := Listen(proto, port)
	if err != nil {
		return errors.Annotate(err, "failed to start node")
	}
	n.s = s
	//start accepting rpc connections
	n.s.Register(calls())
	go n.s.RPC()
	return nil
}

func calls() map[string]interface{} {
	calls := make(map[string]interface{})
	//TODO: use real calls
	return calls
}
