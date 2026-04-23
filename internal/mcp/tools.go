package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/notifypulse/internal/client"
)

// deps is the tiny struct every tool handler closes over. Bundling the
// client + dashboard base URL keeps every register* function's signature
// identical and makes it obvious where dashboard links come from.
type deps struct {
	api     *client.Client
	baseURL string // for dashboard links — same host as the API for now
}

// registerTools wires every v1 tool onto the given server. Each subset lives
// in its own register* function so the tool surface is easy to read top-down.
func registerTools(srv *mcpserver.MCPServer, api *client.Client, baseURL string) {
	d := &deps{api: api, baseURL: baseURL}

	registerDestinationTools(srv, d)
	registerRecipientTools(srv, d)
	registerNotifyTool(srv, d)
	registerNotificationTools(srv, d)
}
