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
	launcher, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/your_program.sh")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "your_program.sh"), launcher, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "codecrafters.yml"), []byte("debug: false\n"), 0644))
	return directory
}

func runLSPScenario(t *testing.T, binary, directory, slug string) (string, error) {
	t.Helper()
	stages, _ := json.Marshal([]map[string]string{{"slug": slug, "tester_log_prefix": "lsp-test", "title": "LSP regression"}})
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "CODECRAFTERS_LSP_OFFLINE=1", "CODECRAFTERS_REPOSITORY_DIR="+directory, "CODECRAFTERS_TEST_CASES_JSON="+string(stages), "CODECRAFTERS_LSP_FIXTURE_BINARY="+binary)
	output, err := command.CombinedOutput()
	return string(output), err
}

// Stage one accepts a sending-only client that never reads the initialization reply.
func TestLSPSendingOnlyClient(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	source := `import argparse,json,os,subprocess,urllib.request
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('-p');parser.parse_args()
server=None
if Path('.lsp.json').exists():
    config=json.loads(Path('.lsp.json').read_text())
    server=subprocess.Popen([config['command'],*config['args']],stdin=subprocess.PIPE,stdout=subprocess.PIPE)
    body=json.dumps({'jsonrpc':'2.0','id':1,'method':'initialize','params':{'processId':os.getpid(),'rootUri':Path.cwd().as_uri(),'capabilities':{}}}).encode()
    server.stdin.write(f'Content-Length: {len(body)}\r\n\r\n'.encode()+body);server.stdin.flush()
try:
    request=urllib.request.Request(os.environ['OPENROUTER_BASE_URL']+'/chat/completions',data=json.dumps({'messages':[{'role':'user','content':'Reply with ready.'}]}).encode(),headers={'Content-Type':'application/json'})
    with urllib.request.urlopen(request) as response: response.read()
finally:
    if server: server.stdin.close();server.wait(timeout=3)
`
	directory := lspScenario(t, source)
	// Act
	output, err := runLSPScenario(t, binary, directory, "ls1")
	// Assert
	require.NoError(t, err, output)
	require.Contains(t, output, "without-configuration")
	require.Contains(t, output, "Test passed")
}

// Broken clients fail the stage whose protocol or state behavior they violate.
func TestLSPRejectsBrokenClients(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	reference, err := os.ReadFile("test_helpers/scenarios/lsp_stages/users_code_pass_all/app/main.py")
	require.NoError(t, err)
	cases := []struct{ name, slug, from, to, want string }{
		{"model-before-initialize", "ir2", "server.initialize()", "run_agent(arguments.p, None)\n            server.initialize()", "read initialization before calling the LLM"},
		{"discarded-hover", "hv2", "return result\n        prepared", "return None\n        prepared", "server result did not reach"},
		{"unsynchronized-write", "sy5", `self.send("textDocument/didChange", {`, `self.send("textDocument/didNotChange", {`, "stale text"},
		{"forward-rename-edits", "rn7", "sorted(replacements, reverse=True)", "sorted(replacements)", "rename"},
		{"codepoint-rename-columns", "rn7", `len(prefix.decode("utf-16-le"))`, `position["character"]`, "rename"},
		{"timeout-as-empty-diagnostics", "dg6", `            self.receive()
        return self.diagnostics[uri]`, `            try:
                self.receive()
            except TimeoutError:
                return {"uri": uri, "version": version, "diagnostics": []}
        return self.diagnostics[uri]`, "missing diagnostics must be an error"},
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
	output, err := runLSPScenario(t, binary, directory, "ir2")
	// Assert
	require.NoError(t, err, output)
	require.Zero(t, legacyRequests.Load())
	require.Contains(t, output, "Test passed")
}
