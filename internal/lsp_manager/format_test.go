package lsp_manager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// claudeCodeFormat renders a result the way Claude Code 2.1.318 shows it to the model.
func claudeCodeFormat(root string, raw json.RawMessage, operation string) string {
	found := locations(raw)
	if operation == "goToDefinition" {
		if len(found) == 0 {
			return "No definition found."
		}
		return "Defined in " + relativeFile(root, found[0].URI) + ":" + onePosition(found[0].Range.Start)
	}
	if len(found) == 0 {
		return "No references found."
	}
	byFile := map[string][]string{}
	for _, location := range found {
		file := relativeFile(root, location.URI)
		byFile[file] = append(byFile[file], "  Line "+onePosition(location.Range.Start))
	}
	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)
	text := fmt.Sprintf("Found %d references across %d files:\n", len(found), len(files))
	for _, file := range files {
		text += "\n" + file + ":\n" + strings.Join(byFile[file], "\n") + "\n"
	}
	return text
}

func location(root, file string, line, character int) Location {
	return Location{URI: URI(filepath.Join(root, file)), Range: Range{Start: Position{Line: line, Character: character}, End: Position{Line: line, Character: character + 5}}}
}

// Any wording passes if it names each location's file and 1-based position.
func TestFormattedResultAcceptsClaudeCodeText(t *testing.T) {
	root := t.TempDir()
	definition, _ := json.Marshal(location(root, "pricing.py", 0, 4))
	references, _ := json.Marshal([]Location{location(root, "pricing.py", 0, 4), location(root, "checkout.py", 4, 11)})
	for _, variant := range []struct {
		name    string
		content string
		raw     json.RawMessage
	}{
		{"definition", "Defined in pricing.py:1:5", definition},
		{"other-wording", "pricing.total is at pricing.py, 1:5", definition},
		{"references", claudeCodeFormat(root, references, "findReferences"), references},
		{"no-references", "No references found.", json.RawMessage(`[]`)},
	} {
		t.Run(variant.name, func(t *testing.T) {
			// Act
			err := checkFormattedResult(variant.content, variant.raw, root)
			// Assert
			require.NoError(t, err)
		})
	}
}

// Raw JSON, 0-based positions, a missing location, or an empty result fail with what to fix.
func TestFormattedResultRejectsWrongText(t *testing.T) {
	root := t.TempDir()
	definition, _ := json.Marshal(location(root, "pricing.py", 0, 4))
	references, _ := json.Marshal([]Location{location(root, "pricing.py", 0, 4), location(root, "checkout.py", 4, 11)})
	for _, variant := range []struct {
		name    string
		content string
		raw     json.RawMessage
		message string
	}{
		{"raw-json", string(definition), definition, "still holds the server's raw JSON"},
		{"zero-based", "Defined in pricing.py:0:4", definition, "should list pricing.py at 1:5"},
		{"missing-location", "Found 1 references across 1 files:\n\npricing.py:\n  Line 1:5", references, "should list checkout.py at 5:12"},
		{"empty", " ", json.RawMessage(`[]`), "the tool result is empty"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			// Act
			err := checkFormattedResult(variant.content, variant.raw, root)
			// Assert
			require.ErrorContains(t, err, variant.message)
		})
	}
}
