package internal

import (
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/filesystem_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
	"github.com/codecrafters-io/tester-utils/testing"
)

func testBashTool(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	// Store file contents in variables
	mainJsContent := `async function main() {
  const response = await fetch('https://jsonplaceholder.typicode.com/posts/1');
  const data = await response.json();
  console.log(data);
}

main();`

	readmeContent := `# My Project
Uses async js to demonstrate web fetch.
Entry point: app/`

	readmeOldContent := `# My project
Uses javascript promise api to demonstrate web fetch.
Entry point: app/`

	// Create files using MustCreateFiles
	workspaceManager.MustCreateFilesWithLogger([]workspace_manager.WorkspaceFile{
		{
			RelativePath: "app/main.js",
			Content:      mainJsContent,
			FileMode:     0644,
		},
		{
			RelativePath: "README.md",
			Content:      readmeContent,
			FileMode:     0644,
		},
		{
			RelativePath: "README_old.md",
			Content:      readmeOldContent,
			FileMode:     0644,
		},
	}, stageLogger)

	guardRailPrompt := ""

	// Ensure fixtures are stable
	if testing.IsRecordingOrEvaluatingFixtures() {
		guardRailPrompt = "Always respond with `Deleted old readme file.`"
	}

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"List files using ls and delete the old readme file.",
				"List project files using ls and delete the old readme file.",
			},
			guardRailPrompt,
		),
		ExpectedExitCode: 0,
	}

	if err := promptTestCase.Run(stageHarness); err != nil {
		return err
	}

	stageLogger.Infof("Checking workspace contents")

	// Assert that main file is intact
	mainFileAbsPath := workspaceManager.ConvertToAbsPath("app/main.js")
	mainJsAssertion := filesystem_assertion.FileContentsAssertion{ExpectedContents: mainJsContent}

	if err := mainJsAssertion.Run(mainFileAbsPath, stageLogger, workspaceManager.GetRelPathConverter()); err != nil {
		return err
	}

	// Assert that readme is intact
	readmePath := workspaceManager.ConvertToAbsPath("README.md")
	readmeAssertion := filesystem_assertion.FileContentsAssertion{ExpectedContents: readmeContent}

	if err := readmeAssertion.Run(readmePath, stageLogger, workspaceManager.GetRelPathConverter()); err != nil {
		return err
	}

	// Assert that old readme is deleted
	oldReadmeAbsPath := workspaceManager.ConvertToAbsPath("README_old.md")
	oldReadmeAssertion := filesystem_assertion.FileDoesNotExistAssertion{}

	return oldReadmeAssertion.Run(oldReadmeAbsPath, stageLogger, workspaceManager.GetRelPathConverter())
}
