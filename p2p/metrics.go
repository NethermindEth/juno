package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type P2PMetrics struct {
	P2PPeerCount       *prometheus.GaugeVec
	PSMsgDeliver       *prometheus.CounterVec
	PSMsgUndeliverable *prometheus.CounterVec
	PSMsgValidate      *prometheus.CounterVec
	PSMsgDuplicate     *prometheus.CounterVec
	PSMsgReject        *prometheus.CounterVec
	PSTopicsActive     *prometheus.GaugeVec
	PSTopicsPrune      *prometheus.CounterVec
	PSTopicsGraft      *prometheus.CounterVec
	PSThrottlePeer     *prometheus.CounterVec
	PSRPCRecv          *prometheus.CounterVec
	PSRPCSubRecv       prometheus.Counter
	PSRPCPubRecv       *prometheus.CounterVec
	PSRPCDrop          *prometheus.CounterVec
	PSRPCSubDrop       prometheus.Counter
	PSRPCPubDrop       *prometheus.CounterVec
	PSRPCSent          *prometheus.CounterVec
	PSRPCSubSent       prometheus.Counter
	PSRPCPubSent       *prometheus.CounterVec
}

func NewP2PMetrics() *P2PMetrics {
	return &P2PMetrics{
		P2PPeerCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "p2p_peer_count",
			Help: "The number of connected libp2p peers by agent string",
		}, []string{"agent"}),
		PSMsgDeliver: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_deliver_total",
			Help: "The number of messages received for delivery",
		}, []string{"topic"}),
		PSMsgUndeliverable: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_undeliverable_total",
			Help: "The number of messages received which weren't able to be delivered",
		}, []string{"topic"}),
		PSMsgValidate: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_validate_total",
			Help: "The number of messages received for validation",
		}, []string{"topic"}),
		PSMsgDuplicate: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_duplicate_total",
			Help: "The number of duplicate messages sent",
		}, []string{"topic"}),
		PSMsgReject: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_reject_total",
			Help: "The number of messages rejected",
		}, []string{"topic", "reason"}),
		PSTopicsActive: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "p2p_pubsub_topic_active",
			Help: "The topics that the peer is participating in gossipsub",
		}, []string{"topic"}),
		PSTopicsPrune: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_prune_total",
			Help: "The number of prune messages sent by the peer",
		}, []string{"topic"}),
		PSTopicsGraft: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_graft_total",
			Help: "The number of graft messages sent by the peer",
		}, []string{"topic"}),
		PSThrottlePeer: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_throttle_total",
			Help: "The number of times the peer has been throttled",
		}, []string{"topic"}),
		PSRPCRecv: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_recv_total",
			Help: "The number of messages received via rpc for a particular control message",
		}, []string{"control_message"}),
		PSRPCSubRecv: promauto.NewCounter(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_recv_sub_total",
			Help: "The number of subscription messages received via rpc",
		}),
		PSRPCPubRecv: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_recv_pub_total",
			Help: "The number of publish messages received via rpc",
		}, []string{"topic"}),
		PSRPCDrop: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_drop_total",
			Help: "The number of messages dropped via rpc for a particular control message",
		}, []string{"control_message"}),
		PSRPCSubDrop: promauto.NewCounter(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_drop_sub_total",
			Help: "The number of subscription messages dropped via rpc",
		}),
		PSRPCPubDrop: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_drop_pub_total",
			Help: "The number of publish messages dropped via rpc",
		}, []string{"topic"}),
		PSRPCSent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_sent_total",
			Help: "The number of messages sent via rpc for a particular control message",
		}, []string{"control_message"}),
		PSRPCSubSent: promauto.NewCounter(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_sent_sub_total",
			Help: "The number of subscription messages sent via rpc",
		}),
		PSRPCPubSent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_sent_pub_total",
			Help: "The number of publish messages sent via rpc",
		}, []string{"topic"}),
	}
}
