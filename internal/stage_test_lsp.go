package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"

	"al.essio.dev/pkg/shellescape"
	"github.com/codecrafters-io/claude-code-tester/internal/lsp_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/lsp_transport"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testLSP(stageHarness *test_case_harness.TestCaseHarness, slug string) error {
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	originalURL, hadURL := os.LookupEnv("OPENROUTER_BASE_URL")
	defer func() {
		if hadURL {
			_ = os.Setenv("OPENROUTER_BASE_URL", originalURL)
		} else {
			_ = os.Unsetenv("OPENROUTER_BASE_URL")
		}
	}()
	for _, testCase := range lsp_manager.Cases(slug) {
		stageHarness.Logger.Infof("Case: %s", testCase.Name)
		workspace := workspace_manager.NewWorkspaceManager()
		workspace.BootstrapExecutableWorkspace(stageHarness)
		// A path containing spaces exercises URI conversion on every stage.
		project := filepath.Join(workspace.GetWorkspaceDirPath(), "invoice project")
		if err := os.Mkdir(project, 0755); err != nil {
			return err
		}
		project, err := filepath.EvalSymlinks(project)
		if err != nil {
			return err
		}
		stageHarness.Executable.WorkingDir = project
		files := testCase.Files
		if files == nil {
			files = lsp_manager.Project(false)
		}
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(project, name), []byte(files[name]), 0644); err != nil {
				return err
			}
			stageHarness.Logger.Infof("Wrote %s: %q", name, files[name])
		}
		controlDir, err := os.MkdirTemp("", "codecrafters-lsp-control-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(controlDir)
		config := testCase.Config
		config.Stage = slug
		config.Root = project
		config.Log = filepath.Join(controlDir, "events.jsonl")
		configPath := filepath.Join(controlDir, "server.json")
		encoded, _ := json.Marshal(config)
		if err := os.WriteFile(configPath, encoded, 0600); err != nil {
			return err
		}
		if !testCase.NoConfig {
			binary, err := os.Executable()
			if err != nil {
				return err
			}
			// Test binaries do not dispatch the fixture CLI. The Go tests set a
			// built tester binary explicitly rather than recursively running tests.
			if fixture := os.Getenv("CODECRAFTERS_LSP_FIXTURE_BINARY"); fixture != "" {
				binary = fixture
			}
			configuration, _ := json.Marshal(map[string]any{"python": map[string]any{"command": binary, "args": []string{"--lsp-fixture", configPath}, "extensionToLanguage": map[string]string{".py": "python"}}})
			if err := os.WriteFile(filepath.Join(project, ".lsp.json"), configuration, 0644); err != nil {
				return err
			}
			stageHarness.Logger.Infof("Wrote .lsp.json: %s", configuration)
		}
		var recorder *proxy_server.RequestRecorder
		var closeModel func()
		var modelURL string
		if os.Getenv("CODECRAFTERS_LSP_OFFLINE") == "1" {
			recorder, modelURL, closeModel, err = lsp_manager.StartOfflineModel(testCase.Actions)
			if err != nil {
				return err
			}
		} else {
			recorder, modelURL, closeModel, err = proxy_server.StartIsolatedProxyServer(stageHarness)
			if err != nil {
				return err
			}
		}
		stageHarness.RegisterTeardownFunc(closeModel)
		if err := os.Setenv("OPENROUTER_BASE_URL", modelURL); err != nil {
			return err
		}
		stageHarness.Logger.Infof("$ ./your_program.sh -p %s", shellescape.Quote(testCase.Prompt))
		// The course owns startup and framing. The submitted agent owns document messages.
		oldTransport, hadTransport := os.LookupEnv("CODECRAFTERS_LSP_URL")
		var transport *lsp_transport.Transport
		var setupProxy *httptest.Server
		var configurationRequests atomic.Int64
		if !testCase.NoConfig {
			var transportErr error
			transport, transportErr = lsp_transport.Start(filepath.Join(project, ".lsp.json"))
			if transportErr != nil {
				return transportErr
			}
			connectionURL := transport.URL
			if slug == "lc1" {
				// Observe setup's configuration call without changing the supplied helper.
				target, err := url.Parse(transport.URL)
				if err != nil {
					transport.Close()
					return err
				}
				proxy := httputil.NewSingleHostReverseProxy(target)
				setupProxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost && r.URL.Path == "/configuration" {
						configurationRequests.Add(1)
					}
					proxy.ServeHTTP(w, r)
				}))
				connectionURL = setupProxy.URL
			}
			os.Setenv("CODECRAFTERS_LSP_URL", connectionURL)
		} else {
			os.Unsetenv("CODECRAFTERS_LSP_URL")
		}

		result, runError := stageHarness.Executable.Run("-p", testCase.Prompt)
		if setupProxy != nil {
			setupProxy.Close()
		}
		if transport != nil {
			transport.Close()
		}
		if hadTransport {
			os.Setenv("CODECRAFTERS_LSP_URL", oldTransport)
		} else {
			os.Unsetenv("CODECRAFTERS_LSP_URL")
		}
		closeModel()
		if runError != nil {
			return fmt.Errorf("%s: %w", testCase.Name, runError)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("%s: program exited %d; stdout %q; stderr %q", testCase.Name, result.ExitCode, result.Stdout, result.Stderr)
		}
		if slug == "lc1" && !testCase.NoConfig && configurationRequests.Load() == 0 {
			return fmt.Errorf("%s: connect with the supplied client and call configuration with {}", testCase.Name)
		}
		if err := lsp_manager.Check(testCase, slug, config, recorder.RequestBodies()); err != nil {
			return fmt.Errorf("%s: %w", testCase.Name, err)
		}
		if len(testCase.WantDiagnosticVersions) > 0 {
			stageHarness.Logger.Successf("Partial API migration: Write saved checkout.py, but the caller still returns Price as int")
			stageHarness.Logger.Successf("Edit feedback for checkout.py: %s", lsp_manager.ArgumentTypeDiagnostic)
			stageHarness.Logger.Successf("Repair feedback for checkout.py: diagnostics []")
		}
		if slug == "lc1" && !testCase.NoConfig {
			stageHarness.Logger.Successf("Connected to the helper and advertised LSP; navigation is next")
		}
		if slug == "df3" && testCase.Config.Definition == "" && !testCase.NoConfig {
			stageHarness.Logger.Successf("Resolved invoice_total to pricing.total; preview.total is unrelated")
		}
		if slug == "rf4" && !testCase.Config.EmptyReferences {
			stageHarness.Logger.Successf("Returned the affected pricing callers; excluded unrelated preview.total")
		}
		if slug == "sy5" && len(testCase.WantDefinitions) > 1 {
			stageHarness.Logger.Successf("Definition lookup followed the updated import without restarting the server")
		}
		stageHarness.Logger.Successf("Case %s passed protocol, tool-result and file checks", testCase.Name)
	}
	return nil
}
