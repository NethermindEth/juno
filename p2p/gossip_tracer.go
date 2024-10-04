package p2p

import (
	"strings"

	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	_                  = pubsub.RawTracer(gossipTracer{})
	knownAgentVersions = []string{"juno", "papyrus", "pathfinder", "madara"}
)

// Metrics collection for gossipsub
type gossipTracer struct {
	host    host.Host
	metrics *P2PMetrics
}

func NewGossipTracer(h host.Host) *gossipTracer {
	return &gossipTracer{host: h, metrics: NewP2PMetrics()}
}

func (g gossipTracer) AddPeer(p peer.ID, proto protocol.ID) {
	g.metrics.P2PPeerCount.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Inc()
}

func (g gossipTracer) RemovePeer(p peer.ID) {
	g.metrics.P2PPeerCount.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Dec()
}

func (g gossipTracer) Join(topic string) {
	g.metrics.PSTopicsActive.WithLabelValues(topic).Set(1)
}

func (g gossipTracer) Leave(topic string) {
	g.metrics.PSTopicsActive.WithLabelValues(topic).Set(0)
}

func (g gossipTracer) Graft(p peer.ID, topic string) {
	g.metrics.PSTopicsGraft.WithLabelValues(topic).Inc()
}

func (g gossipTracer) Prune(p peer.ID, topic string) {
	g.metrics.PSTopicsPrune.WithLabelValues(topic).Inc()
}

func (g gossipTracer) DeliverMessage(msg *pubsub.Message) {
	g.metrics.PSMsgDeliver.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) ValidateMessage(msg *pubsub.Message) {
	g.metrics.PSMsgValidate.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) RejectMessage(msg *pubsub.Message, reason string) {
	g.metrics.PSMsgReject.WithLabelValues(*msg.Topic, reason).Inc()
}

func (g gossipTracer) DuplicateMessage(msg *pubsub.Message) {
	g.metrics.PSMsgDuplicate.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) UndeliverableMessage(msg *pubsub.Message) {
	g.metrics.PSMsgUndeliverable.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) ThrottlePeer(p peer.ID) {
	g.metrics.PSThrottlePeer.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Inc()
}

func (g gossipTracer) RecvRPC(rpc *pubsub.RPC) {
	g.setRPCMetrics(g.metrics.PSRPCSubRecv, g.metrics.PSRPCPubRecv, g.metrics.PSRPCRecv, rpc)
}

func (g gossipTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	g.setRPCMetrics(g.metrics.PSRPCSubSent, g.metrics.PSRPCPubSent, g.metrics.PSRPCSent, rpc)
}

func (g gossipTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
	g.setRPCMetrics(g.metrics.PSRPCSubDrop, g.metrics.PSRPCPubDrop, g.metrics.PSRPCDrop, rpc)
}

func (g gossipTracer) setRPCMetrics(subCtr prometheus.Counter, pubCtr, ctrlCtr *prometheus.CounterVec, rpc *pubsub.RPC) {
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

func agentNameFromPeerID(pid peer.ID, store peerstore.Peerstore) string {
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
