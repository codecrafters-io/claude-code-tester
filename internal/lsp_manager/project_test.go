package lsp_manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func seededProject(t *testing.T) projectState {
	t.Helper()
	root := t.TempDir()
	for name, text := range Project(false) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0644))
	}
	return projectState{Root: root, Documents: map[string]Document{}}
}

// Reference locations include an aliased call and omit unrelated names.
func TestReferencesResolveAlias(t *testing.T) {
	// Arrange
	state := seededProject(t)
	// Act
	locations := state.references(false)
	// Assert
	require.Len(t, locations, 5)
	require.Equal(t, Location{URI: URI(filepath.Join(state.Root, "checkout.py")), Range: Range{Start: Position{Line: 0, Character: 20}, End: Position{Line: 0, Character: 25}}}, locations[0])
	require.Equal(t, Location{URI: URI(filepath.Join(state.Root, "checkout.py")), Range: Range{Start: Position{Line: 4, Character: 11}, End: Position{Line: 4, Character: 24}}}, locations[1])
	for _, location := range locations {
		require.NotContains(t, location.URI, "preview.py")
	}
	require.Len(t, state.references(true), 6)
}

// Diagnostics use the open document and clear after the argument is repaired.
func TestDiagnosticsTrackOpenText(t *testing.T) {
	// Arrange
	state := seededProject(t)
	uri := URI(filepath.Join(state.Root, "checkout.py"))
	document := Document{URI: uri, Version: 1, Text: state.text("checkout.py")}
	// Act
	before := state.diagnostics(document)
	document.Text = strings.Replace(document.Text, `"250"`, `250`, 1)
	after := state.diagnostics(document)
	// Assert
	require.Len(t, before, 1)
	require.Equal(t, Range{Start: Position{Line: 4, Character: 25}, End: Position{Line: 4, Character: 30}}, before[0]["range"])
	require.Empty(t, after)
	require.NotNil(t, after)
}

// An open document overrides disk text when calculating type information.
func TestSignatureUsesSynchronizedText(t *testing.T) {
	// Arrange
	state := seededProject(t)
	uri := URI(filepath.Join(state.Root, "pricing.py"))
	state.Documents[uri] = Document{URI: uri, Version: 2, Text: "def total(price: str, quantity: int) -> str:\n    return price * quantity\n"}
	// Act
	signature := state.signature()
	// Assert
	require.Equal(t, "str", signature[2])
	require.Equal(t, "str", signature[3])
}
