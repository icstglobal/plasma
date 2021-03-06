package plasma

import (
	// "encoding/hex"
	"os"
	"os/user"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/icstglobal/plasma/core"
)

// DefaultConfig contains default settings for use on the Ethereum main net.
var DefaultConfig = Config{
	SyncMode:      downloader.FastSync,
	NetworkId:     1,
	LightPeers:    100,
	DatabaseCache: 768,
	TrieCache:     256,
	TrieTimeout:   60 * time.Minute,

	TxPool:       core.DefaultTxPoolConfig,
	DataDir:      "",
	ChainUrl:     "",
	CxAddr:       "",
	KeystorePath: "",
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	DefaultConfig.Operbase = common.HexToAddress("85d6e595a3e64d3353b888bc49ee27f1b9f2a656")
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// The genesis block, which is inserted if the database is empty.
	// If nil, the Ethereum main net block is used.
	// Genesis *core.Genesis `toml:",omitempty"`

	// Protocol options
	NetworkId uint64 // Network ID to use for selecting peers to connect to
	SyncMode  downloader.SyncMode
	NoPruning bool

	// Light client options
	LightServ  int `toml:",omitempty"` // Maximum percentage of time allowed for serving LES requests
	LightPeers int `toml:",omitempty"` // Maximum number of LES client peers

	// Database options
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int
	TrieCache          int
	TrieTimeout        time.Duration

	// Mining-related options
	Operbase  common.Address `toml:",omitempty"`
	ExtraData []byte         `toml:",omitempty"`

	// Transaction pool options
	TxPool core.TxPoolConfig

	// Enables tracking of SHA3 preimages in the VM
	EnablePreimageRecording bool

	// Miscellaneous options
	DocRoot string `toml:"-"`
	// DataDir is the file system folder the node should use for any data storage
	// requirements. The configured data directory will not be directly shared with
	// registered services, instead those can use utility methods to create/access
	// databases or flat files. This enables ephemeral nodes which can fully reside
	// in memory.
	DataDir string

	ChainUrl     string
	CxAddr       string
	CxAbi        string
	OperPwd      string
	KeystorePath string
	IsOperator   bool
}

type configMarshaling struct {
	ExtraData hexutil.Bytes
}
