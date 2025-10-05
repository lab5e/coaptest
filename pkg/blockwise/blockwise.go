// Package blockwise implements two different blockwise get operations.
package blockwise

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/plgd-dev/go-coap/v2/message"
	"github.com/plgd-dev/go-coap/v2/message/codes"
	"github.com/plgd-dev/go-coap/v2/net/blockwise"
	"github.com/plgd-dev/go-coap/v2/udp"
	udpmsg "github.com/plgd-dev/go-coap/v2/udp/message"
	"github.com/plgd-dev/go-coap/v2/udp/message/pool"
)

// GetWithSameToken performs a blockwise GET on some resource and does not
// rotate the token for each exchange.
func GetWithSameToken(urlStr string, timeout time.Duration) ([]byte, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("URL parse error: %w", err)
	}

	if parsedURL.Scheme != "coap" {
		return nil, errors.New("URL scheme is not coap")
	}

	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
	defer cancel()

	conn, err := udp.Dial(
		parsedURL.Host,
		udp.WithBlockwise(true, blockwise.SZX1024, timeout),
		udp.WithMaxMessageSize(450),
	)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp, err := conn.Get(ctx, parsedURL.Path, message.Option{
		Value: []byte{0, byte(message.AppOctets)},
		ID:    message.Accept,
	})
	if err != nil {
		log.Fatalf("GET error: %v", err)
	}

	data, err := io.ReadAll(resp.Body())
	if err != nil {
		log.Fatalf("error reading data: %v", err)
	}

	return data, nil
}

// GetWithRotatingToken performs a GET request with blockwise transfer
// and rotates the token for each block exchange as per RFC 7959
func GetWithRotatingToken(urlStr string, timeout time.Duration) ([]byte, error) {
	// Parse the CoAP URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsedURL.Scheme != "coap" {
		return nil, fmt.Errorf("unsupported scheme: %s (only 'coap' is supported)", parsedURL.Scheme)
	}

	// Extract host and path
	host := parsedURL.Host
	if host == "" {
		return nil, fmt.Errorf("missing host in URL")
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	// Set default timeout if not provided
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Parse address once
	udpAddr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	// Create UDP connection once
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	defer udpConn.Close()

	return blockwiseGETWithTokenRotation(ctx, udpConn, path)
}

// blockwiseGETWithTokenRotation implements the actual blockwise transfer logic
func blockwiseGETWithTokenRotation(ctx context.Context, udpConn *net.UDPConn, path string) ([]byte, error) {
	msgPool := pool.New(2048, 2048)

	var fullPayload bytes.Buffer
	blockNum := uint32(0)
	szx := uint8(6) // Block size 1024 bytes
	messageID := uint16(1)

	// Reuse buffer for reading responses
	buf := make([]byte, 2048)

	for {
		// Check context
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled: %w", err)
		}

		// Generate a new random token for this exchange
		token := generateToken()

		// Create the GET request
		req := msgPool.AcquireMessage(ctx)
		req.SetCode(codes.GET)
		req.SetToken(token)
		req.SetMessageID(messageID)
		req.SetType(udpmsg.Confirmable)
		req.SetPath(path)

		// Add Block2 option
		block2Value := encodeBlock2(blockNum, false, szx)
		req.SetOptionUint32(message.Block2, block2Value)

		// Marshal and send the request
		data, err := req.Marshal()
		msgPool.ReleaseMessage(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		_, err = udpConn.Write(data)
		if err != nil {
			return nil, fmt.Errorf("failed to write request: %w", err)
		}

		// Set read timeout
		udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))

		// Read response
		n, err := udpConn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read response at block %d: %w", blockNum, err)
		}

		// Parse response
		resp := msgPool.AcquireMessage(ctx)
		_, err = resp.Unmarshal(buf[:n])
		if err != nil {
			msgPool.ReleaseMessage(resp)
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Verify token matches
		if !bytes.Equal(resp.Token(), token) {
			msgPool.ReleaseMessage(resp)
			return nil, fmt.Errorf("token mismatch at block %d: expected %x, got %x", blockNum, token, resp.Token())
		}

		// Check response code
		if resp.Code() != codes.Content && resp.Code() != codes.Valid {
			msgPool.ReleaseMessage(resp)
			return nil, fmt.Errorf("unexpected response code: %v", resp.Code())
		}

		// Get the payload from this block
		payload, err := resp.ReadBody()
		if err != nil {
			msgPool.ReleaseMessage(resp)
			return nil, fmt.Errorf("failed to read body at block %d: %w", blockNum, err)
		}

		fullPayload.Write(payload)

		// Check Block2 option in response
		block2Resp, err := resp.GetOptionUint32(message.Block2)
		msgPool.ReleaseMessage(resp)

		if err != nil {
			// No Block2 option means this is the last (or only) block
			break
		}

		// Decode Block2 response
		more, num, respSzx := decodeBlock2(block2Resp)

		// always follow server
		szx = uint8(respSzx)

		if !more {
			break
		}

		// Prepare for next block
		blockNum = num + 1
		messageID++
	}

	return fullPayload.Bytes(), nil
}

// generateToken creates a random token
func generateToken() []byte {
	token := make([]byte, 8)
	_, _ = rand.Read(token)
	return token
}

// encodeBlock2 encodes the Block2 option value
func encodeBlock2(num uint32, more bool, szx uint8) uint32 {
	value := (num << 4) | (uint32(szx) & 0x07)
	if more {
		value |= 0x08
	}
	return value
}

// decodeBlock2 decodes the Block2 option value
func decodeBlock2(v uint32) (more bool, num uint32, szx uint32) {
	more = ((v >> 3) & 0x1) == 1
	num = v >> 4
	szx = v & 0x7
	return
}
