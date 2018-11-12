package client

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httputil"

	"github.com/ethereum/go-ethereum/accounts/keystore"

	"github.com/ethereum/go-ethereum/common"
	"github.com/icstglobal/plasma/core/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var from []byte
var blockNum uint64
var txIdx uint32
var outIdx byte
var to []byte
var amount uint64
var fee uint64

func init() {
	TransferCmd.Flags().Uint64Var(&blockNum, "blocknum", 0, "the number of block which contains the utxo")
	TransferCmd.Flags().Uint32Var(&txIdx, "txidx", 0, "the index of tx which contains the utxo")
	TransferCmd.Flags().Uint8Var(&outIdx, "outidx", 0, "the utxo out index inside the tx")
	TransferCmd.Flags().BytesHexVar(&from, "from", nil, "the account address to transfer from")
	TransferCmd.Flags().BytesHexVar(&to, "to", nil, "the account address to transfer to")
	TransferCmd.Flags().Uint64Var(&amount, "amount", 0, "the amount to transfer")
	TransferCmd.Flags().Uint64Var(&fee, "fee", 0, "the tx fee to pay")

	TransferCmd.MarkFlagRequired("blocknum")
	TransferCmd.MarkFlagRequired("txidx")
	TransferCmd.MarkFlagRequired("outidx")
	TransferCmd.MarkFlagRequired("from")
	TransferCmd.MarkFlagRequired("to")
	TransferCmd.MarkFlagRequired("amount")
	TransferCmd.MarkFlagRequired("fee")
}

// TransferCmd is a sub command to transfer tokens from A to B
var TransferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "transfer tokens",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		chainID := getPlasmaChainID()
		key, err := getPrivateKey()
		if err != nil {
			log.Error("can not get private key")
			return
		}
		signer := types.NewEIP155Signer(chainID)
		tx := makeTx()
		tx, err = types.SignTx(tx, signer, key, key)
		if err != nil {
			log.WithError(err).Error("sign tx failed")
			return
		}
		send(tx)
	},
}

func makeTx() *types.Transaction {
	in1 := &types.UTXO{UTXOID: types.UTXOID{BlockNum: blockNum, TxIndex: txIdx, OutIndex: outIdx}, TxOut: types.TxOut{Owner: common.BytesToAddress(from)}}
	in2 := types.NewNullUTXO()
	out1 := &types.TxOut{Owner: common.BytesToAddress(to), Amount: new(big.Int).SetUint64(amount)}
	out2 := types.NewNullTxOut()
	tx := types.NewTransaction(in1, in2, out1, out2, new(big.Int).SetUint64(fee))
	return tx
}

func getPrivateKey() (*ecdsa.PrivateKey, error) {
	keyfile := viper.GetString("account.keyfile")
	buf, err := ioutil.ReadFile(keyfile)
	if err != nil {
		log.WithError(err).Error("failed to load key file")
		return nil, err
	}
	auth := viper.GetString("account.pwd")
	key, err := keystore.DecryptKey(buf, auth)
	if err != nil {
		log.WithError(err).Error("failed to decrypted key file")
		return nil, err
	}

	return key.PrivateKey, nil
}

func getPlasmaChainID() *big.Int {
	chainID := viper.GetInt64("plasma.chainID")
	return new(big.Int).SetInt64(chainID)
}

func send(tx *types.Transaction) {
	buf, err := tx.MarshalJSON()
	if err != nil {
		log.WithError(err).Error("marshal tx as json failed")
		return
	}

	body := bytes.NewReader(buf)
	url := fmt.Sprintf("%s%s", viper.GetString("httpserver.url"), "tx/new")
	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		log.WithError(err).WithField("url", url).Error("failed to send to remote http server")
	}
	data, _ := httputil.DumpResponse(resp, true)
	log.WithField("response", string(data)).Info("get response from http server")
}

func initConfig(cmd *cobra.Command) error {
	cfgFile := cmd.Flag("config").Value.String()
	// if confFile is not empty string, then it should has been loaded by the root command
	if cfgFile == "" {
		fmt.Println("config file not set, seaching for config files in current and $HOME dir")

		// Search config in home directory with name ".plasma" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigName(".client")

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		} else {
			fmt.Println("loading config failed:", err)
			return err
		}
	}

	// If a config file is found, read it in.
	slvl := viper.GetString("logger.level")
	if len(slvl) > 0 {
		if lvl, err := log.ParseLevel(slvl); err == nil {
			log.SetLevel(lvl)
			log.WithField("logger.level", slvl).Debug("change log level")
		} else {
			log.WithError(err).WithField("logger.level", slvl).Warn("unknown logger level")
		}
	}

	return nil
}
