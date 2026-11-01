package lsp_manager

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The fixture analyzes only this documented project shape. It is not a Python server.
func Project(shortCheckout bool) map[string]string {
	checkout := "from pricing import total as invoice_total\n\n\ndef checkout_total() -> int:\n    return invoice_total(\"250\", 2)\n"
	if shortCheckout {
		checkout = "from pricing import total as invoice_total\n\namount = invoice_total(250, 2)\n"
	}
	reports := "from pricing import total\n\n\ndef report_total() -> int:\n    return total(250, 2) + total(100, 1)\n"
	return map[string]string{
		"pricing.py":  "def total(price: int, quantity: int) -> int:\n    return price * quantity\n",
		"checkout.py": checkout,
		"reports.py":  reports,
		"preview.py":  "# The word total in this comment should stay as it is.\ndef total(label: str) -> str:\n    return \"Preview total: \" + label\n",
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

var signaturePattern = regexp.MustCompile(`(?m)^def ([a-zA-Z_][a-zA-Z_0-9]*)\(price: (int|str), quantity: int\) -> (int|str):`)
var wordPattern = regexp.MustCompile(`[a-zA-Z_][a-zA-Z_0-9]*`)
var argumentPattern = regexp.MustCompile(`invoice_total\(("[^"]*"|[0-9]+),`)

type Document struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type projectState struct {
	Root      string
	Documents map[string]Document
}

func (p *projectState) text(name string) string {
	uri := URI(filepath.Join(p.Root, name))
	if document, ok := p.Documents[uri]; ok {
		return document.Text
	}
	contents, _ := os.ReadFile(filepath.Join(p.Root, name))
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

func (p *projectState) definition() any {
	text := p.text("pricing.py")
	match := signaturePattern.FindStringSubmatchIndex(text)
	if len(match) == 0 {
		return nil
	}
	return Location{URI: URI(filepath.Join(p.Root, "pricing.py")), Range: textRange(text, match[2], match[3])}
}

const ArgumentTypeDiagnostic = "Argument of type str cannot be assigned to parameter price of type int"

func (p *projectState) diagnostics(document Document) []map[string]any {
	result := []map[string]any{}
	if filepath.Base(mustPath(document.URI)) != "checkout.py" {
		return result
	}
	signature := p.signature()
	match := argumentPattern.FindStringSubmatchIndex(document.Text)
	if len(signature) > 0 && signature[2] == "int" && len(match) > 0 && document.Text[match[2]] == '"' {
		result = append(result, map[string]any{"range": textRange(document.Text, match[2], match[3]), "severity": 1, "message": ArgumentTypeDiagnostic})
	}
	return result
}

func mustPath(uri string) string { path, _ := Path(uri); return path }
