package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"cbox/internal/daemon"
)

func main() {
	configPath := flag.String("config", "", "path to cbox config file")
	socketPath := flag.String("socket", "", "override unix socket path")
	tcpAddr := flag.String("tcp", "", "optional TCP listen address (e.g. 127.0.0.1:8080)")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	opts := daemon.Options{
		ConfigPath: *configPath,
		SocketPath: *socketPath,
		TCPAddr:    *tcpAddr,
		Debug:      *debug,
	}

	if err := daemon.Run(context.Background(), opts); err != nil {
		fmt.Fprintf(os.Stderr, "cboxd error: %v\n", err)
		os.Exit(1)
	}
}
