package internal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/codecrafters-io/claude-code-tester/internal/lsp_manager"
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
		token := make([]byte, 12)
		if _, err := rand.Read(token); err != nil {
			return err
		}
		config.Token = hex.EncodeToString(token)
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
		result, runError := stageHarness.Executable.Run("-p", testCase.Prompt)
		closeModel()
		if runError != nil {
			return fmt.Errorf("%s: %w", testCase.Name, runError)
		}
		if testCase.Failure {
			if result.ExitCode == 0 {
				return fmt.Errorf("%s: initialization failure must exit nonzero", testCase.Name)
			}
			if len(strings.TrimSpace(string(result.Stderr))) == 0 && len(strings.TrimSpace(string(result.Stdout))) == 0 {
				return fmt.Errorf("%s: program exited %d without explaining the initialization error", testCase.Name, result.ExitCode)
			}
		} else if result.ExitCode != 0 {
			return fmt.Errorf("%s: program exited %d; stdout %q; stderr %q", testCase.Name, result.ExitCode, result.Stdout, result.Stderr)
		}
		if err := lsp_manager.Check(testCase, slug, config, recorder.RequestBodies()); err != nil {
			return fmt.Errorf("%s: %w", testCase.Name, err)
		}
		if len(testCase.WantDiagnosticVersions) > 0 {
			stageHarness.Logger.Successf("Edit feedback for checkout.py: %s", lsp_manager.ArgumentTypeDiagnostic)
			stageHarness.Logger.Successf("Repair feedback for checkout.py: diagnostics []")
		}
		stageHarness.Logger.Successf("Case %s passed protocol, tool-result and file checks", testCase.Name)
	}
	return nil
}
