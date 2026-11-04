package lsp_manager

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Diagnostic feedback must describe the saved file and version, including complete errors.
func TestWriteDiagnosticsRejectsIncorrectFeedback(t *testing.T) {
	for _, variant := range []string{"missing", "discarded-error", "wrong-file", "stale-version"} {
		t.Run(variant, func(t *testing.T) {
			// Arrange
			config := Config{Root: t.TempDir()}
			uri := URI(filepath.Join(config.Root, "checkout.py"))
			report := map[string]any{"uri": uri, "version": 2, "diagnostics": []any{map[string]any{
				"message": "Price cannot be returned as int", "severity": 1,
				"range": Range{Start: Position{Line: 4}, End: Position{Line: 4, Character: 10}},
			}}}
			expected, _ := json.Marshal(report)
			params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}})
			events := []Event{
				{Direction: "received", Message: Message{Method: "textDocument/didChange", Params: params}},
				{Direction: "sent", Message: Message{Method: "textDocument/publishDiagnostics", Params: expected}},
			}
			switch variant {
			case "discarded-error":
				report["diagnostics"] = []any{}
			case "wrong-file":
				report["uri"] = URI(filepath.Join(config.Root, "preview.py"))
			case "stale-version":
				report["version"] = 1
			}
			feedback, _ := json.Marshal(map[string]any{"diagnostics": report})
			if variant == "missing" {
				feedback = []byte(`{"written":"checkout.py"}`)
			}
			tools := []call{{ID: "provider-write", Arguments: map[string]any{"file_path": "checkout.py", "content": "updated source"}, Result: string(feedback)}}
			// Act
			err := checkWriteDiagnostics(config, events, tools)
			// Assert
			require.ErrorContains(t, err, "must return current diagnostics")
		})
	}
}

// A missing publication requires an explicit timeout rather than a fabricated clean report.
func TestWriteDiagnosticsRequiresExplicitTimeout(t *testing.T) {
	for _, clean := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout", true: "fake-clean"}[clean], func(t *testing.T) {
			// Arrange
			config := Config{Root: t.TempDir(), MissingDiagnostics: true}
			uri := URI(filepath.Join(config.Root, "checkout.py"))
			params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1}})
			events := []Event{{Direction: "received", Message: Message{Method: "textDocument/didOpen", Params: params}}}
			result := `{"diagnosticsError":"diagnostics timed out"}`
			if clean {
				result = `{"diagnostics":{"uri":"` + uri + `","diagnostics":[]}}`
			}
			tools := []call{{Arguments: map[string]any{"file_path": "checkout.py", "content": "updated source"}, Result: result}}
			// Act
			err := checkWriteDiagnostics(config, events, tools)
			// Assert
			if clean {
				require.ErrorContains(t, err, "must report diagnosticsError")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
