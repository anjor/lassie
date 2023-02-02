package httpserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/filecoin-project/lassie/pkg/lassie"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("lassie/httpserver")

type HttpServer struct {
	cancel   context.CancelFunc
	ctx      context.Context
	lassie   *lassie.Lassie
	listener net.Listener
	server   *http.Server
}

func NewHttpServer(ctx context.Context, address string, port uint, handlerWriter io.Writer) (*HttpServer, error) {
	addr := fmt.Sprintf("%s:%d", address, port)
	listener, err := net.Listen("tcp", addr) // assigns a port if port is 0
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	// create server
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:        addr,
		BaseContext: func(listener net.Listener) context.Context { return ctx },
		Handler:     mux,
	}

	// create a lassie instance
	lassie, err := lassie.NewLassie(ctx, lassie.WithTimeout(20*time.Second))
	if err != nil {
		cancel()
		return nil, err
	}

	httpServer := &HttpServer{
		cancel:   cancel,
		ctx:      ctx,
		lassie:   lassie,
		listener: listener,
		server:   server,
	}

	// Routes
	mux.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/ipfs/", ipfsHandler(lassie, handlerWriter))

	return httpServer, nil
}

func (s HttpServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *HttpServer) Start() error {
	log.Infow("starting http server", "listen_addr", s.listener.Addr())
	err := s.server.Serve(s.listener)
	if err != http.ErrServerClosed {
		log.Errorw("failed to start http server", "err", err)
		return err
	}

	return nil
}

func (s *HttpServer) Close() error {
	log.Info("closing http server")
	s.cancel()
	return s.server.Shutdown(context.Background())
}
