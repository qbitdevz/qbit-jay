package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type panickingObserver struct{ names []string }

func (p *panickingObserver) ObserveTool(name string, _ map[string]any, _ string, _ bool, _ time.Duration) {
	p.names = append(p.names, name)
	panic("observer bug")
}

func TestObserverSeesCallsAndCannotBreakResponses(t *testing.T) {
	s := newMCPTestStore(t)
	obs := &panickingObserver{}
	srv := NewServerWithConfig(s, MCPConfig{Observer: obs}, nil)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mem_stats","arguments":{}}}`
	raw, _ := json.Marshal(srv.HandleMessage(t.Context(), []byte(req)))
	if strings.Contains(string(raw), `"error"`) || !strings.Contains(string(raw), "Memory System Stats") {
		t.Fatalf("a panicking observer changed the response: %s", raw)
	}
	if len(obs.names) != 1 || obs.names[0] != "mem_stats" {
		t.Fatalf("observer calls = %v", obs.names)
	}
}
