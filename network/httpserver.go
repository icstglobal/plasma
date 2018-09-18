package network

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/icstglobal/plasma/network/httphandlers"
	"github.com/juju/errors"
)

// HTTPServer serves HTTP requests
type HTTPServer struct {
	s *http.Server
}

// ServeHTTP start a http server
func ServeHTTP(port int) (*HTTPServer, error) {
	s := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: initMux(),
	}
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return nil, errors.Annotatef(err, "tcp listener for http server failed, addr:%v", s.Addr)
	}
	// start http handling
	go s.Serve(l)

	return &HTTPServer{s: s}, nil
}

// Shutdown stops http processing
func (hs *HTTPServer) Shutdown(ctx context.Context) {
	hs.s.Shutdown(ctx)
}

func initMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/tx/new", httphandlers.Tx.New())
	mux.HandleFunc("/tx/sign", httphandlers.Tx.Sign())
	return mux
}
