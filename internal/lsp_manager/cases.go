package lsp_manager

import "strings"

type Query struct {
	Operation    string
	File         string
	Line         int
	Character    int
	AtDefinition bool
	AtCall       bool
	AfterEdits   bool
}

type Case struct {
	Name            string
	Prompt          string
	Queries         []Query
	ReadFiles       []string
	FinalFiles      map[string]string
	Config          Config
	Files           map[string]string
	WantDefinitions []string
}

func Cases(slug string) []Case {
	files := Project(false)
	definition := Query{Operation: "goToDefinition", File: "checkout.py", Line: 4, Character: 11}
	references := Query{Operation: "findReferences", File: "pricing.py", Character: 4}
	switch slug {
	case "iy3":
		return []Case{
			{Name: "lsp-tool-call-and-response", Prompt: "Use LSP goToDefinition on checkout.py at line 5, character 12. If navigation is not implemented yet, report the tool response without retrying.", Queries: []Query{definition}},
		}
	case "gz6":
		prompt := "Use LSP goToDefinition on checkout.py at line 5, character 12. Read the returned implementation and explain which total is called; preview.total is unrelated."
		return []Case{
			{Name: "resolve-and-read-implementation", Prompt: prompt, Queries: []Query{definition}, ReadFiles: []string{"pricing.py"}},
		}
	case "rb6":
		prompt := "Use LSP findReferences on pricing.py at line 1, character 5, and list the callers that a change to pricing.total's return type would affect. Don't change any code. Preserve the returned locations; preview.total and comments are unrelated."
		return []Case{
			{Name: "affected-callers-across-files", Prompt: prompt, Queries: []Query{references}},
			{Name: "no-references", Prompt: prompt, Queries: []Query{references}, Config: Config{EmptyReferences: true}},
		}
	case "bn2":
		changed := strings.Replace(files["checkout.py"], "from pricing", "from discounts", 1)
		prompt := "Use LSP goToDefinition on checkout.py at line 5, character 12. Use Write to change its import from pricing to discounts, preserving the invoice_total alias. Repeat definition lookup at the same position and report the new target."
		return []Case{
			{Name: "successive-document-versions", Prompt: prompt + " Restore the pricing import using Write and repeat the lookup.", Queries: []Query{definition, definition, definition}, FinalFiles: map[string]string{"checkout.py": files["checkout.py"]}, WantDefinitions: []string{"pricing.py", "discounts.py", "pricing.py"}},
			{Name: "write-before-open", Prompt: "Use Write to change checkout.py's import from pricing to discounts, then look up invoice_total at line 5, character 12.", Queries: []Query{definition}, FinalFiles: map[string]string{"checkout.py": changed}, WantDefinitions: []string{"discounts.py"}},
		}
	case "ql9":
		files["test_invoice.py"] = InvoiceTests
		references.AtDefinition = true
		updatedDefinition := definition
		updatedDefinition.AfterEdits = true
		updatedDefinition.AtCall = true
		updatedReferences := references
		updatedReferences.AfterEdits = true
		prompt := `Change pricing.total to return a Price TypedDict with an integer cents field. Update checkout.py and reports.py so their results stay unchanged. Before any Write edits, use LSP goToDefinition on checkout.py at line 5, character 12, and LSP findReferences on the returned pricing.total definition. After all Write edits, repeat LSP goToDefinition on the invoice_total call in checkout.py at its current line and character, and LSP findReferences at the updated pricing.total definition. Use the returned definition position for findReferences, converting its zero-based range to one-based tool inputs. Both post-edit LSP queries are required even if diagnostics are clean and the project tests pass. Use diagnostic feedback from Write to fix type errors. Report any diagnostic timeout. Leave tests and unrelated files unchanged. Run python3 -m unittest -v test_invoice.py and report the result.`
		queries := []Query{definition, references, updatedDefinition, updatedReferences}
		shifted := Project(false)
		shifted["test_invoice.py"] = InvoiceTests
		shifted["pricing.py"] = "# Pricing API\n" + shifted["pricing.py"]
		return []Case{
			{Name: "api-migration-across-files", Prompt: prompt, Queries: queries, Files: files},
			{Name: "stale-and-unrelated-diagnostics", Prompt: prompt, Queries: queries, Files: files, Config: Config{StaleDiagnostics: true}},
			{Name: "unversioned-diagnostics", Prompt: prompt, Queries: queries, Files: files, Config: Config{UnversionedDiagnostics: true}},
			{Name: "migration-without-diagnostic-publications", Prompt: prompt, Queries: queries, Files: files, Config: Config{MissingDiagnostics: true}},
			{Name: "migration-with-shifted-definition", Prompt: prompt, Queries: queries, Files: shifted},
		}
	}
	return nil
}
