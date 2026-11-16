// Command arcane-mcp serves the Arcane tool catalog over MCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/akrhin/portainer-mcp/internal/arcane"
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
		fmt.Fprintln(os.Stderr, "arcane-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	url := flag.String("url", "", "base URL of the Arcane manager, for example http://arcane:3552")
	apiKey := flag.String("api-key", "${ARCANE_API_KEY}", "API key sent as X-API-Key")
	name := flag.String("name", "arcane", "instance name reported in errors")
	toolsets := flag.String("toolsets", "", "comma-separated toolsets, default all")
	readOnly := flag.Bool("read-only", false, "register read-only tools only")
	exhaustive := flag.Bool("exhaustive", false, "register every operation of the enabled toolsets, not the curated default set")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("arcane-mcp %s (%s, %s)\n", version, commit, date)
		return nil
	}
	if *url == "" {
		return fmt.Errorf("--url is required")
	}

	cat, err := catalog.LoadArcane()
	if err != nil {
		return err
	}

	var selected []string
	for _, s := range strings.Split(*toolsets, ",") {
		if s = strings.TrimSpace(s); s != "" {
			selected = append(selected, s)
		}
	}
	if len(selected) == 0 {
		selected = cat.Names()
	}
	cfg, err := config.Single(cat, *name, *url, *apiKey, selected)
	if err != nil {
		return err
	}

	srv, err := mcpserver.New(cfg, cat, mcpserver.Options{
		Toolsets:   selected,
		ReadOnly:   *readOnly,
		Exhaustive: *exhaustive,
		Name:       "arcane-mcp",
		Open:       arcane.New,
	})
	if err != nil {
		return err
	}
	return srv.Run(context.Background(), &mcp.StdioTransport{})
}
