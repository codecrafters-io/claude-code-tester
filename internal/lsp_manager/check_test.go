package lsp_manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Stage one rejects an invalid initialize even if the client ignores its error reply.
func TestCheckRejectsInvalidInitialize(t *testing.T) {
	// Arrange
	config := Config{Log: filepath.Join(t.TempDir(), "events.jsonl")}
	require.NoError(t, appendEvent(config.Log, "received", Message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize"}, ""))
	require.NoError(t, appendEvent(config.Log, "sent", Message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Error: json.RawMessage(`{"code":-32602,"message":"wrong processId"}`)}, ""))
	require.NoError(t, appendEvent(config.Log, "ended", Message{}, ""))
	// Act
	err := Check(Case{}, "ls1", config, []string{`{"messages":[]}`}, time.Now())
	// Assert
	require.ErrorContains(t, err, "invalid initialization request")
}

// Hover text can reach the model without retaining the LSP JSON envelope.
func TestHoverResultAllowsPlainText(t *testing.T) {
	// Arrange
	result := json.RawMessage(`{"contents":{"kind":"plaintext","value":"price: int; documentation word: orchard"}}`)
	// Act and Assert
	require.True(t, resultReachedModel("textDocument/hover", "price: int; documentation word: orchard", result))
	require.False(t, resultReachedModel("textDocument/hover", "price: int", result))
}

// Restarting a server cannot substitute for synchronizing its open documents.
func TestCheckRejectsRestartedServer(t *testing.T) {
	// Arrange
	config := Config{Log: filepath.Join(t.TempDir(), "events.jsonl")}
	first, _ := json.Marshal(Event{Direction: "received", PID: 1, Message: Message{Method: "initialize"}})
	second, _ := json.Marshal(Event{Direction: "received", PID: 2, Message: Message{Method: "initialize"}})
	require.NoError(t, os.WriteFile(config.Log, append(append(first, '\n'), second...), 0600))
	// Act
	err := Check(Case{}, "sy5", config, []string{"{}"}, time.Now())
	// Assert
	require.ErrorContains(t, err, "one language-server process")
}

// A successful invocation closes every document before shutting down its server.
func TestCheckRejectsUnclosedDocuments(t *testing.T) {
	// Arrange
	config := Config{Root: t.TempDir(), Log: filepath.Join(t.TempDir(), "events.jsonl")}
	params, _ := json.Marshal(map[string]any{"textDocument": Document{URI: URI(filepath.Join(config.Root, "pricing.py")), Version: 1, Text: "text"}})
	messages := []Message{
		{Method: "initialize", ID: json.RawMessage(`1`)},
		{ID: json.RawMessage(`1`), Result: json.RawMessage(`{"capabilities":{}}`)},
		{Method: "initialized"},
		{Method: "textDocument/didOpen", Params: params},
		{Method: "shutdown", ID: json.RawMessage(`2`)},
		{Method: "exit"},
	}
	for index, message := range messages {
		direction := "received"
		if index == 1 {
			direction = "sent"
		}
		require.NoError(t, appendEvent(config.Log, direction, message, ""))
	}
	require.NoError(t, appendEvent(config.Log, "ended", Message{}, ""))
	// Act
	err := Check(Case{}, "ir2", config, []string{"{}"}, time.Now())
	// Assert
	require.ErrorContains(t, err, "didClose")
}
