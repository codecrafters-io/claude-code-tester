package lsp_manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tool arguments use the editor coordinates observed in Claude Code 2.1.266.
func TestActionUsesEditorCoordinates(t *testing.T) {
	// Arrange
	action := Action{Operation: "goToDefinition", File: "pricing.py", Line: 0, Character: 4}
	// Act
	actual := action.arguments()
	// Assert
	require.Equal(t, map[string]any{"operation": "goToDefinition", "filePath": "pricing.py", "line": 1, "character": 5}, actual)
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

// A Bash action uses the advertised shell tool rather than the LSP tool.
func TestScriptedModelRequestsBash(t *testing.T) {
	// Arrange
	action := Action{Operation: "bash", Content: "python3 -m unittest -v test_invoice.py"}
	body := `{"messages":[],"tools":[{"function":{"name":"LSP","parameters":{"properties":{"operation":{}}}}},{"function":{"name":"Bash","parameters":{"properties":{"command":{}}}}}]}`
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	response := httptest.NewRecorder()
	// Act
	ScriptedModel([]Action{action}).ServeHTTP(response, request)
	// Assert
	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct{ Name, Arguments string } `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	function := payload.Choices[0].Message.ToolCalls[0].Function
	require.Equal(t, "Bash", function.Name)
	require.JSONEq(t, `{"command":"python3 -m unittest -v test_invoice.py"}`, function.Arguments)
}
