package lsp_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
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

// Preserve structured result boundaries, including null and complete location arrays.
func resultReachedModel(content string, raw json.RawMessage) bool {
	var expected any
	return json.Unmarshal(raw, &expected) == nil && ContainsJSON(content, expected)
}

func checkSetupTools(requests []string) error {
	if len(requests) == 0 {
		return fmt.Errorf("the user's prompt never reached the LLM")
	}
	for _, body := range requests {
		var request struct {
			Tools []struct {
				Function struct {
					Name       string `json:"name"`
					Parameters struct {
						Properties map[string]json.RawMessage `json:"properties"`
						Required   []string                   `json:"required"`
					} `json:"parameters"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.Unmarshal([]byte(body), &request); err != nil {
			return err
		}
		advertised := false
		for _, tool := range request.Tools {
			parameters := tool.Function.Parameters
			if tool.Function.Name != "LSP" && parameters.Properties["operation"] == nil {
				continue
			}
			for name, kind := range map[string]string{"operation": "string", "filePath": "string", "line": "integer", "character": "integer"} {
				var property struct {
					Type string   `json:"type"`
					Enum []string `json:"enum"`
				}
				if json.Unmarshal(parameters.Properties[name], &property) != nil || property.Type != kind || !slices.Contains(parameters.Required, name) {
					return fmt.Errorf("advertise the supplied LSP schema with required %s of type %s", name, kind)
				}
				if name == "operation" && !slices.Contains(property.Enum, "goToDefinition") {
					return fmt.Errorf("the LSP operation enum must include goToDefinition")
				}
			}
			advertised = true
			break
		}
		if !advertised {
			return fmt.Errorf("advertise the supplied LSP tool in every model request")
		}
	}
	return nil
}

// Stage 2 accepts a placeholder sent back to the model under its original call ID.
func checkPlaceholderResponse(tools []call) error {
	for _, tool := range tools {
		if tool.Arguments["operation"] == "goToDefinition" && (tool.Result == "LSP is not implemented yet" || ContainsJSON(tool.Result, "LSP is not implemented yet")) {
			return nil
		}
	}
	return fmt.Errorf("return LSP is not implemented yet to the model with the matching tool_call_id")
}

// A Read must return the complete file contents in its own tool result.
func checkRead(root, name, text string, tools []call) error {
	for _, tool := range tools {
		file, _ := tool.Arguments["file_path"].(string)
		if file == "" {
			file, _ = tool.Arguments["path"].(string)
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(root, file)
		}
		if tool.Arguments["content"] == nil && filepath.Clean(file) == filepath.Join(root, name) && (tool.Result == text || ContainsJSON(tool.Result, text)) {
			return nil
		}
	}
	return fmt.Errorf("read %s and return its complete contents to the model", name)
}

func Check(c Case, slug string, config Config, requests []string) error {
	if slug == "iy3" {
		if err := checkSetupTools(requests); err != nil {
			return err
		}
	}
	events, err := Events(config.Log)
	if err != nil {
		return fmt.Errorf("language server never recorded initialize: %w", err)
	}
	var received []Message
	responses := map[string]Message{}
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
		}
	}
	if len(received) == 0 || received[0].Method != "initialize" {
		return fmt.Errorf("expected initialize as the first server message")
	}
	if !ended {
		return fmt.Errorf("the language-server process was not closed")
	}

	if len(requests) == 0 {
		return fmt.Errorf("the user's prompt never reached the LLM")
	}
	if reply, ok := responses[string(received[0].ID)]; !ok || len(reply.Error) > 0 {
		return fmt.Errorf("invalid initialization request: %s", reply.Error)
	}

	if len(received) < 2 || received[1].Method != "initialized" {
		return fmt.Errorf("send initialized after the successful initialization response")
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
	var queryTexts []string
	documents := map[string]string{}
	for _, message := range received {
		var params struct {
			TextDocument   Document `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		json.Unmarshal(message.Params, &params)
		if message.Method == "textDocument/didOpen" {
			documents[params.TextDocument.URI] = params.TextDocument.Text
		}
		if message.Method == "textDocument/didChange" && len(params.ContentChanges) == 1 {
			documents[params.TextDocument.URI] = params.ContentChanges[0].Text
		}
		if len(message.ID) > 0 && strings.HasPrefix(message.Method, "textDocument/") {
			queryMessages = append(queryMessages, message)
			queryTexts = append(queryTexts, documents[params.TextDocument.URI])
		}
	}
	allCalls := calls(requests)
	if slug == "iy3" && len(queryMessages) == 0 {
		return checkPlaceholderResponse(allCalls)
	}
	var expectedQueries []Action
	for _, action := range c.Actions {
		if action.Operation != "write" && action.Operation != "read" && action.Operation != "bash" {
			expectedQueries = append(expectedQueries, action)
		}
	}
	if len(queryMessages) < len(expectedQueries) {
		return fmt.Errorf("expected %d LSP queries, got %d", len(expectedQueries), len(queryMessages))
	}
	queryCalls := []call{}
	for _, tool := range allCalls {
		if operation, ok := tool.Arguments["operation"].(string); ok && operation != "" {
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
		file, _ := tool.Arguments["filePath"].(string)
		if file == "" {
			file, _ = tool.Arguments["path"].(string)
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(config.Root, file)
		}
		line, _ := tool.Arguments["line"].(float64)
		character, _ := tool.Arguments["character"].(float64)
		action := Action{Operation: operation, File: file, Line: int(line) - 1, Character: int(character) - 1}
		if len(matchedQueries) < len(expectedQueries) {
			expected := expectedQueries[len(matchedQueries)]
			samePosition := action.Line == expected.Line && action.Character == expected.Character
			if expected.AtDefinition {
				match := signaturePattern.FindStringSubmatchIndex(queryTexts[index])
				samePosition = false
				if match != nil {
					location := textRange(queryTexts[index], match[2], match[3])
					samePosition = action.Line == location.Start.Line && action.Character >= location.Start.Character && action.Character < location.End.Character
				}
			}
			samePosition = samePosition && action.Operation == expected.Operation && filepath.Clean(file) == filepath.Join(config.Root, expected.File)
			currentType := len(c.WantDefinitions) == 0 || len(matchedQueries) < len(c.WantDefinitions) && strings.Contains(tool.Result, c.WantDefinitions[len(matchedQueries)])
			if samePosition && currentType {
				matchedQueries = append(matchedQueries, tool)
			}
		}
		var params struct {
			TextDocument Document `json:"textDocument"`
			Position     Position `json:"position"`
			Context      struct {
				IncludeDeclaration bool `json:"includeDeclaration"`
			} `json:"context"`
		}
		json.Unmarshal(message.Params, &params)
		if message.Method != protocolMethod(action.Operation) || params.TextDocument.URI != URI(action.File) || params.Position != (Position{Line: action.Line, Character: action.Character}) {
			return fmt.Errorf("expected %s for %s at %d:%d, got %s %s", action.Operation, action.File, action.Line, action.Character, message.Method, message.Params)
		}
		if action.Operation == "findReferences" && !params.Context.IncludeDeclaration {
			return fmt.Errorf("findReferences must request includeDeclaration: true, as Claude Code does")
		}
		response, ok := responses[string(message.ID)]
		if !ok {
			return fmt.Errorf("missing response for request %s", message.ID)
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
			if !resultReachedModel(tool.Result, response.Result) {
				return fmt.Errorf("server result did not reach the matching tool call %s: expected %s, got %s", tool.ID, response.Result, tool.Result)
			}
		}
	}
	// Every open document must reflect the final contents on disk. Restarting or
	// omitting synchronization cannot turn an old server view into a pass.
	latest := map[string]Document{}
	for _, message := range received {
		if message.Method == "textDocument/didOpen" {
			var p struct {
				Document Document `json:"textDocument"`
			}
			json.Unmarshal(message.Params, &p)
			latest[p.Document.URI] = p.Document
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
	for uri, document := range latest {
		path, _ := Path(uri)
		text, err := os.ReadFile(path)
		if err != nil || string(text) != document.Text {
			return fmt.Errorf("server still has stale text for %s; after a successful Write, send didChange for an open file or didOpen with its new contents", filepath.Base(path))
		}
	}
	if len(matchedQueries) != len(expectedQueries) {
		return fmt.Errorf("missing the requested LSP operations in the model conversation")
	}
	if slug == "ql9" {
		// Recompute locations from disk so a cached closed file cannot pass with stale ranges.
		current := projectState{Root: config.Root, Documents: map[string]Document{}}
		expected, _ := json.Marshal(current.references(true))
		if !resultReachedModel(matchedQueries[len(matchedQueries)-1].Result, expected) {
			return fmt.Errorf("follow-up references contain stale locations; synchronize edited closed files such as reports.py with didOpen")
		}
		return checkMigration(c, config.Root, allCalls)
	}

	files := c.Files
	if files == nil {
		files = Project(false)
	}
	for _, action := range c.Actions {
		if action.Operation == "read" {
			if err := checkRead(config.Root, action.File, files[action.File], allCalls); err != nil {
				return err
			}
		}
	}

	// Check the final text of every successful scripted write, including unopened files.
	final := map[string]string{}
	for _, action := range c.Actions {
		if action.Operation == "write" {
			final[action.File] = action.Content
		}
	}
	for name, want := range final {
		data, err := os.ReadFile(filepath.Join(config.Root, name))
		if err != nil || string(data) != want {
			return fmt.Errorf("final file %s does not match the requested edit", name)
		}
	}

	return nil
}

// The capstone checks behavior rather than requiring one spelling of the migration.
func checkMigration(c Case, root string, tools []call) error {
	lastWrite := -1
	writes := map[string]int{}
	for index, tool := range tools {
		if tool.Arguments["content"] == nil {
			continue
		}
		lastWrite = index
		file, _ := tool.Arguments["file_path"].(string)
		if file == "" {
			file, _ = tool.Arguments["path"].(string)
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(root, file)
		}
		if _, exists := writes[filepath.Clean(file)]; !exists {
			writes[filepath.Clean(file)] = index
		}
	}
	for _, name := range []string{"pricing.py", "checkout.py", "reports.py"} {
		firstWrite, written := writes[filepath.Join(root, name)]
		if !written {
			return fmt.Errorf("use Write to migrate %s", name)
		}
		if err := checkRead(root, name, c.Files[name], tools[:firstWrite]); err != nil {
			return fmt.Errorf("before editing %s: %w", name, err)
		}
	}
	for name, original := range c.Files {
		if name == "pricing.py" || name == "checkout.py" || name == "reports.py" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(contents) != original {
			return fmt.Errorf("unrelated file %s must remain unchanged", name)
		}
	}
	tested := false
	for index, tool := range tools {
		command, _ := tool.Arguments["command"].(string)
		if index > lastWrite && strings.Contains(command, "unittest") && strings.Contains(command, "test_invoice") && strings.Contains(tool.Result, "Ran 5 tests") && strings.Contains(tool.Result, "OK") && !strings.Contains(tool.Result, "FAILED") {
			tested = true
		}
	}
	if !tested {
		return fmt.Errorf("run the project tests through Bash after editing and return the passing test output to the model")
	}
	// Independently verify the API and caller behavior, even if Bash output was fabricated.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", "-B", "-c", `
from pricing import Price, total
from checkout import checkout_total
from reports import report_total
from typing import is_typeddict, get_type_hints
import unittest
assert is_typeddict(Price) and get_type_hints(Price).get("cents") is int, "Price must be a TypedDict with integer cents"
assert get_type_hints(total).get("return") is Price, "pricing.total must declare its Price return type"
for price, quantity in [(250, 2), (100, 1), (37, 4), (0, 5)]:
    assert total(price, quantity) == {"cents": price * quantity}, "pricing.total must return cents in an object"
assert type(checkout_total()) is int and checkout_total() == 500, "checkout must still return integer 500"
assert type(report_total()) is int and report_total() == 600, "reporting must still return integer 600"
suite = unittest.defaultTestLoader.loadTestsFromName("test_invoice")
assert unittest.TextTestRunner(verbosity=2).run(suite).wasSuccessful()
`)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("migration behavior check failed: %s: %w", output, err)
	}
	return nil
}
