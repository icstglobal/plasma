package httphandlers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/icstglobal/go-icst/chain"
	log "github.com/sirupsen/logrus"
	"math/big"
	"net/http"
)

// ActionHandler is a container for tx related handlers
type ActionHandler struct {
}

type DepositRequest struct {
	Owner  string `json:"owner"`
	Amount int    `json:"amount"`
}

type DepositResponse struct {
	TxHash string `json:"txhash"`
}

// deposit icst to root chain and create block in child chain
func (action ActionHandler) Deposit() http.HandlerFunc {
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
		if err := json.Unmarshal(buf.Bytes(), depositReq); err != nil {
			log.Error(err)
			msg := "request body is not a valid deposit request"
			log.WithError(err).WithField("request.body", buf.String()).Error(msg)

			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		from, err := hex.DecodeString(depositReq.Owner)
		if err != nil {
			log.Error(err)
			msg := "can not decode depositReq.owner"
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		var blc chain.Chain
		blc, err = chain.Get(chain.Eth)
		if err != nil {
			msg := "can not find eth chain!"
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// call rootchain contract deposit
		tx, err := blc.Deposit(context.Background(), from, big.NewInt(int64(depositReq.Amount)))
		if err != nil {
			msg := "blc.Deposit Error!"
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		txHex := hexutil.Encode(tx.Hash())
		log.Debugf("txHash:%v", txHex)

		resp := &DepositResponse{
			TxHash: txHex,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type AfterSignRequest struct {
	Owner  string `json:"owner"`
	Amount int    `json:"amount"`
	Sig    string `json:"sig"`
}
type AfterSignResponse struct {
	Res bool `json:"res"`
}

func (action ActionHandler) AfterSign() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Debug("do aftersign")
		defer req.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(req.Body)
		log.WithFields(log.Fields{
			"url":         req.URL,
			"method":      req.Method,
			"request.boy": buf.String(),
		}).Info("handle http request")

		_req := new(AfterSignRequest)
		if err := json.Unmarshal(buf.Bytes(), _req); err != nil {
			msg := "request body is not a valid aftersign request"
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
		from, err := hex.DecodeString(_req.Owner)
		if err != nil {
			msg := "can not decode depositReq.owner"
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		// call rootchain contract deposit
		tx, err := blc.Deposit(context.Background(), from, big.NewInt(int64(_req.Amount)))
		if err != nil {
			msg := "blc.Deposit Error!"
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		log.Debug(tx)
		sigBytes, err := hexutil.Decode(_req.Sig)
		if err != nil {
			msg := "sig decode Error!"
			log.Error(err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		log.Debugf("sig: %v %v %v", _req.Sig, sigBytes, _req.Amount)
		err = blc.ConfirmTrans(context.Background(), tx, sigBytes)
		if err != nil {
			log.Error(err)
			msg := "blc.ConfirmTrans error!"
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		resp := &AfterSignResponse{
			Res: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	}
}
