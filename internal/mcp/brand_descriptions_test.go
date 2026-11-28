package mcp

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

var upstreamBrandWord = regexp.MustCompile(`(?i)\bengram\b`)

// upstreamBrandIn reports the first user-facing use of the upstream product
// name in text. The legacy `.engram/` project folder is still read, so naming
// it as such is allowed. `marked_by_actor="engram"` is a stored provenance value
// (S11), not prose.
func upstreamBrandIn(text string) string {
	text = strings.ReplaceAll(text, ".engram/", "")
	text = strings.ReplaceAll(text, `marked_by_actor="engram"`, "")
	return upstreamBrandWord.FindString(text)
}

// TestToolDescriptionsCarryQbitJayBrand pins spec S6 for the text agents read:
// no tool title, tool description, parameter description or server instruction
// names the upstream product. Tool names, parameters and the contract ids stay
// as they are (S11).
func TestToolDescriptionsCarryQbitJayBrand(t *testing.T) {
	srv := server.NewMCPServer("brand-check", "metadata")
	registerToolsWithQueue(srv, nil, MCPConfig{}, nil, nil, nil)
	tools := srv.ListTools()
	if len(tools) == 0 {
		t.Fatal("no tools registered")
	}
	for name, tool := range tools {
		if got := upstreamBrandIn(tool.Tool.Description); got != "" {
			t.Errorf("%s description names the upstream product: %q", name, tool.Tool.Description)
		}
		if got := upstreamBrandIn(tool.Tool.Annotations.Title); got != "" {
			t.Errorf("%s title names the upstream product: %q", name, tool.Tool.Annotations.Title)
		}
		for prop, raw := range tool.Tool.InputSchema.Properties {
			schema, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if desc, _ := schema["description"].(string); upstreamBrandIn(desc) != "" {
				t.Errorf("%s parameter %s names the upstream product: %q", name, prop, desc)
			}
		}
	}
	if got := upstreamBrandIn(buildServerInstructions(nil)); got != "" {
		t.Errorf("server instructions name the upstream product: %q", got)
	}
}

func TestUpstreamBrandInTriangulation(t *testing.T) {
	for text, want := range map[string]string{
		"reads .engram/config.json":         "",
		`marked_by_actor="engram"`:          "",
		"into Engram.":                      "Engram",
		"same as `engram doctor --json`":    "engram",
		"into qbit-jay.":                    "",
	} {
		if got := upstreamBrandIn(text); got != want {
			t.Errorf("upstreamBrandIn(%q) = %q, want %q", text, got, want)
		}
	}
}
