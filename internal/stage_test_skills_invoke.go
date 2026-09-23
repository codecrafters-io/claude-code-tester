package internal

import (
	"fmt"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testSkillsInvoke(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	names := skills_manager.RandomNames(2)
	tokens := skills_manager.RandomTokens(2)
	topics := skills_manager.RandomDescriptionTopics(2)

	targetSkill := skills_manager.Skill{
		Name:        names[0],
		Description: topics[0].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[0]),
	}

	// The second skill exists only so that loading every body fails this stage:
	// the model would see two conflicting "respond with exactly" instructions.
	decoySkill := skills_manager.Skill{
		Name:        names[1],
		Description: topics[1].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[1]),
	}

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{targetSkill, decoySkill}, stageLogger)

	stageLogger.Infof("Invoking /%s, expecting only its body to be loaded", targetSkill.Name)

	invokeTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      fmt.Sprintf("/%s", targetSkill.Name),
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: tokens[0],
		},
	}

	return invokeTestCase.Run(stageHarness)
}
