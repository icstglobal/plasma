package network

import (
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
)

//Server handle incoming connections
type Server struct {
	l net.Listener
}

//Listen on a port and return the Server instance listening
func Listen(proto string, port int) (*Server, error) {
	l, err := net.Listen(proto, fmt.Sprintf(":%v", port))
	if err != nil {
		return nil, errors.Annotatef(err, "failed to listen on port %v", port)
	}
	log.WithField("port", port).Info("server listening for connections")
	return &Server{l: l}, nil
}

//RPC handles rpc connection
func (s *Server) RPC() {
	for {
		if conn, err := s.l.Accept(); err != nil {
			log.WithError(err).Error("accept connection failed")
		} else {
			log.Info("new connection established")
			go rpc.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	}
}

//Register rpc calls
func (s *Server) Register(methods map[string]interface{}) {
	for name, method := range methods {
		rpc.RegisterName(name, method)

		log.WithFields(log.Fields{"name": name, "method": method}).Info("service call registered")
	}
}
