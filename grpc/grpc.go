package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"google.golang.org/grpc"
)

type Server struct {
	port    uint16
	version string
	srv     *grpc.Server
	db      db.DB
	log     utils.SimpleLogger
}

func NewServer(port uint16, version string, database db.DB, log utils.SimpleLogger) *Server {
	srv := grpc.NewServer()

	return &Server{
		srv:     srv,
		db:      database,
		port:    port,
		version: version,
		log:     log,
	}
}

func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		s.srv.Stop()
	}()

	gen.RegisterKVServer(s.srv, handlers{s.db, s.version})

	return s.srv.Serve(lis)
}
