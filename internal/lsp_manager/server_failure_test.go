package lsp_manager

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A protocol error the server stops on is logged, so the stage can report it
// instead of the program's own error about the server exiting.
func TestServeLogsTheErrorItStopsOn(t *testing.T) {
	// Arrange
	config := Config{Root: t.TempDir(), Log: filepath.Join(t.TempDir(), "events.jsonl")}
	var input bytes.Buffer
	require.NoError(t, WriteMessage(&input, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{}}, false))
	// Act
	err := Serve(config, &input, io.Discard)
	// Assert
	require.Error(t, err)
	require.Equal(t, err.Error(), ServerFailure(config.Log))
}

// A server that shuts down cleanly reports no failure.
func TestServerFailureIsEmptyAfterCleanExit(t *testing.T) {
	// Arrange
	config := Config{Root: t.TempDir(), Log: filepath.Join(t.TempDir(), "events.jsonl")}
	// Act
	err := Serve(config, &bytes.Buffer{}, io.Discard)
	// Assert
	require.NoError(t, err)
	require.Empty(t, ServerFailure(config.Log))
}
