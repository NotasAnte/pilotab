package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/freepik-company/pilotab/internal/pilot"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pilotab:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/pilot.json", "path to the pilotab config")
	skipConfirmations := flag.Bool("dangerously-skip-confirmations", false, "run writes on confirmHosts and uploads without asking the user")
	flag.Parse()

	cfg, err := pilot.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	cfg.SkipConfirmations = *skipConfirmations
	if cfg.SkipConfirmations {
		fmt.Fprintln(os.Stderr, "pilotab: confirmations are OFF (--dangerously-skip-confirmations)")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	proxy, err := pilot.Start(ctx, cfg)
	if err != nil {
		return err
	}
	defer proxy.Close()

	server, err := proxy.Server(ctx)
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
