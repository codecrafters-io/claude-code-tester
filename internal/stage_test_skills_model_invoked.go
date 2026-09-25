package internal

import (
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testSkillsModelInvoked(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	names := skills_manager.RandomNames(2)
	tokens := skills_manager.RandomTokens(2)
	topics := skills_manager.RandomInvocationTopics(2)

	targetSkill := skills_manager.Skill{
		Name:        names[0],
		Description: topics[0].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[0]),
	}

	// Without a decoy, this stage would pass for a program that loads whatever
	// single skill it finds. The decoy is what makes matching observable.
	decoySkill := skills_manager.Skill{
		Name:        names[1],
		Description: topics[1].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[1]),
	}

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{targetSkill, decoySkill}, stageLogger)

	stageLogger.Infof("Sending a request that matches %q without naming it", targetSkill.Name)
	stageLogger.Debugf("Decoy skill %q must not be loaded", decoySkill.Name)

	// The question reads as a question, so the model often answers it in a
	// sentence rather than emitting the token alone. Which skill was loaded is
	// what this stage is about, and that shows in which token appears.
	modelInvokedTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      topics[0].Question,
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.AllOfAssertion{
			Assertions: []string_assertion.StringAssertion{
				string_assertion.ContainsAllAssertion{ExpectedValues: []string{tokens[0]}},
				string_assertion.DoesNotContainAssertion{UnexpectedValue: tokens[1]},
			},
		},
	}

	return modelInvokedTestCase.Run(stageHarness)
}
