package httphandlers

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// TxHandler is a container for tx related handlers
type TxHandler struct {
}

// New handles transaction creation request
func (th TxHandler) New() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		//TODO:
		log.WithField("url", req.URL).Debug("handle http request")
	}

}

// Sign handles transaction signature
func (th TxHandler) Sign() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		//TODO:
		log.WithField("url", req.URL).Debug("handle http request")
	}

}
