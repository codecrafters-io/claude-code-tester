package lsp_manager

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A server log where checkout.py is open at version 2 and pricing.py's change breaks it.
func publishedLog(t *testing.T, root string, version *int, message string) []Event {
	t.Helper()
	uri := URI(filepath.Join(root, "checkout.py"))
	opened, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}})
	report := map[string]any{"uri": uri, "diagnostics": []any{map[string]any{
		"message": message, "severity": 1,
		"range": Range{Start: Position{Line: 4, Character: 11}, End: Position{Line: 4, Character: 32}},
	}}}
	if version != nil {
		report["version"] = *version
	}
	published, _ := json.Marshal(report)
	return []Event{
		{Direction: "received", Message: Message{Method: "textDocument/didChange", Params: opened}},
		{Direction: "sent", Message: Message{Method: "textDocument/publishDiagnostics", Params: published}},
	}
}

// The conversation the model saw, with one tool result carrying the given text.
func conversation(text string) []string {
	body, _ := json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "tool", "tool_call_id": "write", "content": text}}})
	return []string{string(body)}
}

// A current error in any open file must reach the model with its file and 1-based position.
func TestDiagnosticsMustReachTheModel(t *testing.T) {
	two := 2
	for _, variant := range []struct {
		name    string
		version *int
		text    string
		valid   bool
	}{
		{"reported-1-based", &two, "Wrote pricing.py\n<new-diagnostics>checkout.py:\n  ✘ [Line 5:12] Price cannot be returned as int</new-diagnostics>", true},
		{"unversioned-report", nil, "checkout.py [Line 5:12] Price cannot be returned as int", true},
		{"never-reported", &two, "Wrote pricing.py", false},
		{"reported-0-based", &two, "checkout.py [Line 4:11] Price cannot be returned as int", false},
		{"wrong-file", &two, "pricing.py [Line 5:12] Price cannot be returned as int", false},
	} {
		t.Run(variant.name, func(t *testing.T) {
			// Arrange
			config := Config{Root: t.TempDir()}
			events := publishedLog(t, config.Root, variant.version, "Price cannot be returned as int")
			// Act
			err := checkDiagnosticsReachedModel(config, events, conversation(variant.text))
			// Assert
			if variant.valid {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "the model never saw the server's diagnostic for checkout.py at 5:12")
			}
		})
	}
}

// A report for an older version than the one last sent must not reach the model.
func TestDiagnosticsRejectStaleReports(t *testing.T) {
	// Arrange
	config := Config{Root: t.TempDir()}
	one := 1
	events := publishedLog(t, config.Root, &one, "Stale diagnostic must be ignored")
	// Act
	err := checkDiagnosticsReachedModel(config, events, conversation("checkout.py [Line 5:12] Stale diagnostic must be ignored"))
	// Assert
	require.ErrorContains(t, err, "a stale diagnostic reached the model")
}

// A server that publishes nothing requires nothing to be reported.
func TestDiagnosticsAllowNoReports(t *testing.T) {
	// Arrange
	config := Config{Root: t.TempDir()}
	// Act
	err := checkDiagnosticsReachedModel(config, nil, conversation("Wrote pricing.py"))
	// Assert
	require.NoError(t, err)
}
