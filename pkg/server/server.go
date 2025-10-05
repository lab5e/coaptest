// Package server implements a test server.
package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lab5e/coaptest/pkg/data"
	"github.com/plgd-dev/go-coap/v2/message"
	"github.com/plgd-dev/go-coap/v2/message/codes"
	"github.com/plgd-dev/go-coap/v2/mux"
	coapnet "github.com/plgd-dev/go-coap/v2/net"
	"github.com/plgd-dev/go-coap/v2/net/blockwise"
	"github.com/plgd-dev/go-coap/v2/udp"
	"github.com/plgd-dev/go-coap/v2/udp/client"
)

// Server represents the server
type Server struct {
	config     Config
	listenSock *coapnet.UDPConn
	udpServer  *udp.Server
}

// Config for server
type Config struct {
	ListenAddr string `kong:"help='CoAP server endpoint',default=':5683'"`
}

const (
	fotaTransferTimeout = time.Hour
)

// New server instance.
func New(config Config) Server {
	return Server{
		config: config,
	}
}

// Start server
func (s *Server) Start() error {
	m := mux.NewRouter()
	m.DefaultHandleFunc(s.defaultHandler)
	m.HandleFunc("/fw", s.fwHandler)

	var err error
	s.listenSock, err = coapnet.NewListenUDP("udp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("error creating CoAP listen socket for [%s]: %w", s.config.ListenAddr, err)
	}

	s.udpServer = udp.NewServer(
		udp.WithMux(m),
		udp.WithOnNewClientConn(s.onNewClientConn),
		udp.WithBlockwise(true, blockwise.SZX128, fotaTransferTimeout),
		udp.WithMaxMessageSize(450),
		udp.WithErrors(func(e error) {
			slog.Error("coap error", "err", e)
		}),
	)

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		wg.Done()
		err = s.udpServer.Serve(s.listenSock)
		if err != nil {
			slog.Error("server returned error", "err", err)
		}
	}()

	wg.Wait()

	return nil
}

// Shutdown server.
func (s *Server) Shutdown() error {
	if s.udpServer != nil {
		s.udpServer.Stop()
	}

	if s.listenSock != nil {
		return s.listenSock.Close()
	}

	return nil
}

// ListenAddr returns the local address of the listen socket. Useful for when
// you create a server using port 0 and let the network stack pick a port
// number for you.
func (s *Server) ListenAddr() string {
	if s.listenSock == nil {
		return ""
	}
	return s.listenSock.LocalAddr().String()
}

func (s *Server) defaultHandler(w mux.ResponseWriter, r *mux.Message) {
	slog.Info("defaultHandler", "seq", r.SequenceNumber, "token", r.Token, "opt", r.Options)

	err := w.SetResponse(
		codes.Content,
		message.AppJSON,
		bytes.NewReader([]byte("{'value':'foo'}")),
	)
	if err != nil {
		slog.Error("defaultHandler", "err", err)
	}
}

func (s *Server) fwHandler(w mux.ResponseWriter, r *mux.Message) {
	slog.Info("fwHandler",
		"seq", r.SequenceNumber,
		"token", fmt.Sprintf("%x", r.Token),
		"opt", r.Options,
	)

	imageReader := data.NewImageReader()

	err := w.SetResponse(
		codes.Content,
		message.AppOctets,
		imageReader,
	)
	if err != nil {
		slog.Error("error setting response", "err", err)
	}
}

func (s *Server) onNewClientConn(cc *client.ClientConn) {
	slog.Info("onNewClient", "remoteAddr", cc.Client().RemoteAddr().String())

	cc.AddOnClose(func() {
		slog.Info("closed client connection", "remoteAddr", cc.RemoteAddr().String())
	})
}
