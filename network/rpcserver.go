package network

import (
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
)

// RPCConfig holds the configuraiton for rpc service
type RPCConfig struct {
	Proto   string // network: tcp, udp, unix domain ...
	Port    int
	Methods map[string]interface{}
}

//RPCServer handle incoming connections
type RPCServer struct {
	l net.Listener

	RPCConfig
}

//ServeRPC on a port and return the RPCServer instance listening
func ServeRPC(c RPCConfig) (*RPCServer, error) {
	if len(c.Methods) == 0 {
		return nil, errors.New("no rpc methods found in config")
	}

	l, err := net.Listen(c.Proto, fmt.Sprintf(":%v", c.Port))
	if err != nil {
		return nil, errors.Annotatef(err, "failed to listen on port %v", c.Port)
	}

	s := &RPCServer{l: l}
	s.register(c.Methods)
	// start rpc server
	go s.rpc()

	return &RPCServer{l: l}, nil
}

//rpc handles rpc connection
func (s *RPCServer) rpc() {
	for {
		if conn, err := s.l.Accept(); err != nil {
			log.WithError(err).Error("accept connection failed")
		} else {
			log.Info("new rpc connection established")
			go rpc.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	}
}

//register rpc calls
func (s *RPCServer) register(methods map[string]interface{}) {
	for name, method := range methods {
		rpc.RegisterName(name, method)

		log.WithFields(log.Fields{"name": name, "method": method}).Info("service call registered")
	}
}
