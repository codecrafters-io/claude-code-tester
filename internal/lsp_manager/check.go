package lsp_manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type call struct {
	ID        string
	Arguments map[string]any
	Result    string
}

func calls(requests []string) []call {
	results := ToolResults(requests)
	seen := map[string]bool{}
	var ordered []call
	for _, body := range requests {
		var request struct {
			Messages []struct {
				Role      string `json:"role"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"messages"`
		}
		if json.Unmarshal([]byte(body), &request) != nil {
			continue
		}
		for _, message := range request.Messages {
			for _, tool := range message.ToolCalls {
				if seen[tool.ID] {
					continue
				}
				seen[tool.ID] = true
				var arguments map[string]any
				if json.Unmarshal([]byte(tool.Function.Arguments), &arguments) != nil {
					continue
				}
				ordered = append(ordered, call{ID: tool.ID, Arguments: arguments, Result: results[tool.ID]})
			}
		}
	}
	return ordered
}

// ContainsJSON allows a result envelope, while retaining array cardinality and
// numeric types. A references result with extra grep matches must not pass.
func ContainsJSON(content string, expected any) bool {
	var actual any
	if json.Unmarshal([]byte(content), &actual) != nil {
		return false
	}
	encoded, _ := json.Marshal(expected)
	json.Unmarshal(encoded, &expected)
	return containsNode(actual, expected)
}

func containsNode(actual, expected any) bool {
	if equalNode(actual, expected) {
		return true
	}
	switch value := actual.(type) {
	case map[string]any:
		for _, child := range value {
			if containsNode(child, expected) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if containsNode(child, expected) {
				return true
			}
		}
	}
	return false
}

func equalNode(actual, expected any) bool {
	switch want := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range want {
			child, exists := got[key]
			if !exists || !equalNode(child, value) {
				return false
			}
		}
		return true
	case []any:
		got, ok := actual.([]any)
		if !ok || len(got) != len(want) {
			return false
		}
		used := make([]bool, len(got))
		for _, item := range want {
			found := false
			for index, child := range got {
				if !used[index] && equalNode(child, item) {
					used[index] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

// Hover may be shown as its plaintext contents instead of the JSON envelope.
// Locations and diagnostics retain their structured fields for comparison.
func resultReachedModel(method, content string, raw json.RawMessage) bool {
	var expected any
	if json.Unmarshal(raw, &expected) != nil {
		return false
	}
	if ContainsJSON(content, expected) {
		return true
	}
	if method != "textDocument/hover" {
		return false
	}
	var hover struct {
		Contents struct {
			Value string `json:"value"`
		} `json:"contents"`
	}
	return json.Unmarshal(raw, &hover) == nil && hover.Contents.Value != "" && strings.Contains(content, hover.Contents.Value)
}

func Check(c Case, slug string, config Config, requests []string, firstRequest time.Time) error {
	if c.NoConfig {
		if len(requests) == 0 {
			return fmt.Errorf("the base prompt never reached the LLM")
		}
		return nil
	}
	events, err := Events(config.Log)
	if err != nil {
		return fmt.Errorf("language server never recorded initialize: %w", err)
	}
	var received []Message
	responses := map[string]Message{}
	var initReplyAt int64
	ended := false
	serverPID := 0
	for _, event := range events {
		if serverPID == 0 {
			serverPID = event.PID
		} else if event.PID != serverPID {
			return fmt.Errorf("use one language-server process for the entire invocation")
		}
		if event.Direction == "failure" {
			return fmt.Errorf("language-server protocol failed: %s", event.Detail)
		}
		if event.Direction == "ended" {
			ended = true
		}
		if event.Direction == "received" {
			received = append(received, event.Message)
		}
		if event.Direction == "sent" && len(event.Message.ID) > 0 {
			responses[string(event.Message.ID)] = event.Message
			if initReplyAt == 0 {
				initReplyAt = event.Time
			}
		}
	}
	if len(received) == 0 || received[0].Method != "initialize" {
		return fmt.Errorf("expected initialize as the first server message")
	}
	if !ended {
		return fmt.Errorf("the language-server process was not closed")
	}
	if c.Failure {
		for _, message := range received {
			if message.Method == "initialized" {
				return fmt.Errorf("initialized was sent after a failed initialization")
			}
		}
		if len(requests) > 0 {
			return fmt.Errorf("the LLM was called after initialization failed")
		}
		return nil
	}
	if len(requests) == 0 {
		return fmt.Errorf("the user's prompt never reached the LLM")
	}
	if reply, ok := responses[string(received[0].ID)]; !ok || len(reply.Error) > 0 {
		return fmt.Errorf("invalid initialization request: %s", reply.Error)
	}
	if slug == "ls1" {
		return nil
	}
	if initReplyAt == 0 || firstRequest.IsZero() || initReplyAt > firstRequest.UnixNano() {
		return fmt.Errorf("read initialization before calling the LLM (initialize reply: %d, first model request: %d)", initReplyAt, firstRequest.UnixNano())
	}
	if len(received) < 4 || received[1].Method != "initialized" || received[len(received)-2].Method != "shutdown" || received[len(received)-1].Method != "exit" {
		return fmt.Errorf("expected initialize, initialized, document operations, shutdown, exit in that order")
	}
	ids := map[string]bool{}
	for _, message := range received {
		if message.Method == "initialized" || message.Method == "exit" || strings.HasPrefix(message.Method, "textDocument/did") {
			if len(message.ID) > 0 {
				return fmt.Errorf("notification %s must not have an ID", message.Method)
			}
		} else if len(message.ID) == 0 {
			return fmt.Errorf("request %s must have an ID", message.Method)
		}
		if len(message.ID) > 0 {
			if ids[string(message.ID)] {
				return fmt.Errorf("reused LSP request ID %s", message.ID)
			}
			ids[string(message.ID)] = true
		}
	}
	var queryMessages []Message
	for _, message := range received {
		if len(message.ID) > 0 && strings.HasPrefix(message.Method, "textDocument/") {
			queryMessages = append(queryMessages, message)
		}
	}
	var expectedQueries []Action
	for _, action := range c.Actions {
		if action.Operation != "write" && action.Operation != "diagnostics" {
			expectedQueries = append(expectedQueries, action)
		}
	}
	if len(queryMessages) < len(expectedQueries) {
		return fmt.Errorf("expected %d LSP queries, got %d", len(expectedQueries), len(queryMessages))
	}
	queryCalls := []call{}
	allCalls := calls(requests)
	for _, tool := range allCalls {
		if operation, ok := tool.Arguments["operation"].(string); ok && operation != "diagnostics" {
			queryCalls = append(queryCalls, tool)
		}
	}
	if len(queryCalls) != len(queryMessages) {
		return fmt.Errorf("every LSP query must answer a model tool call, got %d tool calls and %d queries", len(queryCalls), len(queryMessages))
	}
	matchedQueries := []call{}
	for index, message := range queryMessages {
		tool := queryCalls[index]
		operation, _ := tool.Arguments["operation"].(string)
		file, _ := tool.Arguments["file_path"].(string)
		if file == "" {
			file, _ = tool.Arguments["path"].(string)
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(config.Root, file)
		}
		line, _ := tool.Arguments["line"].(float64)
		character, _ := tool.Arguments["character"].(float64)
		declaration, _ := tool.Arguments["include_declaration"].(bool)
		newName, _ := tool.Arguments["new_name"].(string)
		action := Action{Operation: operation, File: file, Line: int(line), Character: int(character), IncludeDeclaration: declaration, NewName: newName}
		if len(matchedQueries) < len(expectedQueries) {
			expected := expectedQueries[len(matchedQueries)]
			samePosition := action.Operation == expected.Operation && filepath.Clean(file) == filepath.Join(config.Root, expected.File) && action.Line == expected.Line && action.Character == expected.Character
			sameOptions := action.IncludeDeclaration == expected.IncludeDeclaration && action.NewName == expected.NewName
			currentType := len(c.WantTypes) == 0 || strings.Contains(tool.Result, "price: "+c.WantTypes[len(matchedQueries)])
			if samePosition && sameOptions && currentType {
				matchedQueries = append(matchedQueries, tool)
			}
		}
		var params struct {
			TextDocument Document `json:"textDocument"`
			Position     Position `json:"position"`
			Context      struct {
				IncludeDeclaration bool `json:"includeDeclaration"`
			} `json:"context"`
			NewName string `json:"newName"`
		}
		json.Unmarshal(message.Params, &params)
		if message.Method != "textDocument/"+action.Operation || params.TextDocument.URI != URI(action.File) || params.Position != (Position{Line: action.Line, Character: action.Character}) {
			return fmt.Errorf("expected %s for %s at %d:%d, got %s %s", action.Operation, action.File, action.Line, action.Character, message.Method, message.Params)
		}
		if action.Operation == "references" && params.Context.IncludeDeclaration != action.IncludeDeclaration {
			return fmt.Errorf("includeDeclaration was not forwarded")
		}
		if action.Operation == "rename" && params.NewName != action.NewName {
			return fmt.Errorf("newName was not forwarded")
		}
		response, ok := responses[string(message.ID)]
		if !ok {
			return fmt.Errorf("missing response for request %s", message.ID)
		}
		if action.Operation == "rename" && c.Rename {
			if tool.Result == "" {
				return fmt.Errorf("rename did not return a tool result")
			}
			continue
		}
		if len(response.Error) > 0 {
			var failure struct {
				Message string `json:"message"`
			}
			json.Unmarshal(response.Error, &failure)
			if !strings.Contains(tool.Result, failure.Message) {
				return fmt.Errorf("the server's error did not reach the model for %s", tool.ID)
			}
		} else {
			if !resultReachedModel(message.Method, tool.Result, response.Result) {
				return fmt.Errorf("server result did not reach the matching tool call %s: expected %s, got %s", tool.ID, response.Result, tool.Result)
			}
		}
	}
	// Every open document must reflect the final contents on disk. Restarting or
	// omitting synchronization cannot turn an old server view into a pass.
	latest := map[string]Document{}
	open := map[string]bool{}
	for _, message := range received {
		if message.Method == "textDocument/didOpen" {
			var p struct {
				Document Document `json:"textDocument"`
			}
			json.Unmarshal(message.Params, &p)
			latest[p.Document.URI] = p.Document
			open[p.Document.URI] = true
		}
		if message.Method == "textDocument/didClose" {
			var p struct {
				Document Document `json:"textDocument"`
			}
			json.Unmarshal(message.Params, &p)
			delete(open, p.Document.URI)
		}
		if message.Method == "textDocument/didChange" {
			var p struct {
				Document Document `json:"textDocument"`
				Changes  []struct {
					Text string `json:"text"`
				} `json:"contentChanges"`
			}
			json.Unmarshal(message.Params, &p)
			if len(p.Changes) == 1 {
				p.Document.Text = p.Changes[0].Text
				latest[p.Document.URI] = p.Document
			}
		}
	}
	if len(open) > 0 {
		return fmt.Errorf("send didClose for every open document before shutdown")
	}
	for uri, document := range latest {
		path, _ := Path(uri)
		text, err := os.ReadFile(path)
		if err != nil || string(text) != document.Text {
			return fmt.Errorf("server still has stale text for %s", filepath.Base(path))
		}
	}
	if len(matchedQueries) != len(expectedQueries) {
		return fmt.Errorf("missing the requested LSP operations in the model conversation")
	}
	if len(c.WantTypes) > 0 {
		for index, kind := range c.WantTypes {
			if !strings.Contains(matchedQueries[index].Result, "price: "+kind) {
				return fmt.Errorf("hover %d should report price: %s", index+1, kind)
			}
		}
	}
	if len(c.WantDiagnosticVersions) > 0 {
		diagnosticCalls := []call{}
		for _, tool := range allCalls {
			if tool.Arguments["operation"] == "diagnostics" || tool.Arguments["content"] != nil {
				diagnosticCalls = append(diagnosticCalls, tool)
			}
		}
		requiredCalls := []call{}
		next := 0
		for _, action := range c.Actions {
			if action.Operation != "diagnostics" && action.Operation != "write" {
				continue
			}
			found := false
			for next < len(diagnosticCalls) {
				tool := diagnosticCalls[next]
				next++
				file, _ := tool.Arguments["file_path"].(string)
				if file == "" {
					file, _ = tool.Arguments["path"].(string)
				}
				if !filepath.IsAbs(file) {
					file = filepath.Join(config.Root, file)
				}
				write := tool.Arguments["content"] != nil
				if filepath.Clean(file) == filepath.Join(config.Root, action.File) && ((action.Operation == "write" && write) || (action.Operation == "diagnostics" && !write)) {
					requiredCalls = append(requiredCalls, tool)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("missing %s tool result for %s", action.Operation, action.File)
			}
		}
		diagnosticCalls = requiredCalls

		for index, version := range c.WantDiagnosticVersions {
			var want any
			for _, event := range events {
				if event.Direction == "sent" && event.Message.Method == "textDocument/publishDiagnostics" {
					var p map[string]any
					json.Unmarshal(event.Message.Params, &p)
					if p["version"] == float64(version) && p["uri"] == URI(filepath.Join(config.Root, "checkout.py")) {
						want = map[string]any{"version": p["version"], "diagnostics": p["diagnostics"]}
						break
					}
				}
			}
			if want == nil || !ContainsJSON(diagnosticCalls[index].Result, want) {
				return fmt.Errorf("tool result %s must carry version %d's diagnostics, got %s", diagnosticCalls[index].ID, version, diagnosticCalls[index].Result)
			}
		}
	}
	if config.MissingDiagnostics {
		if !containsResult(ToolResults(requests), "error") {
			return fmt.Errorf("missing diagnostics must be an error, not an empty success list")
		}
	}
	files := c.Files
	if files == nil {
		files = Project(false, false)
	}
	if c.Rename || slug == "rn7" {
		for name, original := range files {
			expected := original
			if c.Rename && name != "preview.py" {
				expected = wordPattern.ReplaceAllStringFunc(original, func(word string) string {
					if word == "total" {
						return "total_cents"
					}
					return word
				})
			}
			if c.Rename && expected != original {
				uri := URI(filepath.Join(config.Root, name))
				document, ok := latest[uri]
				if !ok || document.Text != expected {
					return fmt.Errorf("rename was not synchronized for %s", name)
				}
				var publication any
				for _, event := range events {
					if event.Direction == "sent" && event.Message.Method == "textDocument/publishDiagnostics" {
						var params map[string]any
						json.Unmarshal(event.Message.Params, &params)
						if params["uri"] == uri && params["version"] == float64(document.Version) {
							publication = map[string]any{"version": params["version"], "diagnostics": params["diagnostics"]}
						}
					}
				}
				result := matchedQueries[0].Result
				if publication == nil || !ContainsJSON(result, publication) || !strings.Contains(result, name) {
					return fmt.Errorf("rename must return the changed path and current diagnostics for %s", name)
				}
			}
			actual, err := os.ReadFile(filepath.Join(config.Root, name))
			if err != nil || string(actual) != expected {
				return fmt.Errorf("incorrect rename in %s\nexpected: %q\nactual: %q", name, expected, string(actual))
			}
		}
	}
	return nil
}
