package main

import (
	"log"
	"net/http"

	"github.com/campfire/campfire/internal/core"
	"github.com/campfire/campfire/internal/mcp"
)

// mountMCP serves the agent interface from this same process, on this same
// origin, at a fixed path, over streamable HTTP (SPEC.md §14.2). There is no
// second port, no second process and no stdio bridge: an agent that speaks
// only stdio puts its own proxy in front of this.
func mountMCP(mux *http.ServeMux, svc *core.Service) {
	mux.Handle("/mcp", mcp.New(svc))
	log.Print("agent interface (MCP) on /mcp, authenticated by agent token")
}
