// Package main implements a coap server
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/lab5e/coaptest/pkg/server"
)

var opt struct {
	server.Config
}

func main() {
	kong.Parse(&opt)

	server := server.New(server.Config{
		ListenAddr: opt.ListenAddr,
	})

	err := server.Start()
	if err != nil {
		slog.Error("error starting server", "err", err)
		return
	}

	slog.Info("server listening to", "listenAddr", server.ListenAddr())

	// block until ctrl-C
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
}
