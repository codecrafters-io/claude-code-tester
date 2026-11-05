package lsp_manager

import (
	"encoding/json"
	"fmt"
	"strings"
)

type diagnosticReport struct {
	URI         string `json:"uri"`
	Version     *int   `json:"version"`
	Diagnostics []struct {
		Range   Range  `json:"range"`
		Message string `json:"message"`
	} `json:"diagnostics"`
}

// conversationText is every string the model was sent, so a check can ask whether a
// diagnostic reached it, wherever the agent put it (a tool result, a system message).
func conversationText(requests []string) string {
	var parts []string
	var walk func(value any)
	walk = func(value any) {
		switch node := value.(type) {
		case string:
			parts = append(parts, node)
		case []any:
			for _, child := range node {
				walk(child)
			}
		case map[string]any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	for _, body := range requests {
		var decoded map[string]any
		if json.Unmarshal([]byte(body), &decoded) == nil {
			walk(decoded["messages"])
		}
	}
	return strings.Join(parts, "\n")
}

// checkDiagnosticsReachedModel requires every current error the server published, for any
// file, to reach the model with its file, 1-based position, and message, as Claude Code's
// <new-diagnostics> block does. A report for an older version than the one last sent must not.
func checkDiagnosticsReachedModel(config Config, events []Event, requests []string) error {
	versions := map[string]int{}
	type issue struct{ file, position, message string }
	var required []issue
	seen := map[issue]bool{}
	var stale []string
	for _, event := range events {
		switch {
		case event.Direction == "received" && (event.Message.Method == "textDocument/didOpen" || event.Message.Method == "textDocument/didChange"):
			var params struct {
				Document Document `json:"textDocument"`
			}
			json.Unmarshal(event.Message.Params, &params)
			versions[params.Document.URI] = params.Document.Version
		case event.Direction == "sent" && event.Message.Method == "textDocument/publishDiagnostics":
			var report diagnosticReport
			json.Unmarshal(event.Message.Params, &report)
			current := report.Version == nil || *report.Version == versions[report.URI]
			for _, diagnostic := range report.Diagnostics {
				if !current {
					stale = append(stale, diagnostic.Message)
					continue
				}
				found := issue{relativeFile(config.Root, report.URI), onePosition(diagnostic.Range.Start), diagnostic.Message}
				if !seen[found] {
					seen[found] = true
					required = append(required, found)
				}
			}
		}
	}
	text := conversationText(requests)
	for _, message := range stale {
		if strings.Contains(text, message) {
			return fmt.Errorf("a stale diagnostic reached the model (%q); ignore reports for an older version than the one you last sent", message)
		}
	}
	for _, want := range required {
		if !strings.Contains(text, want.message) || !strings.Contains(text, want.file) || !strings.Contains(text, want.position) {
			return fmt.Errorf("the model never saw the server's diagnostic for %s at %s (1-based): %q; read queued notifications before each model request and append new diagnostics to the latest tool result", want.file, want.position, want.message)
		}
	}
	return nil
}
