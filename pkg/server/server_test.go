package server

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoAPServer(t *testing.T) {
	server := New(Config{
		ListenAddr: ":0",
	})

	require.NoError(t, server.Start())
	fmt.Println(server.ListenAddr())
	require.NoError(t, server.Shutdown())
}
