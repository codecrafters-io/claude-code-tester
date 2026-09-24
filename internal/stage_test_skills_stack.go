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

func testSkillsStack(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	// Two skills are invoked together; the third is seeded but left alone, so a
	// program that loads every body instead of the invoked ones is caught.
	const seededSkillCount = 3
	const invokedSkillCount = 2

	names := skills_manager.RandomNames(seededSkillCount)
	tokens := skills_manager.RandomTokens(seededSkillCount)
	topics := skills_manager.RandomDescriptionTopics(seededSkillCount)

	skills := make([]skills_manager.Skill, seededSkillCount)

	for i := range skills {
		skills[i] = skills_manager.Skill{
			Name:        names[i],
			Description: topics[i].Description,
			Body:        skills_manager.StackedLineBody(tokens[i]),
		}
	}

	skills_manager.Seed(workspaceManager, skills, stageLogger)

	arguments := fmt.Sprintf("%d", random.RandomInt(1000, 10000))

	// Each expected line pairs a token with the argument, so one value proves
	// both that the body was loaded and that the argument reached it.
	expectedLines := make([]string, invokedSkillCount)

	for i := 0; i < invokedSkillCount; i++ {
		expectedLines[i] = skills_manager.StackedLine(tokens[i], arguments)
	}

	uninvokedToken := tokens[seededSkillCount-1]

	stageLogger.Debugf("Seeded %q alongside the invoked skills, expecting its body to stay unloaded", names[seededSkillCount-1])
	stageLogger.Infof("Invoking /%s and /%s in one prompt, with %s as the shared argument", names[0], names[1], arguments)

	stackTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      fmt.Sprintf("/%s /%s %s", names[0], names[1], arguments),
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.AllOfAssertion{
			Assertions: []string_assertion.StringAssertion{
				string_assertion.ContainsAllAssertion{ExpectedValues: expectedLines},
				string_assertion.DoesNotContainAssertion{UnexpectedValue: uninvokedToken},
			},
		},
	}

	return stackTestCase.Run(stageHarness)
}
