package lsp_manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func seededProject(t *testing.T, unicodePrefix bool) projectState {
	t.Helper()
	root := t.TempDir()
	for name, text := range Project(unicodePrefix, false) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0644))
	}
	return projectState{Root: root, Documents: map[string]Document{}}
}

// Reference locations include an aliased call and omit unrelated names.
func TestReferencesResolveAlias(t *testing.T) {
	// Arrange
	state := seededProject(t, false)
	// Act
	locations := state.references(false, false)
	// Assert
	require.Len(t, locations, 5)
	require.Equal(t, Location{URI: URI(filepath.Join(state.Root, "checkout.py")), Range: Range{Start: Position{Line: 0, Character: 20}, End: Position{Line: 0, Character: 25}}}, locations[0])
	require.Equal(t, Location{URI: URI(filepath.Join(state.Root, "checkout.py")), Range: Range{Start: Position{Line: 4, Character: 11}, End: Position{Line: 4, Character: 24}}}, locations[1])
	for _, location := range locations {
		require.NotContains(t, location.URI, "preview.py")
	}
	require.Len(t, state.references(true, false), 6)
}

// Rename edits target the imported symbol while retaining the local alias.
func TestRenamePreservesAlias(t *testing.T) {
	// Arrange
	state := seededProject(t, true)
	// Act
	changes := state.rename("total_cents")
	// Assert
	require.Len(t, changes, 3)
	checkout := changes[URI(filepath.Join(state.Root, "checkout.py"))]
	require.Len(t, checkout, 1)
	require.Equal(t, Range{Start: Position{Line: 0, Character: 20}, End: Position{Line: 0, Character: 25}}, checkout[0].Range)
	reports := changes[URI(filepath.Join(state.Root, "reports.py"))]
	require.Len(t, reports, 3)
	require.Equal(t, Position{Line: 4, Character: 25}, reports[1].Range.Start)
	require.Equal(t, Position{Line: 4, Character: 41}, reports[2].Range.Start)
	require.NotContains(t, changes, URI(filepath.Join(state.Root, "preview.py")))
}

// Diagnostics use the open document and clear after the argument is repaired.
func TestDiagnosticsTrackOpenText(t *testing.T) {
	// Arrange
	state := seededProject(t, false)
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
	state := seededProject(t, false)
	uri := URI(filepath.Join(state.Root, "pricing.py"))
	state.Documents[uri] = Document{URI: uri, Version: 2, Text: "def total(price: str, quantity: int) -> str:\n    return price * quantity\n"}
	// Act
	signature := state.signature()
	// Assert
	require.Equal(t, "str", signature[2])
	require.Equal(t, "str", signature[3])
}
