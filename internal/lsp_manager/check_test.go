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
