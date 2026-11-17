// Command portainer-mcp serves the Portainer tool catalog over MCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/catalog"
	"github.com/akrhin/portainer-mcp/internal/config"
	"github.com/akrhin/portainer-mcp/internal/mcpserver"
)

// Set by -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "portainer-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "path to the YAML config")
	toolsets := flag.String("toolsets", "", "comma-separated toolsets, overrides the config")
	readOnly := flag.Bool("read-only", false, "register read-only tools only")
	exhaustive := flag.Bool("exhaustive", false, "register every operation of the enabled toolsets, not the curated default set")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("portainer-mcp %s (%s, %s)\n", version, commit, date)
		return nil
	}

	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	var selected []string
	for _, s := range strings.Split(*toolsets, ",") {
		if s = strings.TrimSpace(s); s != "" {
			selected = append(selected, s)
		}
	}
	if len(selected) > 0 {
		if err := cat.CheckToolsets(selected); err != nil {
			return err
		}
	}
	cfg, err := config.Load(*configPath, cat)
	if err != nil {
		return err
	}

	srv, err := mcpserver.New(cfg, cat, mcpserver.Options{
		Toolsets:   selected,
		ReadOnly:   *readOnly,
		Exhaustive: *exhaustive,
		Version:    version,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	if cfg.Server.Transport == "http" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		hs := &http.Server{
			Addr:              cfg.Server.Listen,
			Handler:           mux,
			ReadHeaderTimeout: 15 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		fmt.Fprintf(os.Stderr, "portainer-mcp: streamable HTTP on %s/mcp\n", cfg.Server.Listen)
		return hs.ListenAndServe()
	}
	return srv.Run(ctx, &mcp.StdioTransport{})
}
