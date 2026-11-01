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
	WantTypes              []string
	WantDiagnosticVersions []int
}

func query(operation string) Action {
	return Action{Operation: operation, File: "pricing.py", Character: 4}
}

func Cases(slug string) []Case {
	files := Project(false)
	initial := files["pricing.py"]
	changed := strings.ReplaceAll(initial, "int", "str")
	// quantity stays integer, so the function can still multiply by the count.
	changed = strings.Replace(changed, "quantity: str", "quantity: int", 1)
	hover := query("hover")
	references := query("findReferences")
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
		prompt := "Use LSP hover on pricing.py at line 1, character 5. Report its documentation word."
		return []Case{
			{Name: "server-only-documentation", Prompt: prompt, Actions: []Action{hover}},
			{Name: "null-hover", Prompt: prompt, Actions: []Action{hover}, Config: Config{Hover: "null"}},
			{Name: "hover-error", Prompt: prompt, Actions: []Action{hover}, Config: Config{Hover: "error"}},
			{Name: "reuse-open-document", Prompt: prompt + " Ask for hover at that position again.", Actions: []Action{hover, hover}, Config: Config{Fragmented: true}},
		}
	case "df3":
		overloaded := Project(true)
		overloaded["pricing.pyi"] = "\n\ndef total(price: int, quantity: int) -> int: ...\n"
		definition := Action{Operation: "goToDefinition", File: "checkout.py", Line: 2, Character: 9}
		prompt := "Use LSP goToDefinition on checkout.py at line 3, character 10. Report the definition location."
		return []Case{
			{Name: "imported-alias", Prompt: prompt, Actions: []Action{definition}, Files: Project(true)},
			{Name: "multiple-locations", Prompt: prompt, Actions: []Action{definition}, Files: overloaded, Config: Config{Definition: "array"}},
			{Name: "missing-definition", Prompt: prompt, Actions: []Action{definition}, Files: Project(true), Config: Config{Definition: "null"}},
		}
	case "rf4":
		prompt := "Use LSP findReferences on pricing.py at line 1, character 5. Report every returned location."
		minimal := Project(false)
		delete(minimal, "reports.py")
		return []Case{
			{Name: "aliases-and-declaration", Prompt: prompt, Actions: []Action{references}, Files: minimal},
			{Name: "multiple-files-and-unrelated-names", Prompt: prompt, Actions: []Action{references}},
			{Name: "no-references", Prompt: prompt, Actions: []Action{references}, Config: Config{EmptyReferences: true}},
		}
	case "sy5":
		write := Action{Operation: "write", File: "pricing.py", Content: changed}
		restore := write
		restore.Content = initial
		prompt := "Hover on pricing.py at line 1, character 5. Use Write to change total to accept price: str and return str. Hover at the same position again."
		return []Case{
			{Name: "query-after-write", Prompt: prompt, Actions: []Action{hover, write, hover}, WantTypes: []string{"int", "str"}},
			{Name: "successive-versions", Prompt: prompt + " Then use Write to restore the original int types and hover again.", Actions: []Action{hover, write, hover, restore, hover}, WantTypes: []string{"int", "str", "int"}},
			{Name: "write-before-open", Prompt: "Use Write to change pricing.total to accept price: str and return str, then use LSP hover at line 1, character 5.", Actions: []Action{write, hover}, WantTypes: []string{"str"}},
			{Name: "failed-write", Prompt: "Hover on pricing.py at line 1, character 5. Try Write on the directory . with content invalid, then hover on pricing.py again.", Actions: []Action{hover, {Operation: "write", File: ".", Content: "invalid"}, hover}, WantTypes: []string{"int", "int"}},
		}
	case "dg6":
		valid := Project(false)
		valid["checkout.py"] = writeGood.Content
		writeGood.AfterDiagnostic = true
		read := Action{Operation: "read", File: "checkout.py"}
		definition := Action{Operation: "goToDefinition", File: "checkout.py", Line: 4, Character: 11}
		prompt := `Read checkout.py and use LSP goToDefinition on its invoice_total call at line 5, character 12. Use Write to replace integer 250 with string "250". Use the diagnostic feedback to repair the argument with Write, then report whether the error cleared.`
		actions := []Action{read, definition, writeBad, writeGood}
		return []Case{
			{Name: "invalid-edit-then-repair", Prompt: prompt, Actions: actions, Files: valid, WantDiagnosticVersions: []int{2, 3}},
			{Name: "unversioned-publications", Prompt: prompt, Actions: actions, Files: valid, Config: Config{UnversionedDiagnostics: true}, WantDiagnosticVersions: []int{2, 3}},
			{Name: "older-version-after-repair", Prompt: prompt, Actions: actions, Files: valid, Config: Config{StaleDiagnostics: true}, WantDiagnosticVersions: []int{2, 3}},
			{Name: "missing-publication-is-an-error", Prompt: `Read checkout.py, then use Write to change integer 250 to string "250". Report an error if diagnostic feedback never arrives.`, Actions: []Action{read, writeBad}, Files: valid, Config: Config{MissingDiagnostics: true}},
		}

	}
	return nil
}
