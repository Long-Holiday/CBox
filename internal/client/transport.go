package client

import (
	"context"
	"net"
	"net/http"
	"time"
)

func NewUnixSocketClient(socketPath string) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
		DisableCompression: true,
		IdleConnTimeout:    30 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}
