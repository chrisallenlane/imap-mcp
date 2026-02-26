// Package main is the entry point for the imap-mcp MCP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	imapmanager "github.com/chrisallenlane/imap-mcp/internal/imap"
	"github.com/chrisallenlane/imap-mcp/internal/server"
)

func main() {
	configPath := flag.String(
		"config",
		"",
		"path to TOML config file",
	)
	versionFlag := flag.Bool(
		"version",
		false,
		"print version and exit",
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf(
			"imap-mcp v%s\n",
			server.ServerVersion,
		)
		os.Exit(0)
	}

	if *configPath == "" {
		log.Fatal("--config flag is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mgr := imapmanager.NewManager(cfg)
	defer mgr.Close()

	s := server.New(mgr)

	if err := s.Run(
		context.Background(),
		os.Stdin,
		os.Stdout,
	); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
