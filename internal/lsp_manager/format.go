package lsp_manager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// locations decodes a definition or references result: one location, a list of them, or null.
func locations(raw json.RawMessage) []Location {
	var many []Location
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	var one Location
	if json.Unmarshal(raw, &one) == nil && one.URI != "" {
		return []Location{one}
	}
	return nil
}

// relativeFile is the path Claude Code shows the model: relative to the project, with forward slashes.
func relativeFile(root, uri string) string {
	path, err := Path(uri)
	if err != nil {
		return uri
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

// onePosition is a zero-based LSP position shown 1-based, as Claude Code shows it ("1:5").
func onePosition(position Position) string {
	return fmt.Sprintf("%d:%d", position.Line+1, position.Character+1)
}

// checkFormattedResult accepts any wording that names each location's file and 1-based
// position, as Claude Code's "Defined in pricing.py:1:5" does. The server's raw JSON, with
// its zero-based positions, is rejected with a hint, since the model would reuse those numbers.
func checkFormattedResult(content string, raw json.RawMessage, root string) error {
	found := locations(raw)
	if len(found) == 0 {
		if strings.TrimSpace(content) == "" {
			return fmt.Errorf("the tool result is empty; say that no locations were found")
		}
		return nil
	}
	if strings.Contains(content, `"range"`) && strings.Contains(content, `"uri"`) {
		return fmt.Errorf("the tool result still holds the server's raw JSON; format each location as text with its file and 1-based line:character, such as %s:%s",
			relativeFile(root, found[0].URI), onePosition(found[0].Range.Start))
	}
	for _, location := range found {
		file, position := relativeFile(root, location.URI), onePosition(location.Range.Start)
		if !strings.Contains(content, file) || !strings.Contains(content, position) {
			return fmt.Errorf("the tool result should list %s at %s (1-based line:character); got %q", file, position, content)
		}
	}
	return nil
}
