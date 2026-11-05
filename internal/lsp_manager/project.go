package lsp_manager

import (
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The fixture analyzes only this documented project shape. It is not a Python server.
func Project(shortCheckout bool) map[string]string {
	checkout := "from pricing import total as invoice_total\n\n\ndef checkout_total() -> int:\n    return invoice_total(250, 2)\n"
	if shortCheckout {
		checkout = "from pricing import total as invoice_total\n\namount = invoice_total(250, 2)\n"
	}
	reports := "from pricing import total\n\n\ndef report_total() -> int:\n    return total(250, 2) + total(100, 1)\n"
	return map[string]string{
		"discounts.py": "def total(price: int, quantity: int) -> int:\n    return price * quantity - 50\n",
		"pricing.py":   "def total(price: int, quantity: int) -> int:\n    return price * quantity\n",
		"checkout.py":  checkout,
		"reports.py":   reports,
		"preview.py":   "# The word total in this comment should stay as it is.\ndef total(label: str) -> str:\n    return \"Preview total: \" + label\n",
	}
}

func URI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func Path(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" || parsed.Host != "" {
		return "", fmt.Errorf("expected a local file URI, got %q", uri)
	}
	return filepath.FromSlash(parsed.Path), nil
}

var signaturePattern = regexp.MustCompile(`(?m)^def ([a-zA-Z_][a-zA-Z_0-9]*)\(price: (int|str), quantity: int\) -> (int|str|Price):`)
var wordPattern = regexp.MustCompile(`[a-zA-Z_][a-zA-Z_0-9]*`)

type Document struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type projectState struct {
	Root      string
	Documents map[string]Document
	Indexed   map[string]string
}

func (p *projectState) text(name string) string {
	uri := URI(filepath.Join(p.Root, name))
	if document, ok := p.Documents[uri]; ok {
		return document.Text
	}
	// Simulate a server's indexed closed-file text until the client supplies an update.
	if p.Indexed == nil {
		p.Indexed = map[string]string{}
	}
	if text, ok := p.Indexed[uri]; ok {
		return text
	}
	contents, _ := os.ReadFile(filepath.Join(p.Root, name))
	p.Indexed[uri] = string(contents)
	return string(contents)
}

func (p *projectState) signature() []string {
	return signaturePattern.FindStringSubmatch(p.text("pricing.py"))
}

func (p *projectState) references(includeDeclaration bool) []Location {
	signature := p.signature()
	result := []Location{}
	if len(signature) == 0 {
		return result
	}
	name := signature[1]
	for _, file := range []string{"pricing.py", "checkout.py", "reports.py"} {
		text := p.text(file)
		for _, match := range wordPattern.FindAllStringIndex(text, -1) {
			word := text[match[0]:match[1]]
			if file == "pricing.py" && (!includeDeclaration || word != name) {
				continue
			}
			if file != "pricing.py" && word != name && !(file == "checkout.py" && word == "invoice_total") {
				continue
			}
			lineStart := strings.LastIndex(text[:match[0]], "\n") + 1
			line := text[lineStart:match[0]]
			if strings.Contains(line, "#") {
				continue
			}
			// The local alias declaration is not another reference.
			if word == "invoice_total" && strings.HasPrefix(line, "from ") {
				continue
			}
			result = append(result, Location{URI: URI(filepath.Join(p.Root, file)), Range: textRange(text, match[0], match[1])})
		}
	}
	return result
}

// Only resolve positions on this fixture's pricing symbol and its references.
func (p *projectState) hasSymbolAt(uri string, position Position) bool {
	for _, location := range p.references(true) {
		if location.URI == uri && position.Line == location.Range.Start.Line && position.Character >= location.Range.Start.Character && position.Character < location.Range.End.Character {
			return true
		}
	}
	return false
}

func (p *projectState) definition() any {
	target := "pricing.py"
	if strings.Contains(p.text("checkout.py"), "from discounts import") {
		target = "discounts.py"
	}
	text := p.text(target)
	match := signaturePattern.FindStringSubmatchIndex(text)
	if len(match) == 0 {
		return nil
	}
	return Location{URI: URI(filepath.Join(p.Root, target)), Range: textRange(text, match[2], match[3])}
}

const ArgumentTypeDiagnostic = "Type Price is not assignable to return type int"

const OperatorDiagnostic = "Operator + not supported for types Price and Price"

const PriceAPI = "from typing import TypedDict\n\n\nclass Price(TypedDict):\n    cents: int\n\n\ndef total(price: int, quantity: int) -> Price:\n    return {\"cents\": price * quantity}\n"

// diagnostics reports what Pyright reports once pricing.total returns a Price: a caller
// that still treats the result as an int, whether or not that caller was the file written.
func (p *projectState) diagnostics(document Document) []map[string]any {
	result := []map[string]any{}
	if !strings.Contains(p.text("pricing.py"), "-> Price:") {
		return result
	}
	// report flags expression where it follows prefix, so only the unconverted use matches.
	report := func(prefix, expression, message string) {
		if at := strings.Index(document.Text, prefix+expression+"\n"); at >= 0 {
			start := at + len(prefix)
			result = append(result, map[string]any{"range": textRange(document.Text, start, start+len(expression)), "severity": 1, "message": message})
		}
	}
	switch filepath.Base(mustPath(document.URI)) {
	case "checkout.py":
		report("    return ", "amount", ArgumentTypeDiagnostic)
		report("    return ", "invoice_total(250, 2)", ArgumentTypeDiagnostic)
	case "reports.py":
		report("    return ", "total(250, 2) + total(100, 1)", OperatorDiagnostic)
	}
	return result
}

func mustPath(uri string) string { path, _ := Path(uri); return path }

// These behavior tests are supplied with the invoice project for the Bash check.
//
//go:embed testdata/test_invoice.py
var InvoiceTests string
