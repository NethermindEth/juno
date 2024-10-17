package p2p

import (
	"strings"

	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	_                  pubsub.RawTracer = (*gossipTracer)(nil)
	knownAgentVersions                  = []string{"juno", "papyrus", "pathfinder", "madara"}
)

// Metrics collection for gossipsub
type gossipTracer struct {
	host host.Host

	p2PPeerCount       *prometheus.GaugeVec
	psMsgDeliver       *prometheus.CounterVec
	psMsgUndeliverable *prometheus.CounterVec
	psMsgValidate      *prometheus.CounterVec
	psMsgDuplicate     *prometheus.CounterVec
	psMsgReject        *prometheus.CounterVec
	psTopicsActive     *prometheus.GaugeVec
	psTopicsPrune      *prometheus.CounterVec
	psTopicsGraft      *prometheus.CounterVec
	psThrottlePeer     *prometheus.CounterVec
	psRPCRecv          *prometheus.CounterVec
	psRPCSubRecv       prometheus.Counter
	psRPCPubRecv       *prometheus.CounterVec
	psRPCDrop          *prometheus.CounterVec
	psRPCSubDrop       prometheus.Counter
	psRPCPubDrop       *prometheus.CounterVec
	psRPCSent          *prometheus.CounterVec
	psRPCSubSent       prometheus.Counter
	psRPCPubSent       *prometheus.CounterVec
}

func NewGossipTracer(h host.Host) *gossipTracer {
	return &gossipTracer{
		host: h,
		p2PPeerCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "p2p_peer_count",
			Help: "The number of connected libp2p peers by agent string",
		}, []string{"agent"}),
		psMsgDeliver: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_deliver_total",
			Help: "The number of messages received for delivery",
		}, []string{"topic"}),
		psMsgUndeliverable: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_undeliverable_total",
			Help: "The number of messages received which weren't able to be delivered",
		}, []string{"topic"}),
		psMsgValidate: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_validate_total",
			Help: "The number of messages received for validation",
		}, []string{"topic"}),
		psMsgDuplicate: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_duplicate_total",
			Help: "The number of duplicate messages sent",
		}, []string{"topic"}),
		psMsgReject: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_reject_total",
			Help: "The number of messages rejected",
		}, []string{"topic", "reason"}),
		psTopicsActive: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "p2p_pubsub_topic_active",
			Help: "The topics that the peer is participating in gossipsub",
		}, []string{"topic"}),
		psTopicsPrune: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_prune_total",
			Help: "The number of prune messages sent by the peer",
		}, []string{"topic"}),
		psTopicsGraft: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_graft_total",
			Help: "The number of graft messages sent by the peer",
		}, []string{"topic"}),
		psThrottlePeer: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_throttle_total",
			Help: "The number of times the peer has been throttled",
		}, []string{"topic"}),
		psRPCRecv: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_recv_total",
			Help: "The number of messages received via rpc for a particular control message",
		}, []string{"control_message"}),
		psRPCSubRecv: promauto.NewCounter(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_recv_sub_total",
			Help: "The number of subscription messages received via rpc",
		}),
		psRPCPubRecv: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_recv_pub_total",
			Help: "The number of publish messages received via rpc",
		}, []string{"topic"}),
		psRPCDrop: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_drop_total",
			Help: "The number of messages dropped via rpc for a particular control message",
		}, []string{"control_message"}),
		psRPCSubDrop: promauto.NewCounter(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_drop_sub_total",
			Help: "The number of subscription messages dropped via rpc",
		}),
		psRPCPubDrop: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_drop_pub_total",
			Help: "The number of publish messages dropped via rpc",
		}, []string{"topic"}),
		psRPCSent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_sent_total",
			Help: "The number of messages sent via rpc for a particular control message",
		}, []string{"control_message"}),
		psRPCSubSent: promauto.NewCounter(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_sent_sub_total",
			Help: "The number of subscription messages sent via rpc",
		}),
		psRPCPubSent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "p2p_pubsub_rpc_sent_pub_total",
			Help: "The number of publish messages sent via rpc",
		}, []string{"topic"}),
	}
}

func (g *gossipTracer) AddPeer(p peer.ID, proto protocol.ID) {
	g.p2PPeerCount.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Inc()
}

func (g *gossipTracer) RemovePeer(p peer.ID) {
	g.p2PPeerCount.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Dec()
}

func (g *gossipTracer) Join(topic string) {
	g.psTopicsActive.WithLabelValues(topic).Set(1)
}

func (g *gossipTracer) Leave(topic string) {
	g.psTopicsActive.WithLabelValues(topic).Set(0)
}

func (g *gossipTracer) Graft(p peer.ID, topic string) {
	g.psTopicsGraft.WithLabelValues(topic).Inc()
}

func (g *gossipTracer) Prune(p peer.ID, topic string) {
	g.psTopicsPrune.WithLabelValues(topic).Inc()
}

func (g *gossipTracer) DeliverMessage(msg *pubsub.Message) {
	g.psMsgDeliver.WithLabelValues(*msg.Topic).Inc()
}

func (g *gossipTracer) ValidateMessage(msg *pubsub.Message) {
	g.psMsgValidate.WithLabelValues(*msg.Topic).Inc()
}

func (g *gossipTracer) RejectMessage(msg *pubsub.Message, reason string) {
	g.psMsgReject.WithLabelValues(*msg.Topic, reason).Inc()
}

func (g *gossipTracer) DuplicateMessage(msg *pubsub.Message) {
	g.psMsgDuplicate.WithLabelValues(*msg.Topic).Inc()
}

func (g *gossipTracer) UndeliverableMessage(msg *pubsub.Message) {
	g.psMsgUndeliverable.WithLabelValues(*msg.Topic).Inc()
}

func (g *gossipTracer) ThrottlePeer(p peer.ID) {
	g.psThrottlePeer.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Inc()
}

func (g *gossipTracer) RecvRPC(rpc *pubsub.RPC) {
	g.setRPCMetrics(g.psRPCSubRecv, g.psRPCPubRecv, g.psRPCRecv, rpc)
}

func (g *gossipTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	g.setRPCMetrics(g.psRPCSubSent, g.psRPCPubSent, g.psRPCSent, rpc)
}

func (g *gossipTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
	g.setRPCMetrics(g.psRPCSubDrop, g.psRPCPubDrop, g.psRPCDrop, rpc)
}

func (g *gossipTracer) setRPCMetrics(subCtr prometheus.Counter, pubCtr, ctrlCtr *prometheus.CounterVec, rpc *pubsub.RPC) {
	subCtr.Add(float64(len(rpc.Subscriptions)))
	if rpc.Control != nil {
		ctrlCtr.WithLabelValues("ihave").Add(float64(len(rpc.Control.Ihave)))
		ctrlCtr.WithLabelValues("iwant").Add(float64(len(rpc.Control.Iwant)))
		ctrlCtr.WithLabelValues("graft").Add(float64(len(rpc.Control.Graft)))
		ctrlCtr.WithLabelValues("prune").Add(float64(len(rpc.Control.Prune)))
		ctrlCtr.WithLabelValues("idontwant").Add(float64(len(rpc.Control.Idontwant)))
	}

	for _, msg := range rpc.Publish {
		pubCtr.WithLabelValues(*msg.Topic).Inc()
	}
}

func agentNameFromPeerID(pid peer.ID, store peerstore.PeerMetadata) string {
	const unknownAgent = "unknown"

	rawAgent, err := store.Get(pid, "AgentVersion")
	agent, ok := rawAgent.(string)
	if err != nil || !ok {
		return unknownAgent
	}

	agent = strings.ToLower(agent)
	for _, knownAgent := range knownAgentVersions {
		if strings.Contains(agent, knownAgent) {
			return knownAgent
		}
	}
	return unknownAgent
}
