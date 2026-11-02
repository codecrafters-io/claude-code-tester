package lsp_manager

import "strings"

type Case struct {
	Name            string
	Prompt          string
	Actions         []Action
	Config          Config
	Files           map[string]string
	WantDefinitions []string
}

func query(operation string) Action {
	return Action{Operation: operation, File: "pricing.py", Character: 4}
}

func Cases(slug string) []Case {
	files := Project(false)
	definition := Action{Operation: "goToDefinition", File: "checkout.py", Line: 4, Character: 11}
	references := query("findReferences")
	switch slug {
	case "cx7":
		return []Case{
			{Name: "connected-tool-advertised", Prompt: "Use only Read to inspect checkout.py and pricing.py. Explain how checkout uses the pricing function.", Actions: []Action{{Operation: "read", File: "checkout.py"}, {Operation: "read", File: "pricing.py"}}},
		}
	case "gz6":
		prompt := "Use LSP goToDefinition on checkout.py at line 5, character 12. Read the returned implementation and explain which total is called; preview.total is unrelated."
		overloaded := Project(false)
		overloaded["pricing.pyi"] = "\n\ndef total(price: int, quantity: int) -> int: ...\n"
		return []Case{
			{Name: "resolve-and-read-implementation", Prompt: prompt, Actions: []Action{definition, {Operation: "read", File: "pricing.py"}}, Config: Config{Fragmented: true}},
			{Name: "multiple-definitions", Prompt: prompt, Actions: []Action{definition}, Files: overloaded, Config: Config{Definition: "array"}},
			{Name: "missing-definition", Prompt: prompt, Actions: []Action{definition}, Config: Config{Definition: "null"}},
			{Name: "definition-error", Prompt: prompt, Actions: []Action{definition}, Config: Config{Definition: "error"}},
		}
	case "rb6":
		prompt := "Use LSP findReferences on pricing.py at line 1, character 5. Identify every affected caller before changing its return type. Preserve the returned locations; preview.total and comments are unrelated."
		minimal := Project(false)
		delete(minimal, "reports.py")
		return []Case{
			{Name: "aliased-caller-and-declaration", Prompt: prompt, Actions: []Action{references}, Files: minimal},
			{Name: "affected-callers-across-files", Prompt: prompt, Actions: []Action{references}},
			{Name: "no-references", Prompt: prompt, Actions: []Action{references}, Config: Config{EmptyReferences: true}},
		}
	case "bn2":
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
	case "ql9":
		files["test_invoice.py"] = InvoiceTests
		api := Action{Operation: "write", File: "pricing.py", Content: PriceAPI}
		checkout := Action{Operation: "write", File: "checkout.py", Content: strings.Replace(files["checkout.py"], "invoice_total(250, 2)", `invoice_total(250, 2)["cents"]`, 1)}
		reports := Action{Operation: "write", File: "reports.py", Content: strings.ReplaceAll(strings.ReplaceAll(files["reports.py"], "total(250, 2)", `total(250, 2)["cents"]`), "total(100, 1)", `total(100, 1)["cents"]`)}
		updatedReferences := references
		updatedReferences.Line = 7
		prompt := `Change pricing.total to return a Price TypedDict with a cents field. Use LSP goToDefinition on checkout.py at line 5, character 12 and findReferences on pricing.py at line 1, character 5. Read the implementation and affected callers. Update checkout.py and reports.py so they still return integer cents. Don't modify test_invoice.py. Leave preview.total and other unrelated files unchanged. After editing, repeat definition lookup on checkout.py and findReferences on pricing.py at line 8, character 5 to check the current code. Run python3 -m unittest -v test_invoice.py and report the result.`
		actions := []Action{
			definition, {Operation: "read", File: "pricing.py"}, references,
			{Operation: "read", File: "checkout.py"}, {Operation: "read", File: "reports.py"},
			api, checkout, reports, definition, updatedReferences,
			{Operation: "bash", Content: "python3 -m unittest -v test_invoice.py"},
		}
		return []Case{
			{Name: "api-migration-across-files", Prompt: prompt, Actions: actions, Files: files},
			{Name: "migration-without-diagnostic-publications", Prompt: prompt, Actions: actions, Files: files, Config: Config{MissingDiagnostics: true}},
		}
	}
	return nil
}
