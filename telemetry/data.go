package telemetry

type Capability string

const (
	FullNode  Capability = "FullNode"
	Sequencer Capability = "Sequencer"
)

type Node struct {
	Name       string     `json:"name"`
	PeerID     string     `json:"peer_id"`
	Client     string     `json:"client"`
	Version    string     `json:"version"`
	Capability Capability `json:"capability"`
}

type Network struct {
	Network              string  `json:"Network"`
	StarknetVersion      string  `json:"Starknet_version"`
	JSONRPCVersion       string  `json:"JSON_RPC_version"`
	CurrentBlockHashL2   string  `json:"current_block_hash_l2"`
	CurrentBlockNumberL2 string  `json:"current_block_number_l2"`
	CurrentBlockHashL1   string  `json:"current_block_hash_l1"`
	CurrentBlockNumberL1 string  `json:"current_block_number_l1"`
	LatestBlockTime      int     `json:"latest_block_time"` // Expressed in ms
	AvgSyncTime          int     `json:"avg_sync_time"`     // Expressed in ms
	EstimateSync         float64 `json:"estimate_sync"`     // Expressed in ms
	TxNumber             int     `json:"tx_number"`
	EventNumber          int     `json:"event_number"`
	MessageNumber        int     `json:"message_number"`
	PeerCount            int     `json:"peer_count"`
}

type System struct {
	NodeUptime      float64 `json:"node_uptime"`
	OperatingSystem string  `json:"operating_system"`
	Memory          int     `json:"memory"`       // Expressed in MB
	MemoryUsage     float64 `json:"memory_usage"` // Expressed in %
	CPU             string  `json:"CPU"`
	CPUUsage        int     `json:"CPU_usage"`     // Expressed in %
	Storage         int     `json:"storage"`       // Expressed in MB
	StorageUsage    float64 `json:"storage_usage"` // Expressed in %
}

type Data struct {
	Node    Node    `json:"Node"`
	Network Network `json:"Network"`
	System  System  `json:"System"`
}
