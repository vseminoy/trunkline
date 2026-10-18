package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-jira-mcp/internal/config"
	"go-jira-mcp/internal/tools"
)

const (
	serverName    = "jira-tasks"
	serverVersion = "0.1.0"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	svc := tools.NewService(cfg)
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)
	tools.Register(server, svc)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}
