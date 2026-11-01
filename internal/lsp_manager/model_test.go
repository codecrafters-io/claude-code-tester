package lsp_manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tool arguments use the editor coordinates observed in Claude Code 2.1.266.
func TestActionUsesEditorCoordinates(t *testing.T) {
	// Arrange
	action := Action{Operation: "hover", File: "pricing.py", Line: 0, Character: 4}
	// Act
	actual := action.arguments()
	// Assert
	require.Equal(t, map[string]any{"operation": "hover", "filePath": "pricing.py", "line": 1, "character": 5}, actual)
}

// Product operation names map to the protocol methods captured from the pinned CLI.
func TestProtocolMethodsMatchClaudeCode(t *testing.T) {
	// Arrange
	methods := map[string]string{"hover": "textDocument/hover", "goToDefinition": "textDocument/definition", "findReferences": "textDocument/references"}
	for operation, expected := range methods {
		t.Run(operation, func(t *testing.T) {
			// Act
			actual := protocolMethod(operation)
			// Assert
			require.Equal(t, expected, actual)
		})
	}
}

// The model chooses a repair only after the preceding Write delivers an error.
func TestScriptedModelRequiresDiagnosticBeforeRepair(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		result string
		status int
	}{
		{"error-feedback", "lsp_call_0", `{"diagnostics":[{"severity":1,"message":"server-specific type error"}]}`, http.StatusOK},
		{"empty-feedback", "lsp_call_0", `{"diagnostics":[]}`, http.StatusBadRequest},
		{"warning-only", "lsp_call_0", `{"diagnostics":[{"severity":2}]}`, http.StatusBadRequest},
		{"stale-tool-result", "older_call", `{"diagnostics":[{"severity":1}]}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			actions := []Action{{Operation: "write"}, {Operation: "write", File: "checkout.py", Content: "repaired", AfterDiagnostic: true}}
			result, err := json.Marshal(tc.result)
			require.NoError(t, err)
			body := fmt.Sprintf(`{"messages":[{"role":"tool","tool_call_id":%q,"content":%s}],"tools":[{"function":{"name":"Write","parameters":{"properties":{"content":{},"file_path":{}}}}}]}`, tc.id, result)
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			response := httptest.NewRecorder()
			// Act
			ScriptedModel(actions).ServeHTTP(response, request)
			// Assert
			require.Equal(t, tc.status, response.Code, response.Body.String())
			if tc.status == http.StatusOK {
				require.Contains(t, response.Body.String(), "repaired")
			} else {
				require.NotContains(t, response.Body.String(), "repaired")
			}
		})
	}
}
