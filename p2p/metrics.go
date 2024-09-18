package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	knownAgentVersions = []string{"juno", "papyrus", "pathfinder", "madara"}
	p2pPeerCount       = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "p2p_peer_count",
		Help: "The number of connected libp2p peers by agent string",
	}, []string{"agent"})
	psMsgDeliver = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_deliver_total",
		Help: "The number of messages received for delivery",
	}, []string{"topic"})
	psMsgUndeliverable = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_undeliverable_total",
		Help: "The number of messages received which weren't able to be delivered",
	}, []string{"topic"})
	psMsgValidate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_validate_total",
		Help: "The number of messages received for validation",
	}, []string{"topic"})
	psMsgDuplicate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_duplicate_total",
		Help: "The number of duplicate messages sent",
	}, []string{"topic"})
	psMsgReject = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_reject_total",
		Help: "The number of messages rejected",
	}, []string{"topic", "reason"})
	psTopicsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "p2p_pubsub_topic_active",
		Help: "The topics that the peer is participating in gossipsub",
	}, []string{"topic"})
	psTopicsPrune = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_prune_total",
		Help: "The number of prune messages sent by the peer",
	}, []string{"topic"})
	psTopicsGraft = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_graft_total",
		Help: "The number of graft messages sent by the peer",
	}, []string{"topic"})
	psThrottlePeer = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_throttle_total",
		Help: "The number of times the peer has been throttled",
	}, []string{"topic"})
	psRPCRecv = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_recv_total",
		Help: "The number of messages received via rpc for a particular control message",
	}, []string{"control_message"})
	psRPCSubRecv = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_recv_sub_total",
		Help: "The number of subscription messages received via rpc",
	})
	psRPCPubRecv = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_recv_pub_total",
		Help: "The number of publish messages received via rpc",
	}, []string{"topic"})
	psRPCDrop = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_drop_total",
		Help: "The number of messages dropped via rpc for a particular control message",
	}, []string{"control_message"})
	psRPCSubDrop = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_drop_sub_total",
		Help: "The number of subscription messages dropped via rpc",
	})
	psRPCPubDrop = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_drop_pub_total",
		Help: "The number of publish messages dropped via rpc",
	}, []string{"topic"})
	psRPCSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_sent_total",
		Help: "The number of messages sent via rpc for a particular control message",
	}, []string{"control_message"})
	psRPCSubSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_sent_sub_total",
		Help: "The number of subscription messages sent via rpc",
	})
	psRPCPubSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2p_pubsub_rpc_sent_pub_total",
		Help: "The number of publish messages sent via rpc",
	}, []string{"topic"})
)
