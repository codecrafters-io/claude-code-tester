package lsp_manager

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type oneByteReader struct{ io.Reader }

func (r oneByteReader) Read(p []byte) (int, error) { return r.Reader.Read(p[:1]) }

// Partial reads preserve UTF-8 bodies and the boundary between adjacent frames.
func TestReadMessageAcrossPartialReads(t *testing.T) {
	// Arrange
	var stream bytes.Buffer
	require.NoError(t, WriteMessage(&stream, map[string]any{"jsonrpc": "2.0", "method": "window/logMessage", "params": map[string]any{"message": "caf\u00e9"}}, true))
	require.NoError(t, WriteMessage(&stream, map[string]any{"jsonrpc": "2.0", "id": 7, "result": nil}, false))
	reader := bufio.NewReader(oneByteReader{&stream})
	// Act
	notification, err := ReadMessage(reader)
	require.NoError(t, err)
	response, err := ReadMessage(reader)
	// Assert
	require.NoError(t, err)
	require.Equal(t, "window/logMessage", notification.Method)
	require.Contains(t, string(notification.Params), "caf\u00e9")
	require.JSONEq(t, `7`, string(response.ID))
	require.JSONEq(t, `null`, string(response.Result))
}

// Invalid lengths and incomplete bodies fail instead of yielding a partial message.
func TestReadMessageRejectsMalformedFrames(t *testing.T) {
	// Arrange
	frames := []string{"Content-Length: -1\r\n\r\n", "Content-Length: 4194305\r\n\r\n", "Content-Length: nope\r\n\r\n", "Content-Length: 30\r\n\r\n{}", "\r\n", "Content-Length: 2\r\n\r\n{}"}
	for _, frame := range frames {
		// Act
		_, err := ReadMessage(bufio.NewReader(strings.NewReader(frame)))
		// Assert
		require.Error(t, err, "frame %q", frame)
	}
}

// LSP columns count UTF-16 code units after an astral character.
func TestPositionUsesUTF16(t *testing.T) {
	// Arrange
	text := "header\n\U0001f680 total"
	offset := strings.Index(text, "total")
	// Act
	actual := textRange(text, offset, len(text))
	// Assert
	require.Equal(t, Range{Start: Position{Line: 1, Character: 3}, End: Position{Line: 1, Character: 8}}, actual)
}

// File URI conversion preserves spaces and URI-reserved path characters.
func TestFileURIRoundTrip(t *testing.T) {
	// Arrange
	path := "/tmp/invoice project/#cost%.py"
	// Act
	uri := URI(path)
	actual, err := Path(uri)
	// Assert
	require.NoError(t, err)
	require.Equal(t, "file:///tmp/invoice%20project/%23cost%25.py", uri)
	require.Equal(t, path, actual)
}

// Result comparison preserves array cardinality, even when results are wrapped.
func TestContainsJSONPreservesLocations(t *testing.T) {
	// Arrange
	expected := []any{map[string]any{"uri": "file:///a.py"}, map[string]any{"uri": "file:///b.py"}}
	// Act and Assert
	require.True(t, ContainsJSON(`{"result":[{"uri":"file:///b.py"},{"uri":"file:///a.py"}]}`, expected))
	require.False(t, ContainsJSON(`[{"uri":"file:///a.py"}]`, expected))
	require.False(t, ContainsJSON(`[{"uri":"file:///a.py"},{"uri":"file:///b.py"},{"uri":"file:///decoy.py"}]`, expected))
	require.False(t, ContainsJSON(`{"version":"3","diagnostics":[]}`, map[string]any{"version": 3, "diagnostics": []any{}}))
}

// Tool result lookup keeps the first result for its own call ID, not a later history entry.
func TestToolResultsKeepsCallBoundaries(t *testing.T) {
	// Arrange
	requests := []string{`{"messages":[{"role":"tool","tool_call_id":"old","content":"old diagnostic"},{"role":"tool","tool_call_id":"new","content":"cleared"}]}`, `{"messages":[{"role":"tool","tool_call_id":"new","content":"rewritten"}]}`}
	// Act
	results := ToolResults(requests)
	// Assert
	require.Equal(t, map[string]string{"old": "old diagnostic", "new": "cleared"}, results)
}

// Empty protocol results survive JSON decoding as arrays rather than null.
func TestEmptyReferencesIsAnArray(t *testing.T) {
	// Arrange
	state := projectState{Root: t.TempDir(), Documents: map[string]Document{}}
	// Act
	encoded, err := json.Marshal(state.references(false))
	// Assert
	require.NoError(t, err)
	require.Equal(t, "[]", string(encoded))
}

// Product operation names map to the protocol methods captured from the pinned CLI.
func TestProtocolMethodsMatchClaudeCode(t *testing.T) {
	// Arrange
	methods := map[string]string{"goToDefinition": "textDocument/definition", "findReferences": "textDocument/references"}
	for operation, expected := range methods {
		t.Run(operation, func(t *testing.T) {
			// Act
			actual := protocolMethod(operation)
			// Assert
			require.Equal(t, expected, actual)
		})
	}
}
