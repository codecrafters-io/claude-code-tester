package internal

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func lspScenario(t *testing.T, source string) string {
	t.Helper()
	directory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(directory, "app"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "app/main.py"), []byte(source), 0644))
	helper, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/lsp_client.py")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "app/lsp_client.py"), helper, 0644))
	launcher, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/your_program.sh")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "your_program.sh"), launcher, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "codecrafters.yml"), []byte("debug: false\n"), 0644))
	return directory
}

// Reference-agent document opening uses the helper's configured language mapping.
func TestLSPConfiguredDocumentLanguage(t *testing.T) {
	// Arrange
	command := exec.Command("python3", "-m", "unittest", "discover", "-s", "test_helpers/scenarios/lsp_stages/users_code_pass_all/app", "-p", "test_language.py")
	// Act
	output, err := command.CombinedOutput()
	// Assert
	require.NoError(t, err, string(output))
}

func runLSPScenario(t *testing.T, binary, directory, slug string) (string, error) {
	t.Helper()
	stages, _ := json.Marshal([]map[string]string{{"slug": slug, "tester_log_prefix": "lsp-test", "title": "LSP regression"}})
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "CODECRAFTERS_LSP_OFFLINE=1", "CODECRAFTERS_REPOSITORY_DIR="+directory, "CODECRAFTERS_TEST_CASES_JSON="+string(stages), "CODECRAFTERS_LSP_FIXTURE_BINARY="+binary)
	output, err := command.CombinedOutput()
	return string(output), err
}

// Broken clients fail the stage whose protocol or state behavior they violate.
func TestLSPRejectsBrokenClients(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	cases := []struct{ name, slug, from, to, want string }{
		{"connection-without-configuration-request", "dz5", `self.languages = self.transport.call("configuration", {})["extensionToLanguage"]`, `self.languages = {".py": "python"}`, "call configuration with {}"},
		{"advertisement-without-configuration-request", "iy3", `self.languages = self.transport.call("configuration", {})["extensionToLanguage"]`, `self.languages = {".py": "python"}`, "call configuration with {}"},
		{"advertisement-without-lsp-tool", "iy3", `"tools": TOOLS`, `"tools": [tool for tool in TOOLS if tool["function"]["name"] != "LSP"]`, "advertise the supplied LSP tool"},
		{"advertisement-incomplete-tool-schema", "iy3", `"required": ["operation", "filePath", "line", "character"]`, `"required": ["operation", "filePath", "line"]`, "required character of type integer"},
		{"advertisement-missing-from-later-request", "iy3", `"tools": TOOLS`, `"tools": TOOLS if len(messages) == 2 else [tool for tool in TOOLS if tool["function"]["name"] != "LSP"]`, "advertise the supplied LSP tool"},
		{"tool-coordinates-not-converted", "gz6", `"line": arguments["line"] - 1`, `"line": arguments["line"]`, "expected goToDefinition"},
		{"definition-read-not-returned", "gz6", `result = Path(arguments["file_path"]).read_text(encoding="utf-8")`, `result = "File access disabled"`, "read pricing.py and return its complete contents"},
		{"discarded-definition", "gz6", "        return result\n\n", "        return None\n\n", "server result did not reach"},
		{"references-declaration-omitted", "rb6", `{"includeDeclaration": True}`, `{"includeDeclaration": False}`, "includeDeclaration: true"},
		{"unsynchronized-write", "bn2", `self.send("textDocument/didChange", {`, `self.send("textDocument/didNotChange", {`, "stale text"},
		{"capstone-unsynchronized-write", "ql9", `self.send("textDocument/didChange", {`, `self.send("textDocument/didNotChange", {`, "stale text"},
		{"capstone-closed-file-not-synchronized", "ql9", `elif path.suffix in self.languages:
            self.open(path)`, `elif path.suffix in self.languages:
            pass`, "follow-up references contain stale locations"},
		{"capstone-missed-reporting-caller", "ql9", `path.write_text(content, encoding="utf-8")`, `if path.name != "reports.py":
            path.write_text(content, encoding="utf-8")`, "return the passing test output"},
		{"capstone-unrelated-preview-edited", "ql9", `path.write_text(content, encoding="utf-8")`, `path.write_text(content, encoding="utf-8")
        Path("preview.py").write_text("# unrelated edit\n")`, "unrelated file preview.py must remain unchanged"},
		{"capstone-lost-test-output", "ql9", `result = {"stdout": output.stdout, "stderr": output.stderr, "exit_code": output.returncode}`, `result = {"exit_code": output.returncode}`, "return the passing test output"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			require.Contains(t, string(reference), testCase.from)
			source := strings.Replace(string(reference), testCase.from, testCase.to, 1)
			directory := lspScenario(t, source)
			// Act
			output, err := runLSPScenario(t, binary, directory, testCase.slug)
			// Assert
			require.Error(t, err, output)
			require.Contains(t, output, testCase.want)
		})
	}
}

// Definition lookup passes with an agent that handles only successful locations.
func TestLSPDefinitionNeedsOnlySuccessfulLocation(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	request := `result = self.request("textDocument/" + methods[operation], params)`
	require.Contains(t, string(reference), request)
	source := strings.Replace(string(reference), request, request+`
        assert isinstance(result, dict) and "uri" in result and "range" in result`, 1)
	errorHandler := `            except Exception as error:
                result = {"error": str(error)}`
	require.Contains(t, source, errorHandler)
	source = strings.Replace(source, errorHandler, `            except Exception as error:
                raise AssertionError("This agent only supports successful tool calls") from error`, 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "gz6")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "Resolved invoice_total to pricing.total")
}

// Synchronization passes without converting failed tool calls into error results.
func TestLSPSynchronizationNeedsOnlySuccessfulWrites(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	errorHandler := `            except Exception as error:
                result = {"error": str(error)}`
	require.Contains(t, string(reference), errorHandler)
	source := strings.Replace(string(reference), errorHandler, `            except Exception as error:
                raise AssertionError("This agent only supports successful tool calls") from error`, 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "bn2")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "Case successive-document-versions passed")
	require.Contains(t, output, "Case write-before-open passed")
}

// A legacy model server on port 10000 cannot contaminate LSP stage recordings.
func TestLSPIsolatesLegacyModelTraffic(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	var legacyRequests atomic.Int64
	legacy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		legacyRequests.Add(1)
		http.Error(w, "unrelated model endpoint", http.StatusUnauthorized)
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:10000")
	require.NoError(t, err)
	legacy.Listener = listener
	legacy.Start()
	defer legacy.Close()
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	directory := lspScenario(t, string(reference))
	// Act
	output, err := runLSPScenario(t, binary, directory, "gz6")
	// Assert
	require.NoError(t, err, output)
	require.Zero(t, legacyRequests.Load())
	require.Contains(t, output, "Test passed")
}

// Tool advertisement passes before the agent implements navigation.
func TestLSPAdvertisementWithoutNavigation(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	source := strings.Replace(string(reference), "    def tool(self, arguments):", "    def tool(self, arguments):\n        raise AssertionError(\"Navigation is not implemented\")", 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "iy3")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "Advertised LSP; definition lookup is next")
}

// Connecting the client passes before the agent advertises an LSP tool.
func TestLSPConnectionWithoutToolAdvertisement(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	source := strings.Replace(string(reference), `"tools": TOOLS`, `"tools": [tool for tool in TOOLS if tool["function"]["name"] != "LSP"]`, 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "dz5")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "Connected the supplied LSP client and read configuration; tool advertisement is next")
}

// A configuration call after the first model response cannot satisfy startup.
func TestLSPConfigurationBeforeModelRequest(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	source := strings.Replace(string(reference), `self.languages = self.transport.call("configuration", {})["extensionToLanguage"]`, `self.languages = {".py": "python"}`, 1)
	source = strings.Replace(source, "        messages.append(message)", `        server.transport.call("configuration", {})
        messages.append(message)`, 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "dz5")
	// Assert
	require.Error(t, err, output)
	require.Contains(t, output, "before the first model request")
}

// The capstone passes without implementing diagnostic collection in the agent.
func TestLSPMigrationWithoutDiagnostics(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	require.NotContains(t, string(reference), "publishDiagnostics")
	directory := lspScenario(t, string(reference))
	// Act
	output, err := runLSPScenario(t, binary, directory, "ql9")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "migration-without-diagnostic-publications")
	require.Contains(t, output, "Project tests passed through Bash")
}

// Equivalent source code passes the capstone's behavior checks.
func TestLSPMigrationAcceptsEquivalentImplementation(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	source := strings.Replace(string(reference), `path.write_text(content, encoding="utf-8")`, `content = content.replace('return {"cents": price * quantity}', 'return dict(cents=price * quantity)')
        path.write_text(content, encoding="utf-8")`, 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "ql9")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "Project tests passed through Bash")
}

// Fabricated passing Bash output cannot hide a caller that was never migrated.
func TestLSPMigrationRejectsFabricatedTestOutput(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	source := strings.Replace(string(reference), `path.write_text(content, encoding="utf-8")`, `if path.name != "reports.py":
            path.write_text(content, encoding="utf-8")`, 1)
	source = strings.Replace(source, `result = {"stdout": output.stdout, "stderr": output.stderr, "exit_code": output.returncode}`, `result = {"stdout": "Ran 5 tests\nOK", "exit_code": 0}`, 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "ql9")
	// Assert
	require.Error(t, err, output)
	require.Contains(t, output, "migration behavior check failed")
}
