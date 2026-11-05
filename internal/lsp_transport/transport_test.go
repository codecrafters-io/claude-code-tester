package lsp_transport_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codecrafters-io/claude-code-tester/internal/lsp_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/lsp_transport"
	"github.com/stretchr/testify/require"
)

// The subprocess runs the controlled stdio server for transport tests.
func TestTransportFixture(t *testing.T) {
	// Arrange
	encoded := os.Getenv("CODECRAFTERS_TRANSPORT_TEST_CONFIG")
	if encoded == "" {
		return
	}
	var config lsp_manager.Config
	require.NoError(t, json.Unmarshal([]byte(encoded), &config))
	// Act
	err := lsp_manager.Serve(config, os.Stdin, os.Stdout)
	// Assert
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func configuration(t *testing.T, fault string) (string, lsp_manager.Config) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(root)
	config := lsp_manager.Config{Root: root, Log: filepath.Join(root, "events.jsonl"), Fragmented: true, Fault: fault}
	data, err := json.Marshal(config)
	require.NoError(t, err)
	t.Setenv("CODECRAFTERS_TRANSPORT_TEST_CONFIG", string(data))
	for name, text := range lsp_manager.Project(false) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0644))
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	data, err = json.Marshal(map[string]any{"python": map[string]any{"command": executable, "args": []string{"-test.run=^TestTransportFixture$"}, "extensionToLanguage": map[string]string{".py": "python"}}})
	require.NoError(t, err)
	path := filepath.Join(root, ".lsp.json")
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path, config
}

func call(t *testing.T, url, action string, payload any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	response, err := http.Post(url+"/"+action, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer response.Body.Close()
	var result json.RawMessage
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	require.Equal(t, http.StatusOK, response.StatusCode, string(result))
	return result
}

// opened waits until the server has handled earlier notifications. A notify call returns
// before the server reads it, and the server checks didOpen against the file on disk, so a
// test must not edit the file until the response to a later request has arrived.
func opened(t *testing.T, url, uri string) {
	t.Helper()
	call(t, url, "request", map[string]any{"method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 0, "character": 0}}})
}

// The supplied helper reads fragmented frames and retains notifications during requests.
func TestTransportRetainsNotifications(t *testing.T) {
	// Arrange
	path, config := configuration(t, "")
	transport, err := lsp_transport.Start(path)
	require.NoError(t, err)
	t.Cleanup(transport.Close)
	text := lsp_manager.Project(false)["checkout.py"]
	uri := lsp_manager.URI(filepath.Join(config.Root, "checkout.py"))
	// Act
	call(t, transport.URL, "notify", map[string]any{"method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "python", "version": 1, "text": text}}})
	result := call(t, transport.URL, "request", map[string]any{"method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 4, "character": 11}}})
	batch := call(t, transport.URL, "notifications", map[string]int{"after": 0, "timeoutMs": 1000})
	transport.Close()
	events, eventErr := lsp_manager.Events(config.Log)
	// Assert
	require.Contains(t, string(result), "pricing.py")
	require.Contains(t, string(batch), "window/logMessage")
	require.Contains(t, string(batch), "textDocument/publishDiagnostics")
	require.NoError(t, eventErr)
	require.Equal(t, "ended", events[len(events)-1].Direction)
}

// Failed initialization stops the supplied helper before it advertises a usable URL.
func TestTransportRejectsFailedStartup(t *testing.T) {
	for _, fault := range []string{"initialize-error", "incomplete"} {
		t.Run(fault, func(t *testing.T) {
			// Arrange
			path, _ := configuration(t, fault)
			// Act
			transport, err := lsp_transport.Start(path)
			// Assert
			require.Error(t, err)
			require.Nil(t, transport)
		})
	}
}

// A symlinked configuration identifies the same physical project as document URIs.
func TestTransportResolvesProjectSymlink(t *testing.T) {
	// Arrange
	_, config := configuration(t, "")
	alias := filepath.Join(t.TempDir(), "project alias")
	require.NoError(t, os.Symlink(config.Root, alias))
	// Act
	transport, err := lsp_transport.Start(filepath.Join(alias, ".lsp.json"))
	// Assert
	require.NoError(t, err)
	transport.Close()
}

// The helper exposes the configured extensions without assuming a Python server.
func TestTransportExposesConfiguredLanguages(t *testing.T) {
	// Arrange
	path, _ := configuration(t, "")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var servers map[string]map[string]any
	require.NoError(t, json.Unmarshal(data, &servers))
	server := servers["python"]
	mapping := map[string]string{".ts": "typescript", ".tsx": "typescript"}
	server["extensionToLanguage"] = mapping
	data, err = json.Marshal(map[string]any{"typescript": server})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
	transport, err := lsp_transport.Start(path)
	require.NoError(t, err)
	t.Cleanup(transport.Close)

	// Act
	result := call(t, transport.URL, "configuration", map[string]any{})

	// Assert
	want, err := json.Marshal(map[string]any{"extensionToLanguage": mapping})
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(result))
}

// Missing language configuration is rejected instead of defaulting to Python.
func TestTransportRequiresLanguageMapping(t *testing.T) {
	// Arrange
	path, _ := configuration(t, "")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var servers map[string]map[string]any
	require.NoError(t, json.Unmarshal(data, &servers))
	delete(servers["python"], "extensionToLanguage")
	data, err = json.Marshal(servers)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	// Act
	transport, err := lsp_transport.Start(path)

	// Assert
	require.ErrorContains(t, err, "expected extensionToLanguage")
	require.Nil(t, transport)
}

// A lookup at the old call position stops resolving after an edit moves the call.
func TestTransportDefinitionUsesCurrentCallPosition(t *testing.T) {
	// Arrange
	path, config := configuration(t, "")
	transport, err := lsp_transport.Start(path)
	require.NoError(t, err)
	t.Cleanup(transport.Close)
	text := lsp_manager.Project(false)["checkout.py"]
	uri := lsp_manager.URI(filepath.Join(config.Root, "checkout.py"))
	call(t, transport.URL, "notify", map[string]any{"method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "python", "version": 1, "text": text}}})
	opened(t, transport.URL, uri)
	text = strings.Replace(text, "return invoice_total(250, 2)", `return (invoice_total(250, 2)["cents"])`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(config.Root, "checkout.py"), []byte(text), 0644))
	call(t, transport.URL, "notify", map[string]any{"method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []any{map[string]any{"text": text}}}})

	// Act
	stale := call(t, transport.URL, "request", map[string]any{"method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 4, "character": 11}}})
	current := call(t, transport.URL, "request", map[string]any{"method": "textDocument/definition", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 4, "character": 12}}})

	// Assert
	require.JSONEq(t, `null`, string(stale))
	require.Contains(t, string(current), "pricing.py")
}

// References at the old definition position stop resolving after the definition moves.
func TestTransportReferencesUseCurrentDefinitionPosition(t *testing.T) {
	// Arrange
	path, config := configuration(t, "")
	transport, err := lsp_transport.Start(path)
	require.NoError(t, err)
	t.Cleanup(transport.Close)
	text := lsp_manager.Project(false)["pricing.py"]
	uri := lsp_manager.URI(filepath.Join(config.Root, "pricing.py"))
	call(t, transport.URL, "notify", map[string]any{"method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "python", "version": 1, "text": text}}})
	opened(t, transport.URL, uri)
	require.NoError(t, os.WriteFile(filepath.Join(config.Root, "pricing.py"), []byte(lsp_manager.PriceAPI), 0644))
	call(t, transport.URL, "notify", map[string]any{"method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []any{map[string]any{"text": lsp_manager.PriceAPI}}}})

	// Act
	stale := call(t, transport.URL, "request", map[string]any{"method": "textDocument/references", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 0, "character": 4}, "context": map[string]bool{"includeDeclaration": true}}})
	current := call(t, transport.URL, "request", map[string]any{"method": "textDocument/references", "params": map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]int{"line": 7, "character": 4}, "context": map[string]bool{"includeDeclaration": true}}})

	// Assert
	require.JSONEq(t, `[]`, string(stale))
	var locations []lsp_manager.Location
	require.NoError(t, json.Unmarshal(current, &locations))
	require.Len(t, locations, 6)
}
