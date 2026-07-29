package node

import (
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
)

// Config is the top-level juno configuration.
type Config struct {
	LogLevel                 string           `mapstructure:"log-level"`
	LogJSON                  bool             `mapstructure:"log-json"`
	HTTP                     bool             `mapstructure:"http"`
	HTTPHost                 string           `mapstructure:"http-host"`
	HTTPPort                 uint16           `mapstructure:"http-port"`
	RPCCorsEnable            bool             `mapstructure:"rpc-cors-enable"`
	Websocket                bool             `mapstructure:"ws"`
	WebsocketHost            string           `mapstructure:"ws-host"`
	WebsocketPort            uint16           `mapstructure:"ws-port"`
	GRPC                     bool             `mapstructure:"grpc"`
	GRPCHost                 string           `mapstructure:"grpc-host"`
	GRPCPort                 uint16           `mapstructure:"grpc-port"`
	DatabasePath             string           `mapstructure:"db-path"`
	Network                  networks.Network `mapstructure:"network"`
	EthNode                  string           `mapstructure:"eth-node"`
	DisableL1Verification    bool             `mapstructure:"disable-l1-verification"`
	Pprof                    bool             `mapstructure:"pprof"`
	PprofHost                string           `mapstructure:"pprof-host"`
	PprofPort                uint16           `mapstructure:"pprof-port"`
	Colour                   bool             `mapstructure:"colour"`
	PreConfirmedPollInterval time.Duration    `mapstructure:"preconfirmed-poll-interval"`
	RemoteDB                 string           `mapstructure:"remote-db"`
	VersionedConstantsFile   string           `mapstructure:"versioned-constants-file"`

	Sequencer      bool   `mapstructure:"seq-enable"`
	SeqBlockTime   uint   `mapstructure:"seq-block-time"`
	SeqGenesisFile string `mapstructure:"seq-genesis-file"`
	SeqDisableFees bool   `mapstructure:"seq-disable-fees"`

	Metrics     bool   `mapstructure:"metrics"`
	MetricsHost string `mapstructure:"metrics-host"`
	MetricsPort uint16 `mapstructure:"metrics-port"`

	P2P           bool   `mapstructure:"p2p"`
	P2PAddr       string `mapstructure:"p2p-addr"`
	P2PPublicAddr string `mapstructure:"p2p-public-addr"`
	P2PPeers      string `mapstructure:"p2p-peers"`
	P2PFeederNode bool   `mapstructure:"p2p-feeder-node"`
	P2PPrivateKey string `mapstructure:"p2p-private-key"`

	MaxVMs                  uint   `mapstructure:"max-vms"`
	MaxVMQueue              uint   `mapstructure:"max-vm-queue"`
	RPCMaxBlockScan         uint   `mapstructure:"rpc-max-block-scan"`
	RPCCallMaxSteps         uint64 `mapstructure:"rpc-call-max-steps"`
	RPCCallMaxGas           uint64 `mapstructure:"rpc-call-max-gas"`
	ReadinessBlockTolerance uint   `mapstructure:"readiness-block-tolerance"`

	SubmittedTransactionsCacheSize     uint          `mapstructure:"submitted-transactions-cache-size"`
	SubmittedTransactionsCacheEntryTTL time.Duration `mapstructure:"submitted-transactions-cache-entry-ttl"` //nolint:lll // the mapstructure key cannot be split

	DBCacheSize             uint   `mapstructure:"db-cache-size"`
	DBMaxHandles            int    `mapstructure:"db-max-handles"`
	DBCompactionConcurrency string `mapstructure:"db-compaction-concurrency"`
	DBMemtableSize          uint   `mapstructure:"db-memtable-size"`
	DBMemtableCount         uint   `mapstructure:"db-memtable-count"`
	DBCompression           string `mapstructure:"db-compression"`

	GatewayAPIKey   string `mapstructure:"gw-api-key"`
	GatewayTimeouts string `mapstructure:"gw-timeouts"`

	PluginPath string `mapstructure:"plugin-path"`

	HTTPUpdateHost string `mapstructure:"http-update-host"`
	HTTPUpdatePort uint16 `mapstructure:"http-update-port"`

	ForbidRPCBatchRequests bool `mapstructure:"disable-rpc-batch-requests"`

	DisableReceivedTxnStream bool `mapstructure:"disable-received-txn-stream"`

	RPCRequestTimeout        time.Duration `mapstructure:"rpc-request-timeout"`
	RPCMaxConcurrentRequests uint          `mapstructure:"rpc-max-concurrent-requests"`
	RPCMaxRequestQueue       uint          `mapstructure:"rpc-max-request-queue"`

	// If MaxConcurrentCompilations or MaxCompilationQueue are not informed (Explicit is false)
	// the value is derived at startup. An informed 0 stays valid (no compilations / no queue).
	MaxConcurrentCompilations         uint64 `mapstructure:"max-concurrent-compilations"`
	MaxConcurrentCompilationsExplicit bool
	MaxCompilationQueue               uint64 `mapstructure:"max-compilation-queue"`
	MaxCompilationQueueExplicit       bool

	MaxCompilationMemory  uint `mapstructure:"max-compilation-memory"`   // megabytes
	NodeMemoryReserve     uint `mapstructure:"node-memory-reserve"`      // megabytes
	MaxCompilationCPUTime uint `mapstructure:"max-compilation-cpu-time"` // CPU seconds
	NewState              bool `mapstructure:"new-state"`

	// Prune is true when --prune-mode was provided (any value, including 0
	// or absent). Set in cmd PreRunE; not bound via mapstructure.
	Prune          bool
	RetainedBlocks uint64        `mapstructure:"prune-mode"`
	PruneMinAge    time.Duration `mapstructure:"prune-min-age"`
}
