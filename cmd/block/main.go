// Package main is a client
package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lab5e/coaptest/pkg/blockwise"
)

var opt struct {
	URL     string        `kong:"arg,help='CoAP URL'"`
	Timeout time.Duration `kong:"help='timeout for blockwise transfer',default='60s'"`
	Rotate  bool          `kong:"help='rotate request tokens for each exchange'"`
}

func main() {
	kong.Parse(&opt)

	var data []byte
	var err error

	if opt.Rotate {
		slog.Info("GetWithTokenRotation")
		data, err = blockwise.GetWithRotatingToken(opt.URL, opt.Timeout)
	} else {
		slog.Info("GetWithSameToken")
		data, err = blockwise.GetWithSameToken(opt.URL, opt.Timeout)
	}
	if err != nil {
		log.Fatal(err)
	}

	checksum := sha256.Sum256(data)
	fmt.Printf("size=%d checksum=%x\n", len(data), checksum)
}
