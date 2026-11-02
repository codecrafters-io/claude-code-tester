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
		{"setup-without-configuration-request", "cx7", `self.languages = self.transport.call("configuration", {})["extensionToLanguage"]`, `self.languages = {".py": "python"}`, "call configuration with {}"},
		{"setup-without-lsp-tool", "cx7", `"tools": TOOLS`, `"tools": [tool for tool in TOOLS if tool["function"]["name"] != "LSP"]`, "advertise the supplied LSP tool"},
		{"setup-incomplete-tool-schema", "cx7", `"required": ["operation", "filePath", "line", "character"]`, `"required": ["operation", "filePath", "line"]`, "required character of type integer"},
		{"tool-coordinates-not-converted", "gz6", `"line": arguments["line"] - 1`, `"line": arguments["line"]`, "expected goToDefinition"},
		{"discarded-definition", "gz6", "        return result\n\n", "        return None\n\n", "server result did not reach"},
		{"references-declaration-omitted", "rb6", `{"includeDeclaration": True}`, `{"includeDeclaration": False}`, "includeDeclaration: true"},
		{"unsynchronized-write", "bn2", `self.send("textDocument/didChange", {`, `self.send("textDocument/didNotChange", {`, "stale text"},
		{"unversioned-diagnostics-discarded", "ql9", `version = diagnostic.get("version")`, `version = diagnostic.get("version")
            if version is None:
                continue`, "Return an error diagnostic from the preceding Write"},
		{"cached-unversioned-diagnostics", "ql9", `before = self.diagnostic_ticks.get(uri, 0)`, `before = -1`, "Return an error diagnostic from the preceding Write"},
		{"timeout-as-empty-diagnostics", "ql9", `raise TimeoutError("Diagnostic feedback error: no fresh publication")`, `return {"uri":uri,"diagnostics":[]}`, "missing diagnostics must be an error"},
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

// Setup passes before the agent implements any navigation operations.
func TestLSPSetupWithoutNavigation(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	source := strings.Replace(string(reference), "    def tool(self, arguments):", "    def tool(self, arguments):\n        raise AssertionError(\"Navigation is not implemented\")", 1)
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "cx7")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "Connected to the helper and advertised LSP; navigation is next")
}
