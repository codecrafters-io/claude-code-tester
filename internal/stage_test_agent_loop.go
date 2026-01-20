package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testAgentLoop(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	workspaceDirPath := stageHarness.Executable.WorkingDir

	appDirPath := filepath.Join(workspaceDirPath, "app")
	utils.MustCreateDirWithLogging(appDirPath, stageHarness.Logger)

	// Create supporting file
	extraFilePath := filepath.Join(appDirPath, random.RandomElementFromArray([]string{"chemical.py", "substance.py", "expiry.py", "duration.py"}))
	chemicalExpiryPeriod := random.RandomInt(6, 36) // Random value between 6 and 36 months
	extraFileContent := fmt.Sprintf("chemical_expiry_period = %d  # months", chemicalExpiryPeriod)
	utils.MustCreateFileWithContentsWithLogger(extraFilePath, extraFileContent, stageHarness.Logger)

	// Create main file
	mainFilePath := filepath.Join(
		appDirPath,
		random.RandomElementFromArray([]string{"main.py", "init.py", "start.py"}),
	)

	extraFileBaseNameWithoutExtension := strings.TrimSuffix(filepath.Base(extraFilePath), ".py")
	mainContent := fmt.Sprintf(`from %s import chemical_expiry_period

def main():
    print(f"Chemical expiry period: {chemical_expiry_period} months")

if __name__ == "__main__":
    main()
`, extraFileBaseNameWithoutExtension)

	utils.MustCreateFileWithContentsWithLogger(mainFilePath, mainContent, stageHarness.Logger)

	// Create README.md
	readmePath := filepath.Join(workspaceDirPath, "README.md")
	readmeContent := fmt.Sprintf(`This is a simple python project.
The starting point of this project is app/%s.`, filepath.Base(mainFilePath))
	utils.MustCreateFileWithContentsWithLogger(readmePath, readmeContent, stageHarness.Logger)

	expectedOutput := fmt.Sprintf("%d", chemicalExpiryPeriod)

	prompt := utils.GetPromptWithGuardRailPrompt(
		[]string{
			"Read the README.md, figure out the file that contains the chemical expiry period in months, and get me that value.",
			"Read README.md, determine which file contains the chemical expiry period in months, and get me that value.",
			"Read the README.md file, find the file that contains the chemical expiry period in months, and get me that value.",
			"Read README.md, figure out which file has the chemical expiry period in months, and get me that value.",
			"Read the README.md, identify the file containing the chemical expiry period in months, and get me that value.",
		},
		"Number only.",
	)

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      prompt,
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: expectedOutput,
		},
	}

	return promptTestCase.Run(stageHarness)
}
