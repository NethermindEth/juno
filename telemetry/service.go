package telemetry

import (
	"context"
	"encoding/json"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/peer"
	"nhooyr.io/websocket"
	"runtime"
	"time"
)

var startTime = time.Now()

type service struct {
	peerID       peer.ID
	version      string
	websocketURL string

	log  utils.SimpleLogger
	conn *websocket.Conn
}

func NewService(peerID peer.ID, websocketURL, version string) *service {
	return &service{
		peerID:       peerID,
		version:      version,
		websocketURL: websocketURL,
	}
}

func (s *service) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := s.sendTelemetry(ctx)
			if err != nil {
				s.log.Errorw("Failed to send telemetry", "err", err)
			}
		}
	}
}

func (s *service) connect(ctx context.Context) (*websocket.Conn, error) {
	conn, _, err := websocket.Dial(ctx, s.websocketURL, nil)
	return conn, err
}

func (s *service) sendTelemetry(ctx context.Context) error {
	if s.conn == nil {
		conn, err := s.connect(ctx)
		if err != nil {
			return err
		}
		s.conn = conn
	}

	/*
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
	*/

	systemMsg := System{
		NodeUptime:      time.Since(startTime).Seconds(),
		OperatingSystem: runtime.GOOS,
		Memory:          0,
		MemoryUsage:     0,
		CPU:             "",
		CPUUsage:        0,
		Storage:         0,
		StorageUsage:    0,
	}
	bytes, err := json.Marshal(systemMsg)
	if err != nil {
		return err
	}

	return s.conn.Write(ctx, websocket.MessageText, bytes)
}
