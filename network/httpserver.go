package network

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/icstglobal/plasma/core"
	"github.com/icstglobal/plasma/network/httphandlers"
	"github.com/icstglobal/plasma/plasma"
	"github.com/juju/errors"
)

// HTTPServer serves HTTP requests
type HTTPServer struct {
	Port   int
	Plasma *plasma.Plasma
	Chain  *core.BlockChain

	s *http.Server
}

// Start is called to accept http requests
func (hs *HTTPServer) Start() error {
	s := &http.Server{
		Addr:    fmt.Sprintf(":%v", hs.Port),
		Handler: hs.initMux(),
	}
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return errors.Annotatef(err, "tcp listener for http server failed, addr:%v", s.Addr)
	}
	hs.s = s
	// start http handling
	go s.Serve(l)

	return nil
}

// Shutdown stops http processing
func (hs *HTTPServer) Shutdown(ctx context.Context) {
	hs.s.Shutdown(ctx)
}

func (hs *HTTPServer) initMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/tx/new", httphandlers.Tx.New(hs.Plasma))
	mux.HandleFunc("/tx/sign", httphandlers.Tx.Sign())
	return mux
}
