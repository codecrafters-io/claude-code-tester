package internal

import (
	"fmt"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testSkillsScript(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	topic := skills_manager.RandomDescriptionTopics(1)[0]

	// The data file's contents are random, so the expected checksum can't be
	// derived by reading the script. Something has to actually run.
	dataFileContents := random.RandomString()
	expectedChecksum := skills_manager.ChecksumOf(dataFileContents)

	skill := skills_manager.Skill{
		Name:        skills_manager.RandomNames(1)[0],
		Description: topic.Description,
	}

	scriptPath := skill.ScriptPath(skills_manager.ChecksumScriptFileName)

	// The body names the script relative to the skill's own folder, so the
	// user's program has to tell the model where that folder is before the
	// model can turn the reference into a command that runs.
	skill.Body = fmt.Sprintf(
		"Run `%s` using the Bash tool.\n\nRespond with only the value it prints, and nothing else.",
		skills_manager.ScriptReference(skills_manager.ChecksumScriptFileName),
	)

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{skill}, stageLogger)

	workspaceManager.MustCreateFilesWithLogger([]workspace_manager.WorkspaceFile{
		{
			RelativePath: skills_manager.DataFileName,
			Content:      dataFileContents,
			FileMode:     0644,
		},
		{
			RelativePath: scriptPath,
			Content:      skills_manager.ChecksumScriptContents,
			FileMode:     0755,
		},
	}, stageLogger)

	stageLogger.Debugf("Expected checksum of %s is %s", skills_manager.DataFileName, expectedChecksum)
	stageLogger.Infof("Invoking /%s, expecting it to run its bundled script", skill.Name)

	scriptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      fmt.Sprintf("/%s", skill.Name),
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: expectedChecksum,
		},
	}

	return scriptTestCase.Run(stageHarness)
}
