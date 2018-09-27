package httphandlers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/go-icst/chain"
	"github.com/icstglobal/plasma/plasma"
	log "github.com/sirupsen/logrus"
	"math/big"
	"net/http"
)

// ActionHandler is a container for tx related handlers
type ActionHandler struct {
}

type DepositRequest struct {
	owner  string
	amount int
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (req *DepositRequest) UnmarshalJSON(input []byte) error {
	if err := json.Unmarshal(input, &req); err != nil {
		return err
	}
	return nil
}

// deposit icst to root chain and create block in child chain
func (action ActionHandler) Deposit(plasma *plasma.Plasma) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Debug("do deposit")
		defer req.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(req.Body)
		log.WithFields(log.Fields{
			"url":         req.URL,
			"method":      req.Method,
			"request.boy": buf.String(),
		}).Info("handle http request")

		depositReq := new(DepositRequest)
		if err := depositReq.UnmarshalJSON(buf.Bytes()); err != nil {
			msg := "request body is not a valid deposit request"
			log.WithError(err).WithField("request.body", buf.String()).Error(msg)

			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		var blc chain.Chain
		blc, err := chain.Get(chain.Eth)
		if err != nil {
			msg := "can not find eth chain!"
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		from, err := hex.DecodeString(depositReq.owner)
		blc.Deposit(context.Background(), from, big.NewInt(int64(depositReq.amount)))

		// call rootchain contract deposit

	}
}
