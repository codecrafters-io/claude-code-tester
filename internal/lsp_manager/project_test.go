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
	state.Documents[URI(filepath.Join(state.Root, "pricing.py"))] = Document{Text: PriceAPI}
	document := Document{URI: uri, Version: 1, Text: strings.Replace(state.text("checkout.py"), "    return invoice_total(250, 2)", "    amount = invoice_total(250, 2)\n    return amount", 1)}
	// Act
	before := state.diagnostics(document)
	document.Text = strings.Replace(document.Text, "return amount", `return amount["cents"]`, 1)
	after := state.diagnostics(document)
	// Assert
	require.Len(t, before, 1)
	require.Equal(t, Range{Start: Position{Line: 5, Character: 11}, End: Position{Line: 5, Character: 17}}, before[0]["range"])
	require.Empty(t, after)
	require.NotNil(t, after)
}

// Definition lookup follows the synchronized import rather than the old disk text.
func TestDefinitionUsesSynchronizedImport(t *testing.T) {
	// Arrange
	state := seededProject(t)
	uri := URI(filepath.Join(state.Root, "checkout.py"))
	state.Documents[uri] = Document{URI: uri, Version: 2, Text: strings.Replace(state.text("checkout.py"), "from pricing", "from discounts", 1)}
	// Act
	location := state.definition()
	// Assert
	require.Equal(t, Location{URI: URI(filepath.Join(state.Root, "discounts.py")), Range: Range{Start: Position{Line: 0, Character: 4}, End: Position{Line: 0, Character: 9}}}, location)
}
