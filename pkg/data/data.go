// Package data provides some data we can download
package data

import (
	_ "embed"

	"bytes"
	"io"
)

//go:embed bus.jpg
var imageData []byte

// NewImageReader returns a new image reader
func NewImageReader() io.ReadSeeker {
	return bytes.NewReader(imageData)
}
