package lsp_manager

import "strings"

type Case struct {
	Name                   string
	Prompt                 string
	Actions                []Action
	Config                 Config
	Files                  map[string]string
	NoConfig               bool
	Failure                bool
	Rename                 bool
	WantTypes              []string
	WantDiagnosticVersions []int
}

func query(operation string) Action {
	return Action{Operation: operation, File: "pricing.py", Character: 4}
}

func Cases(slug string) []Case {
	files := Project(false, false)
	initial := files["pricing.py"]
	changed := strings.ReplaceAll(initial, "int", "str")
	// quantity stays integer, so the function can still multiply by the count.
	changed = strings.Replace(changed, "quantity: str", "quantity: int", 1)
	hover := query("hover")
	references := query("references")
	rename := query("rename")
	rename.NewName = "total_cents"
	diagnostics := Action{Operation: "diagnostics", File: "checkout.py"}
	writeBad := Action{Operation: "write", File: "checkout.py", Content: files["checkout.py"]}
	writeGood := writeBad
	writeGood.Content = strings.Replace(writeBad.Content, `"250"`, `250`, 1)
	ready := "Reply with ready."
	switch slug {
	case "ls1":
		return []Case{{Name: "initialize-project", Prompt: ready}, {Name: "without-configuration", Prompt: ready, NoConfig: true}}
	case "ir2":
		return []Case{
			{Name: "fragmented-reply-and-notification", Prompt: ready, Config: Config{Fragmented: true}},
			{Name: "initialization-error", Prompt: ready, Config: Config{Fault: "initialize-error"}, Failure: true},
			{Name: "incomplete-response", Prompt: ready, Config: Config{Fault: "incomplete"}, Failure: true},
		}
	case "hv2":
		prompt := "Use LSP hover on pricing.py at line 0, character 4. Report its documentation word."
		return []Case{
			{Name: "server-only-documentation", Prompt: prompt, Actions: []Action{hover}},
			{Name: "null-hover", Prompt: prompt, Actions: []Action{hover}, Config: Config{Hover: "null"}},
			{Name: "hover-error", Prompt: prompt, Actions: []Action{hover}, Config: Config{Hover: "error"}},
			{Name: "reuse-open-document", Prompt: prompt + " Ask for hover at that position again.", Actions: []Action{hover, hover}, Config: Config{Fragmented: true}},
		}
	case "df3":
		overloaded := Project(false, true)
		overloaded["pricing.pyi"] = "\n\ndef total(price: int, quantity: int) -> int: ...\n"
		definition := Action{Operation: "definition", File: "checkout.py", Line: 2, Character: 9}
		prompt := "Use LSP definition on checkout.py at line 2, character 9. Report the definition location."
		return []Case{
			{Name: "imported-alias", Prompt: prompt, Actions: []Action{definition}, Files: Project(false, true)},
			{Name: "multiple-locations", Prompt: prompt, Actions: []Action{definition}, Files: overloaded, Config: Config{Definition: "array"}},
			{Name: "missing-definition", Prompt: prompt, Actions: []Action{definition}, Files: Project(false, true), Config: Config{Definition: "null"}},
		}
	case "rf4":
		prompt := "Use LSP references on pricing.py at line 0, character 4, without the declaration. Report every returned location."
		minimal := Project(false, false)
		delete(minimal, "reports.py")
		withDeclaration := references
		withDeclaration.IncludeDeclaration = true
		return []Case{
			{Name: "aliases-and-unrelated-names", Prompt: prompt, Actions: []Action{references}, Files: minimal},
			{Name: "include-declaration", Prompt: "Use LSP references on pricing.py at line 0, character 4, including the declaration. Report every location.", Actions: []Action{withDeclaration}},
			{Name: "no-references", Prompt: prompt, Actions: []Action{references}, Config: Config{EmptyReferences: true}},
		}
	case "sy5":
		write := Action{Operation: "write", File: "pricing.py", Content: changed}
		restore := write
		restore.Content = initial
		prompt := "Hover on pricing.py at line 0, character 4. Use Write to change total to accept price: str and return str. Hover at the same position again."
		return []Case{
			{Name: "query-after-write", Prompt: prompt, Actions: []Action{hover, write, hover}, WantTypes: []string{"int", "str"}},
			{Name: "successive-versions", Prompt: prompt + " Then use Write to restore the original int types and hover again.", Actions: []Action{hover, write, hover, restore, hover}, WantTypes: []string{"int", "str", "int"}},
			{Name: "write-before-open", Prompt: "Use Write to change pricing.total to accept price: str and return str, then use LSP hover at line 0, character 4.", Actions: []Action{write, hover}, WantTypes: []string{"str"}},
			{Name: "failed-write", Prompt: "Hover on pricing.py at line 0, character 4. Try Write on the directory . with content invalid, then hover on pricing.py again.", Actions: []Action{hover, {Operation: "write", File: ".", Content: "invalid"}, hover}, WantTypes: []string{"int", "int"}},
		}
	case "dg6":
		valid := Project(false, false)
		valid["checkout.py"] = writeGood.Content
		prompt := `Check diagnostics for checkout.py. Use Write to make invoice_total receive "250" instead of 250, then fix that argument using Write. Check the diagnostics returned by each write.`
		return []Case{
			{Name: "invalid-edit-then-repair", Prompt: prompt, Actions: []Action{diagnostics, writeBad, writeGood}, Files: valid, WantDiagnosticVersions: []int{1, 2, 3}},
			{Name: "older-notification-after-repair", Prompt: prompt, Actions: []Action{diagnostics, writeBad, writeGood}, Files: valid, Config: Config{StaleDiagnostics: true}, WantDiagnosticVersions: []int{1, 2, 3}},
			{Name: "write-opens-document", Prompt: "Use Write to fix checkout.py by passing integer 250, and report the diagnostics returned by Write.", Actions: []Action{writeGood}, WantDiagnosticVersions: []int{1}},
			{Name: "missing-publication-is-an-error", Prompt: "Check LSP diagnostics for checkout.py. Report an error if none arrive.", Actions: []Action{diagnostics}, Config: Config{MissingDiagnostics: true}},
		}
	case "rn7":
		prompt := "Use LSP rename on pricing.py at line 0, character 4, with new name total_cents."
		unchanged := rename
		unchanged.NewName = "total"
		return []Case{
			{Name: "rename-preserves-alias-and-decoy", Prompt: prompt, Actions: []Action{rename}, Rename: true},
			{Name: "utf16-and-unsorted-edits", Prompt: prompt, Actions: []Action{rename}, Files: Project(true, false), Config: Config{ReverseEdits: true}, Rename: true},
			{Name: "no-rename-edits", Prompt: "Use LSP rename on pricing.py at line 0, character 4, keeping the name total.", Actions: []Action{unchanged}},
		}
	}
	return nil
}
