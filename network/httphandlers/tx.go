package httphandlers

import (
	"bytes"
	"net/http"

	// "github.com/icstglobal/plasma/core"
	"github.com/icstglobal/plasma/core/types"
	"github.com/icstglobal/plasma/plasma"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
)

// TxHandler is a container for tx related handlers
type TxHandler struct {
}

// New handles transaction creation request
func (th TxHandler) New(plasma *plasma.Plasma) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(req.Body)
		log.WithFields(log.Fields{
			"url":         req.URL,
			"method":      req.Method,
			"request.boy": buf.String(),
		}).Info("handle http request")

		tx := new(types.Transaction)
		if err := tx.UnmarshalJSON(buf.Bytes()); err != nil {
			msg := "request body is not a valid tx json"
			log.WithError(err).WithField("request.body", buf.String()).Error(msg)

			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		if err := plasma.TxPool().AddLocal(tx); err != nil {
			log.WithError(err).WithField("tx", tx).Warn("can not add tx to pool")

			http.Error(w, errors.NewBadRequest(err, "can not add tx to pool").Error(), http.StatusBadRequest)
			return
		}
	}

}

// Sign handles transaction signature
func (th TxHandler) Sign() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		//TODO:
		log.WithField("url", req.URL).WithField("method", req.Method).Debug("handle http request")
	}
}
