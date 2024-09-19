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

var _ = pubsub.RawTracer(gossipTracer{})

const (
	unknownAgent = "unknown"
)

// Metrics collection for gossipsub
type gossipTracer struct {
	host host.Host
}

func NewGossipTracer(h host.Host) *gossipTracer {
	return &gossipTracer{host: h}
}

func (g gossipTracer) AddPeer(p peer.ID, proto protocol.ID) {
	p2pPeerCount.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Inc()
}

func (g gossipTracer) RemovePeer(p peer.ID) {
	p2pPeerCount.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Dec()
}

func (g gossipTracer) Join(topic string) {
	psTopicsActive.WithLabelValues(topic).Set(1)
}

func (g gossipTracer) Leave(topic string) {
	psTopicsActive.WithLabelValues(topic).Set(0)
}

func (g gossipTracer) Graft(p peer.ID, topic string) {
	psTopicsGraft.WithLabelValues(topic).Inc()
}

func (g gossipTracer) Prune(p peer.ID, topic string) {
	psTopicsPrune.WithLabelValues(topic).Inc()
}

func (g gossipTracer) DeliverMessage(msg *pubsub.Message) {
	psMsgDeliver.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) ValidateMessage(msg *pubsub.Message) {
	psMsgValidate.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) RejectMessage(msg *pubsub.Message, reason string) {
	psMsgReject.WithLabelValues(*msg.Topic, reason).Inc()
}

func (g gossipTracer) DuplicateMessage(msg *pubsub.Message) {
	psMsgDuplicate.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) UndeliverableMessage(msg *pubsub.Message) {
	psMsgUndeliverable.WithLabelValues(*msg.Topic).Inc()
}

func (g gossipTracer) ThrottlePeer(p peer.ID) {
	psThrottlePeer.WithLabelValues(agentNameFromPeerID(p, g.host.Peerstore())).Inc()
}

func (g gossipTracer) RecvRPC(rpc *pubsub.RPC) {
	g.setRPCMetrics(psRPCSubRecv, psRPCPubRecv, psRPCRecv, rpc)
}

func (g gossipTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	g.setRPCMetrics(psRPCSubSent, psRPCPubSent, psRPCSent, rpc)
}

func (g gossipTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
	g.setRPCMetrics(psRPCSubDrop, psRPCPubDrop, psRPCDrop, rpc)
}

func (g gossipTracer) setRPCMetrics(subCtr prometheus.Counter, pubCtr, ctrlCtr *prometheus.CounterVec, rpc *pubsub.RPC) {
	subCtr.Add(float64(len(rpc.Subscriptions)))
	if rpc.Control != nil {
		ctrlCtr.WithLabelValues("ihave").Add(float64(len(rpc.Control.Ihave)))
		ctrlCtr.WithLabelValues("iwant").Add(float64(len(rpc.Control.Iwant)))
		ctrlCtr.WithLabelValues("graft").Add(float64(len(rpc.Control.Graft)))
		ctrlCtr.WithLabelValues("prune").Add(float64(len(rpc.Control.Prune)))
	}
	for _, msg := range rpc.Publish {
		pubCtr.WithLabelValues(*msg.Topic).Inc()
	}
}

func agentNameFromPeerID(pid peer.ID, store peerstore.Peerstore) string {
	rawAgent, err := store.Get(pid, "AgentVersion")
	agent, ok := rawAgent.(string)
	if err != nil || !ok {
		return unknownAgent
	}
	for _, knownAgent := range knownAgentVersions {
		if strings.Contains(strings.ToLower(agent), knownAgent) {
			return knownAgent
		}
	}
	return unknownAgent
}
