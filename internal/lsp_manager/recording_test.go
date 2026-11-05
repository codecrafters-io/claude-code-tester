package lsp_manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// checkRecording supplies checker inputs directly; it never serves model responses.
func checkRecording(t *testing.T, c Case, root string, tools []call) (Config, []string) {
	t.Helper()
	config := c.Config
	config.Root = root
	config.Log = filepath.Join(t.TempDir(), "events.jsonl")
	state := projectState{Root: root, Documents: map[string]Document{}, Indexed: map[string]string{}}
	for name, text := range c.Files {
		state.Indexed[URI(filepath.Join(root, name))] = text
	}
	emit := func(direction, method string, id int, params any, result any) {
		t.Helper()
		message := Message{JSONRPC: "2.0", Method: method}
		if id != 0 {
			message.ID = json.RawMessage(fmt.Sprint(id))
		}
		if params != nil {
			message.Params, _ = json.Marshal(params)
		}
		if result != nil {
			message.Result, _ = json.Marshal(result)
		}
		require.NoError(t, appendEvent(config.Log, direction, message, ""))
	}
	emit("received", "initialize", 1, nil, nil)
	emit("sent", "", 1, nil, map[string]any{"capabilities": map[string]any{}})
	emit("received", "initialized", 0, nil, nil)
	// publish mirrors the fixture server: after a document changes, it reports every open
	// document. Like Claude Code, it returns a <new-diagnostics> block of issues not yet shown.
	shown := map[string]bool{}
	publish := func(changed string) string {
		t.Helper()
		uris := []string{changed}
		var others []string
		for uri := range state.Documents {
			if uri != changed {
				others = append(others, uri)
			}
		}
		sort.Strings(others)
		block := ""
		for _, uri := range append(uris, others...) {
			document := state.Documents[uri]
			diagnostics := state.diagnostics(document)
			report := map[string]any{"uri": uri, "diagnostics": diagnostics}
			if !config.UnversionedDiagnostics {
				report["version"] = document.Version
			}
			emit("sent", "textDocument/publishDiagnostics", 0, report, nil)
			lines := ""
			for _, diagnostic := range diagnostics {
				line := fmt.Sprintf("  ✘ [Line %s] %s", onePosition(diagnostic["range"].(Range).Start), diagnostic["message"])
				if !shown[uri+line] {
					shown[uri+line] = true
					lines += "\n" + line
				}
			}
			if lines != "" {
				block += "\n\n" + relativeFile(root, uri) + ":" + lines
			}
		}
		if block == "" {
			return ""
		}
		return "\n\n<new-diagnostics>The following new diagnostic issues were detected:" + block + "</new-diagnostics>"
	}
	open := func(name string) (Document, string) {
		t.Helper()
		uri := URI(filepath.Join(root, name))
		if document, ok := state.Documents[uri]; ok {
			return document, ""
		}
		document := Document{URI: uri, LanguageID: "python", Version: 1, Text: state.text(name)}
		state.Documents[uri] = document
		emit("received", "textDocument/didOpen", 0, map[string]any{"textDocument": document}, nil)
		return document, publish(uri)
	}
	var messages []any
	for index, tool := range tools {
		id := fmt.Sprintf("provider-call-%d", index)
		name := "Read"
		if operation, ok := tool.Arguments["operation"].(string); ok {
			name = "LSP"
			file := tool.Arguments["filePath"].(string)
			document, reported := open(file)
			params := map[string]any{
				"textDocument": map[string]any{"uri": document.URI},
				"position":     Position{Line: tool.Arguments["line"].(int) - 1, Character: tool.Arguments["character"].(int) - 1},
			}
			var result any
			if operation == "findReferences" {
				params["context"] = map[string]any{"includeDeclaration": true}
				result = state.references(true)
			} else {
				result = state.definition()
			}
			emit("received", protocolMethod(operation), index+2, params, nil)
			emit("sent", "", index+2, nil, result)
			encoded, _ := json.Marshal(result)
			tool.Result = claudeCodeFormat(root, encoded, operation) + reported
		} else if content, ok := tool.Arguments["content"].(string); ok {
			name = "Write"
			file := tool.Arguments["file_path"].(string)
			uri := URI(filepath.Join(root, file))
			document, opened := state.Documents[uri]
			document.URI, document.LanguageID, document.Text = uri, "python", content
			document.Version++
			state.Documents[uri] = document
			if opened {
				emit("received", "textDocument/didChange", 0, map[string]any{
					"textDocument":   map[string]any{"uri": uri, "version": document.Version},
					"contentChanges": []any{map[string]any{"text": content}},
				}, nil)
			} else {
				emit("received", "textDocument/didOpen", 0, map[string]any{"textDocument": document}, nil)
			}
			tool.Result = "Wrote " + file + publish(uri)
		} else if tool.Arguments["command"] != nil {
			name = "Bash"
		}
		arguments, _ := json.Marshal(tool.Arguments)
		messages = append(messages,
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(arguments)},
			}}},
			map[string]any{"role": "tool", "tool_call_id": id, "content": tool.Result},
		)
	}
	emit("ended", "", 0, nil, nil)
	body, err := json.Marshal(map[string]any{"messages": messages})
	require.NoError(t, err)
	return config, []string{string(body)}
}

func navigation(operation, file string, line int) call {
	return call{Arguments: map[string]any{"operation": operation, "filePath": file, "line": line, "character": 5}}
}

func migrationCalls(t *testing.T) (Case, string, []call) {
	t.Helper()
	c, root, edits := migratedProject(t)
	definition := navigation("goToDefinition", "checkout.py", 5)
	definition.Arguments["character"] = 12
	tools := []call{definition, navigation("findReferences", "pricing.py", 1)}
	tools = append(tools, edits[:len(edits)-1]...)
	tools = append(tools, definition, navigation("findReferences", "pricing.py", 8), edits[len(edits)-1])
	return c, root, tools
}

// Provider-selected navigation order can vary within each migration phase.
func TestCheckMigrationAcceptsEitherNavigationOrder(t *testing.T) {
	for _, phase := range []string{"original", "before", "after", "both"} {
		t.Run(phase, func(t *testing.T) {
			// Arrange
			c, root, tools := migrationCalls(t)
			if phase == "before" || phase == "both" {
				tools[0], tools[1] = tools[1], tools[0]
			}
			if phase == "after" || phase == "both" {
				tools[8], tools[9] = tools[9], tools[8]
			}
			config, requests := checkRecording(t, c, root, tools)
			// Act
			err := Check(c, "ql9", config, requests)
			// Assert
			require.NoError(t, err)
		})
	}
}

// A zero-based result coordinate cannot satisfy a one-based follow-up query.
func TestCheckMigrationRejectsZeroBasedFollowup(t *testing.T) {
	// Arrange
	c, root, tools := migrationCalls(t)
	tools[9].Arguments["line"] = 7
	config, requests := checkRecording(t, c, root, tools)
	// Act
	err := Check(c, "ql9", config, requests)
	// Assert
	require.ErrorContains(t, err, "missing findReferences on pricing.py at the current definition")
}

// Navigation before the final file edit cannot satisfy the post-edit checks.
func TestCheckMigrationRequiresNavigationAfterAllEdits(t *testing.T) {
	// Arrange
	c, root, tools := migrationCalls(t)
	tools = append(append(append([]call{}, tools[:7]...), tools[8:10]...), tools[7], tools[10])
	config, requests := checkRecording(t, c, root, tools)
	// Act
	err := Check(c, "ql9", config, requests)
	// Assert
	require.ErrorContains(t, err, "after editing")
}

// A server result must reach the model under the matching query call ID.
func TestCheckRejectsDiscardedDefinition(t *testing.T) {
	// Arrange
	c, root, tools := migrationCalls(t)
	config, requests := checkRecording(t, c, root, tools)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(requests[0]), &body))
	body["messages"].([]any)[1].(map[string]any)["content"] = ""
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	// Act
	err = Check(c, "ql9", config, []string{string(encoded)})
	// Assert
	require.ErrorContains(t, err, "should list pricing.py at 1:5")
}

// A document that differs from the final saved file is stale even if lookups succeeded.
func TestCheckRejectsUnsynchronizedText(t *testing.T) {
	// Arrange
	c, root, tools := migrationCalls(t)
	config, requests := checkRecording(t, c, root, tools)
	require.NoError(t, os.WriteFile(filepath.Join(root, "checkout.py"), []byte("# unsynchronized edit\n"), 0644))
	// Act
	err := Check(c, "ql9", config, requests)
	// Assert
	require.ErrorContains(t, err, "server still has stale text for checkout.py")
}

// A shifted starting definition is graded at its current position without prescribing edits.
func TestCheckMigrationAcceptsShiftedStartingDefinition(t *testing.T) {
	// Arrange
	_, root, tools := migrationCalls(t)
	c := Cases("ql9")[1]
	// This test grades the shifted position, so it uses normal diagnostics.
	c.Config = Config{}
	tools[1].Arguments["line"] = 2
	tools[2].Result = c.Files["pricing.py"]
	config, requests := checkRecording(t, c, root, tools)
	// Act
	err := Check(c, "ql9", config, requests)
	// Assert
	require.NoError(t, err)
}

// Missing navigation reports the required operation and phase instead of only a query count.
func TestCheckMigrationNamesMissingLookupAndPhase(t *testing.T) {
	for _, variant := range []struct {
		name      string
		omit      []int
		operation string
		phase     string
	}{
		{"only-initial-lookups", []int{8, 9}, "findReferences on pricing.py", "after editing"},
		{"missing-follow-up-references", []int{9}, "findReferences on pricing.py", "after editing"},
		{"missing-initial-definition", []int{0}, "goToDefinition on checkout.py", "before editing"},
		{"missing-initial-references", []int{1}, "findReferences on pricing.py", "before editing"},
		{"only-follow-up-lookups", []int{0, 1}, "goToDefinition on checkout.py", "before editing"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			// Arrange
			c, root, tools := migrationCalls(t)
			var remaining []call
			for index, tool := range tools {
				if !slices.Contains(variant.omit, index) {
					remaining = append(remaining, tool)
				}
			}
			config, requests := checkRecording(t, c, root, remaining)
			// Act
			err := Check(c, "ql9", config, requests)
			// Assert
			require.ErrorContains(t, err, "missing "+variant.operation)
			require.ErrorContains(t, err, variant.phase)
		})
	}
}

// A post-edit definition lookup is the model's choice of position, so it isn't graded.
func TestCheckMigrationDoesNotGradeFollowUpDefinition(t *testing.T) {
	for _, variant := range []struct {
		name string
		line int
		omit bool
	}{
		{"omitted", 0, true},
		{"stale-position", 1, false},
	} {
		t.Run(variant.name, func(t *testing.T) {
			// Arrange
			c, root, tools := migrationCalls(t)
			if variant.omit {
				tools = append(append([]call{}, tools[:8]...), tools[9:]...)
			} else {
				tools[8] = navigation("goToDefinition", "checkout.py", variant.line)
			}
			config, requests := checkRecording(t, c, root, tools)
			// Act
			err := Check(c, "ql9", config, requests)
			// Assert
			require.NoError(t, err)
		})
	}
}
