package lsp_manager

import "strings"

type Case struct {
	Name                   string
	Prompt                 string
	Actions                []Action
	Config                 Config
	Files                  map[string]string
	WantDefinitions        []string
	WantDiagnosticVersions []int
}

func query(operation string) Action {
	return Action{Operation: operation, File: "pricing.py", Character: 4}
}

func Cases(slug string) []Case {
	files := Project(false)
	definition := Action{Operation: "goToDefinition", File: "checkout.py", Line: 4, Character: 11}
	references := query("findReferences")
	switch slug {
	case "lc1":
		return []Case{
			{Name: "connected-tool-advertised", Prompt: "Use only Read to inspect checkout.py and pricing.py. Explain how checkout uses the pricing function.", Actions: []Action{{Operation: "read", File: "checkout.py"}, {Operation: "read", File: "pricing.py"}}},
		}
	case "df3":
		prompt := "Use LSP goToDefinition on checkout.py at line 5, character 12. Read the returned implementation and explain which total is called; preview.total is unrelated."
		overloaded := Project(false)
		overloaded["pricing.pyi"] = "\n\ndef total(price: int, quantity: int) -> int: ...\n"
		return []Case{
			{Name: "resolve-and-read-implementation", Prompt: prompt, Actions: []Action{definition, {Operation: "read", File: "pricing.py"}}, Config: Config{Fragmented: true}},
			{Name: "multiple-definitions", Prompt: prompt, Actions: []Action{definition}, Files: overloaded, Config: Config{Definition: "array"}},
			{Name: "missing-definition", Prompt: prompt, Actions: []Action{definition}, Config: Config{Definition: "null"}},
			{Name: "definition-error", Prompt: prompt, Actions: []Action{definition}, Config: Config{Definition: "error"}},
		}
	case "rf4":
		prompt := "Use LSP findReferences on pricing.py at line 1, character 5. Identify every affected caller before changing its return type. Preserve the returned locations; preview.total and comments are unrelated."
		minimal := Project(false)
		delete(minimal, "reports.py")
		return []Case{
			{Name: "aliased-caller-and-declaration", Prompt: prompt, Actions: []Action{references}, Files: minimal},
			{Name: "affected-callers-across-files", Prompt: prompt, Actions: []Action{references}},
			{Name: "no-references", Prompt: prompt, Actions: []Action{references}, Config: Config{EmptyReferences: true}},
		}
	case "sy5":
		changed := strings.Replace(files["checkout.py"], "from pricing", "from discounts", 1)
		write := Action{Operation: "write", File: "checkout.py", Content: changed}
		restore := write
		restore.Content = files["checkout.py"]
		prompt := "Use LSP goToDefinition on checkout.py at line 5, character 12. Use Write to change its import from pricing to discounts, preserving the invoice_total alias. Repeat definition lookup at the same position and report the new target."
		return []Case{
			{Name: "definition-follows-edited-import", Prompt: prompt, Actions: []Action{definition, write, definition}, WantDefinitions: []string{"pricing.py", "discounts.py"}},
			{Name: "successive-document-versions", Prompt: prompt + " Restore the pricing import using Write and repeat the lookup.", Actions: []Action{definition, write, definition, restore, definition}, WantDefinitions: []string{"pricing.py", "discounts.py", "pricing.py"}},
			{Name: "write-before-open", Prompt: "Use Write to change checkout.py's import from pricing to discounts, then look up invoice_total at line 5, character 12.", Actions: []Action{write, definition}, WantDefinitions: []string{"discounts.py"}},
			{Name: "failed-write-keeps-analysis", Prompt: "Look up invoice_total in checkout.py at line 5, character 12. Try Write on the directory . with content invalid. Repeat the lookup.", Actions: []Action{definition, {Operation: "write", File: ".", Content: "invalid"}, definition}, WantDefinitions: []string{"pricing.py", "pricing.py"}},
		}
	case "dg6":
		api := Action{Operation: "write", File: "pricing.py", Content: PriceAPI}
		reports := Action{Operation: "write", File: "reports.py", Content: strings.ReplaceAll(strings.ReplaceAll(files["reports.py"], "total(250, 2)", `total(250, 2)["cents"]`), "total(100, 1)", `total(100, 1)["cents"]`)}
		// Replay a partial migration: the caller stores the new object but still returns it as int.
		partial := strings.Replace(files["checkout.py"], "    return invoice_total(250, 2)", "    amount = invoice_total(250, 2)\n    return amount", 1)
		bad := Action{Operation: "write", File: "checkout.py", Content: partial}
		repair := bad
		repair.Content = strings.Replace(partial, "return amount", `return amount["cents"]`, 1)
		repair.AfterDiagnostic = true
		prompt := `Migrate pricing.total from integer cents to a Price TypedDict containing cents. Use LSP definition on checkout.py at line 5, character 12 and references on pricing.py at line 1, character 5 to inspect affected callers. Update reporting to read cents. Replay the supplied partial checkout migration (store invoice_total(250, 2) in amount, then return amount), inspect its Write feedback, and repair the caller using that feedback. Report whether the error cleared. The fixed first attempt makes this a controlled before/after demonstration.`
		actions := []Action{definition, references, api, reports, bad, repair}
		return []Case{
			{Name: "api-change-missed-caller-repaired", Prompt: prompt, Actions: actions, WantDiagnosticVersions: []int{2, 3}},
			{Name: "unversioned-publications", Prompt: prompt, Actions: actions, Config: Config{UnversionedDiagnostics: true}, WantDiagnosticVersions: []int{2, 3}},
			{Name: "older-version-after-repair", Prompt: prompt, Actions: actions, Config: Config{StaleDiagnostics: true}, WantDiagnosticVersions: []int{2, 3}},
			{Name: "missing-feedback-is-an-error", Prompt: "Use Write to migrate pricing.total to a Price TypedDict with cents. Report a feedback error if the server never publishes diagnostics.", Actions: []Action{api}, Config: Config{MissingDiagnostics: true}},
		}
	}
	return nil
}
