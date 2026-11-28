package mcp

import (
	"context"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolObserver is the one hook the MCP layer offers to qbit-jay extensions
// (local telemetry, spec 10.1). It sees every tool call after the handler has
// produced its response and can neither change the response nor fail the call.
// The default is no observer: tool handling is exactly upstream's.
type ToolObserver interface {
	// ObserveTool receives the tool name, its arguments, the concatenated text
	// of the response, whether the response is an error, and the handler time.
	// Implementations must return quickly and must not panic.
	ObserveTool(name string, args map[string]any, text string, isError bool, latency time.Duration)
}

// observerMiddleware reports each call to obs. Panics in the observer are
// swallowed so an observer can never change a tool response.
func observerMiddleware(obs ToolObserver) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := time.Now()
			res, err := next(ctx, req)
			latency := time.Since(start)
			isError := err != nil
			var text strings.Builder
			if res != nil {
				isError = isError || res.IsError
				for _, c := range res.Content {
					if tc, ok := c.(mcp.TextContent); ok {
						text.WriteString(tc.Text)
					}
				}
			}
			func() {
				defer func() { _ = recover() }()
				obs.ObserveTool(req.Params.Name, req.GetArguments(), text.String(), isError, latency)
			}()
			return res, err
		}
	}
}
