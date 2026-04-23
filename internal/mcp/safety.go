package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// confirmField is the schema fragment used by every destructive tool. The
// agent must pass `confirm: true` for the handler to proceed; this guards
// against an LLM that calls delete_destination while "tidying up" without
// first surfacing a yes/no to the human.
//
// The description is intentionally explicit because the model reads it before
// deciding whether to ask the user.
func confirmField() mcp.ToolOption {
	return mcp.WithBoolean("confirm",
		mcp.Required(),
		mcp.Description(
			"Required guard against accidental destructive calls. Must be set to true. "+
				"Surface this to the user as an explicit confirmation before calling — "+
				"do NOT pass true without checking. The user will get no other warning.",
		),
	)
}

// requireConfirm returns an error result if confirm != true. Call this at
// the very top of every destructive handler.
func requireConfirm(req mcp.CallToolRequest) *mcp.CallToolResult {
	if req.GetBool("confirm", false) {
		return nil
	}
	return mcp.NewToolResultError(
		"refused: this is a destructive operation — pass confirm:true after the user explicitly approves",
	)
}
