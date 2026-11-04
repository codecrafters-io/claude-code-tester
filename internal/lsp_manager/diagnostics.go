package lsp_manager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Each Write must return the report for the document version it synchronized.
func checkWriteDiagnostics(config Config, events []Event, tools []call) error {
	previous := -1
	for _, tool := range tools {
		if operation, ok := tool.Arguments["operation"].(string); ok {
			for index := previous + 1; index < len(events); index++ {
				if events[index].Direction == "received" && events[index].Message.Method == protocolMethod(operation) {
					previous = index
					break
				}
			}
			continue
		}
		_, written := tool.Arguments["content"].(string)
		if !written {
			continue
		}
		file, _ := tool.Arguments["file_path"].(string)
		if file == "" {
			file, _ = tool.Arguments["path"].(string)
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(config.Root, file)
		}
		uri := URI(filepath.Clean(file))
		var result struct {
			Diagnostics      json.RawMessage `json:"diagnostics"`
			DiagnosticsError string          `json:"diagnosticsError"`
		}
		if json.Unmarshal([]byte(tool.Result), &result) != nil {
			return fmt.Errorf("Write %s must return diagnostic feedback in its matching tool result", tool.ID)
		}
		var document Document
		start := -1
		for index := previous + 1; index < len(events); index++ {
			event := events[index]
			if event.Direction != "received" || (event.Message.Method != "textDocument/didOpen" && event.Message.Method != "textDocument/didChange") {
				continue
			}
			var params struct {
				Document Document `json:"textDocument"`
			}
			json.Unmarshal(event.Message.Params, &params)
			if params.Document.URI == uri {
				document, start = params.Document, index
				break
			}
		}
		if start == -1 {
			return fmt.Errorf("Write %s did not synchronize %s before collecting diagnostics", tool.ID, filepath.Base(file))
		}
		previous = start
		var expected json.RawMessage
		for _, event := range events[start+1:] {
			// Reports belong to this save, before the next client operation.
			if event.Direction == "received" {
				break
			}
			if event.Direction != "sent" || event.Message.Method != "textDocument/publishDiagnostics" {
				continue
			}
			var publication struct {
				URI     string `json:"uri"`
				Version *int   `json:"version"`
			}
			json.Unmarshal(event.Message.Params, &publication)
			if publication.URI == uri && (publication.Version == nil || *publication.Version == document.Version) {
				expected = event.Message.Params
			}
		}
		if config.MissingDiagnostics {
			errorText := strings.ToLower(result.DiagnosticsError)
			if len(expected) != 0 || len(result.Diagnostics) != 0 && string(result.Diagnostics) != "null" || !strings.Contains(errorText, "diagnostic") || !strings.Contains(errorText, "timed out") {
				return fmt.Errorf("Write %s must report diagnosticsError with an explicit diagnostic timeout; no publication is not a clean report", tool.ID)
			}
			continue
		}
		if len(expected) == 0 || result.DiagnosticsError != "" || !resultReachedModel(string(result.Diagnostics), expected) {
			return fmt.Errorf("Write %s must return current diagnostics for %s version %d in its matching tool result, preserving the URI, version when supplied, and complete diagnostic array", tool.ID, filepath.Base(file), document.Version)
		}
	}
	return nil
}
