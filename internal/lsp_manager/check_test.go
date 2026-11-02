package lsp_manager

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	err := Check(Case{}, "df3", config, []string{`{"messages":[]}`})
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
	err := Check(Case{}, "sy5", config, []string{"{}"})
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
	err := Check(Case{}, "df3", config, []string{`{"messages":[]}`})
	// Assert
	require.NoError(t, err)
}
