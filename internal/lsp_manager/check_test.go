package lsp_manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The checker rejects an invalid initialize even if the client ignores its error reply.
func TestCheckRejectsInvalidInitialize(t *testing.T) {
	// Arrange
	config := Config{Log: filepath.Join(t.TempDir(), "events.jsonl")}
	require.NoError(t, appendEvent(config.Log, "received", Message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize"}, ""))
	require.NoError(t, appendEvent(config.Log, "sent", Message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Error: json.RawMessage(`{"code":-32602,"message":"wrong processId"}`)}, ""))
	require.NoError(t, appendEvent(config.Log, "ended", Message{}, ""))
	// Act
	err := Check(Case{}, "gz6", config, []string{`{"messages":[]}`})
	// Assert
	require.ErrorContains(t, err, "invalid initialization request")
}

// Restarting a server cannot substitute for synchronizing its open documents.
func TestCheckRejectsRestartedServer(t *testing.T) {
	// Arrange
	config := Config{Log: filepath.Join(t.TempDir(), "events.jsonl")}
	first, _ := json.Marshal(Event{Direction: "received", PID: 1, Message: Message{Method: "initialize"}})
	second, _ := json.Marshal(Event{Direction: "received", PID: 2, Message: Message{Method: "initialize"}})
	require.NoError(t, os.WriteFile(config.Log, append(append(first, '\n'), second...), 0600))
	// Act
	err := Check(Case{}, "bn2", config, []string{"{}"})
	// Assert
	require.ErrorContains(t, err, "one language-server process")
}

// A transport that closes stdin can clean up without a separate shutdown exchange.
func TestCheckAcceptsEOFClosingAfterInitialization(t *testing.T) {
	// Arrange
	config := Config{Root: t.TempDir(), Log: filepath.Join(t.TempDir(), "events.jsonl")}
	require.NoError(t, appendEvent(config.Log, "received", Message{Method: "initialize", ID: json.RawMessage(`1`)}, ""))
	require.NoError(t, appendEvent(config.Log, "sent", Message{ID: json.RawMessage(`1`), Result: json.RawMessage(`{"capabilities":{}}`)}, ""))
	require.NoError(t, appendEvent(config.Log, "received", Message{Method: "initialized"}, ""))
	require.NoError(t, appendEvent(config.Log, "ended", Message{}, ""))
	// Act
	err := Check(Case{}, "gz6", config, []string{`{"messages":[]}`})
	// Assert
	require.NoError(t, err)
}

// An initialized server log with no document traffic, for checks on tool-call accounting.
func initializedLog(t *testing.T, extra ...Message) Config {
	t.Helper()
	config := Config{Root: t.TempDir(), Log: filepath.Join(t.TempDir(), "events.jsonl")}
	require.NoError(t, appendEvent(config.Log, "received", Message{Method: "initialize", ID: json.RawMessage(`1`)}, ""))
	require.NoError(t, appendEvent(config.Log, "sent", Message{ID: json.RawMessage(`1`), Result: json.RawMessage(`{"capabilities":{}}`)}, ""))
	require.NoError(t, appendEvent(config.Log, "received", Message{Method: "initialized"}, ""))
	for _, message := range extra {
		require.NoError(t, appendEvent(config.Log, "received", message, ""))
	}
	require.NoError(t, appendEvent(config.Log, "ended", Message{}, ""))
	return config
}

// An LSP tool call that never reaches the server says which request is missing.
func TestCheckNamesAMissingServerRequest(t *testing.T) {
	// Arrange
	config := initializedLog(t)
	request := `{"messages":[{"role":"assistant","tool_calls":[{"id":"lsp-call","function":{"arguments":"{\"operation\":\"goToDefinition\",\"filePath\":\"checkout.py\",\"line\":5,\"character\":12}"}}]}]}`
	// Act
	err := Check(Case{}, "gz6", config, []string{request})
	// Assert
	require.ErrorContains(t, err, "the model made 1 LSP tool call(s), but your program sent 0 request(s) to the language server")
}

// A request the model never asked for is reported as an extra request.
func TestCheckNamesAnExtraServerRequest(t *testing.T) {
	// Arrange
	config := initializedLog(t, Message{Method: "textDocument/definition", ID: json.RawMessage(`2`)})
	// Act
	err := Check(Case{}, "gz6", config, []string{`{"messages":[]}`})
	// Assert
	require.ErrorContains(t, err, "your program sent 1 request(s) to the language server, but the model made only 0 LSP tool call(s)")
}

func migratedProject(t *testing.T) (Case, string, []call) {
	t.Helper()
	c := Cases("ql9")[0]
	root := t.TempDir()
	files := map[string]string{}
	for name, text := range c.Files {
		files[name] = text
	}
	files["pricing.py"] = PriceAPI
	files["checkout.py"] = strings.Replace(files["checkout.py"], "invoice_total(250, 2)", `invoice_total(250, 2)["cents"]`, 1)
	files["reports.py"] = strings.ReplaceAll(strings.ReplaceAll(files["reports.py"], "total(250, 2)", `total(250, 2)["cents"]`), "total(100, 1)", `total(100, 1)["cents"]`)
	for name, text := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0644))
	}
	var tools []call
	for _, name := range []string{"pricing.py", "checkout.py", "reports.py"} {
		tools = append(tools,
			call{Arguments: map[string]any{"file_path": name}, Result: c.Files[name]},
			call{Arguments: map[string]any{"file_path": name, "content": files[name]}},
		)
	}
	tools = append(tools, call{Arguments: map[string]any{"command": "python3 -m unittest -v test_invoice.py"}, Result: "Ran 5 tests\nOK"})
	return c, root, tools
}

// Each file can be read immediately before its own first edit.
func TestMigrationAcceptsReadBeforeEachFilesWrite(t *testing.T) {
	// Arrange
	c, root, tools := migratedProject(t)
	// Act
	err := checkMigration(c, root, tools)
	// Assert
	require.NoError(t, err)
}

// Reading a file only after its first edit does not satisfy the pre-edit Read.
func TestMigrationRejectsReadAfterFirstWrite(t *testing.T) {
	// Arrange
	c, root, tools := migratedProject(t)
	tools[0], tools[1] = tools[1], tools[0]
	// Act
	err := checkMigration(c, root, tools)
	// Assert
	require.ErrorContains(t, err, "before editing pricing.py")
}

// Equivalent Price construction passes without requiring a predetermined source spelling.
func TestMigrationAcceptsEquivalentImplementation(t *testing.T) {
	// Arrange
	c, root, tools := migratedProject(t)
	text := strings.Replace(PriceAPI, `return {"cents": price * quantity}`, `return dict(cents=price * quantity)`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "pricing.py"), []byte(text), 0644))
	// Act
	err := checkMigration(c, root, tools)
	// Assert
	require.NoError(t, err)
}

// A caller left on the old API fails even when the recorded Bash output claims success.
func TestMigrationRejectsFabricatedTestOutput(t *testing.T) {
	// Arrange
	c, root, tools := migratedProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "reports.py"), []byte(c.Files["reports.py"]), 0644))
	// Act
	err := checkMigration(c, root, tools)
	// Assert
	require.ErrorContains(t, err, "migration behavior check failed")
}

// An unrelated file edit fails the migration check.
func TestMigrationRejectsUnrelatedEdits(t *testing.T) {
	// Arrange
	c, root, tools := migratedProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "preview.py"), []byte("# unrelated edit\n"), 0644))
	// Act
	err := checkMigration(c, root, tools)
	// Assert
	require.ErrorContains(t, err, "unrelated file preview.py must remain unchanged")
}

// A successful Bash exit code alone cannot replace its actual test output.
func TestMigrationRejectsMissingTestOutput(t *testing.T) {
	// Arrange
	c, root, tools := migratedProject(t)
	tools[len(tools)-1].Result = `{"exit_code":0}`
	// Act
	err := checkMigration(c, root, tools)
	// Assert
	require.ErrorContains(t, err, "return the passing test output")
}

// A placeholder reaches the model only when it answers its own tool call.
func TestPlaceholderRequiresMatchingCallID(t *testing.T) {
	// Arrange
	request := `{"messages":[{"role":"assistant","tool_calls":[{"id":"provider-call","function":{"arguments":"{\"operation\":\"goToDefinition\"}"}}]},{"role":"tool","tool_call_id":"unrelated-call","content":"LSP is not implemented yet"}]}`
	// Act
	err := checkPlaceholderResponse(calls([]string{request}))
	// Assert
	require.ErrorContains(t, err, `got no tool result; send a tool message with tool_call_id "provider-call"`)
}

// Any non-empty placeholder satisfies advertisement before navigation exists,
// in plain text or JSON-wrapped, whatever its wording.
func TestPlaceholderAcceptsUnimplementedNavigation(t *testing.T) {
	for _, result := range []string{"LSP is not implemented yet", `"LSP is not implemented yet"`, "LSP is not implemented", "TODO"} {
		t.Run(result, func(t *testing.T) {
			// Arrange
			tools := []call{{Arguments: map[string]any{"operation": "goToDefinition"}, Result: result}}
			// Act
			err := checkPlaceholderResponse(tools)
			// Assert
			require.NoError(t, err)
		})
	}
}

// A run where the model never calls the tool points at the advertisement.
func TestPlaceholderRequiresAnLSPCall(t *testing.T) {
	// Arrange
	tools := []call{{ID: "read-call", Arguments: map[string]any{"file_path": "checkout.py"}, Result: "contents"}}
	// Act
	err := checkPlaceholderResponse(tools)
	// Assert
	require.ErrorContains(t, err, "never called the LSP tool")
}

// Read must return the source contents rather than an unrelated success message.
func TestReadRejectsMissingSource(t *testing.T) {
	// Arrange
	tools := []call{{Arguments: map[string]any{"file_path": "pricing.py"}, Result: "File access disabled"}}
	// Act
	err := checkRead(t.TempDir(), "pricing.py", Project(false)["pricing.py"], tools)
	// Assert
	require.ErrorContains(t, err, "read pricing.py and return its complete contents")
}

// Every request must advertise the complete LSP schema.
func TestSetupRequiresCompleteSchemaInEveryRequest(t *testing.T) {
	const valid = `{"tools":[{"function":{"name":"LSP","parameters":{"properties":{"operation":{"type":"string","enum":["goToDefinition"]},"filePath":{"type":"string"},"line":{"type":"integer"},"character":{"type":"integer"}},"required":["operation","filePath","line","character"]}}}]}`
	for _, variant := range []struct {
		name     string
		requests []string
		want     string
	}{
		{"complete", []string{valid, valid}, ""},
		{"missing-tool", []string{`{"tools":[]}`}, "advertise the supplied LSP tool"},
		{"missing-later-tool", []string{valid, `{"tools":[]}`}, "advertise the supplied LSP tool"},
		{"incomplete-schema", []string{strings.Replace(valid, `"required":["operation","filePath","line","character"]`, `"required":["operation","filePath","line"]`, 1)}, "required character of type integer"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			// Arrange / Act
			err := checkSetupTools(variant.requests)
			// Assert
			if variant.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, variant.want)
			}
		})
	}
}
