package internal

import (
	"fmt"
	"path/filepath"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/filesystem_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testBashTool(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	workspaceDirPath := stageHarness.Executable.WorkingDir

	appDirPath := filepath.Join(workspaceDirPath, "app")
	utils.MustCreateDirWithLogging(appDirPath, stageHarness.Logger)

	readmePath := filepath.Join(workspaceDirPath, "README.md")
	readmeContent := `# My Project
Uses async js to demonstrate web fetch.
Entry point: app/`
	utils.MustCreateFileWithContentsWithLogger(readmePath, readmeContent, stageHarness.Logger)

	readmeOldPath := filepath.Join(workspaceDirPath, "README_old.md")
	readmeOldContent := `# My project
Uses javascript promise api to demonstrate web fetch.
Entry point: app/`
	utils.MustCreateFileWithContentsWithLogger(readmeOldPath, readmeOldContent, stageHarness.Logger)

	mainFilePath := filepath.Join(appDirPath, "main.js")
	mainContent := `async function main() {
  const response = await fetch('https://jsonplaceholder.typicode.com/posts/1');
  const data = await response.json();
  console.log(data);
}

main();`
	utils.MustCreateFileWithContentsWithLogger(mainFilePath, mainContent, stageHarness.Logger)

	// Prompt to delete old readme
	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"Delete the old readme file.",
				"Remove the old readme from the project.",
			},
			"",
		),
		ExpectedExitCode: 0,
	}

	// Run the test case to execute the prompt
	if err := promptTestCase.Run(stageHarness); err != nil {
		return err
	}

	oldReadmeAssertion := filesystem_assertion.FileDoesNotExistAssertion{}
	if err := oldReadmeAssertion.Run(readmeOldPath, stageHarness.Logger); err != nil {
		return fmt.Errorf("README_old.md deletion assertion failed: %w", err)
	}

	newReadmeAssertion := filesystem_assertion.FileContentsAssertion{
		ExpectedContents: readmeContent,
	}
	if err := newReadmeAssertion.Run(readmePath, stageHarness.Logger); err != nil {
		return fmt.Errorf("README.md assertion failed: %w", err)
	}

	mainJsAssertion := filesystem_assertion.FileContentsAssertion{
		ExpectedContents: mainContent,
	}
	if err := mainJsAssertion.Run(mainFilePath, stageHarness.Logger); err != nil {
		return fmt.Errorf("app/main.js assertion failed: %w", err)
	}

	return nil
}
