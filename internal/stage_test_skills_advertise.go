package internal

import (
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testSkillsAdvertise(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	skillCount := random.RandomInt(2, 5) // 2, 3 or 4
	names := skills_manager.RandomNames(skillCount)
	topics := skills_manager.RandomDescriptionTopics(skillCount)

	skills := make([]skills_manager.Skill, skillCount)

	for i := range skills {
		skills[i] = skills_manager.Skill{
			Name:        names[i],
			Description: topics[i].Description,
			Body:        "Follow the project's conventions and report what you did.",
		}
	}

	skills_manager.Seed(workspaceManager, skills, stageLogger)

	// Assertion 1: the model can see how many skills exist.
	stageLogger.Infof("Checking that the model can count the advertised skills")

	countTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"How many skills are available to you?",
				"What is the count of skills available to you?",
				"Count the number of skills available to you.",
			},
			"Respond with only a number.",
		),
		ExpectedExitCode: 0,
		// A minimum, matching the tool-advertise stage: Claude Code ships its own
		// built-in skills, so the real product answers with these plus a dozen more.
		StdoutAssertion: string_assertion.MinimumValueAssertion{
			ExpectedMinimumValue: skillCount,
		},
	}

	if err := countTestCase.Run(stageHarness); err != nil {
		return err
	}

	// Assertion 2: the model can see the descriptions, not just the names.
	//
	// The target skill's description never mentions its own name, so answering
	// this correctly is only possible if the description reached the prompt.
	targetIndex := random.RandomInt(0, skillCount)
	targetSkill := skills[targetIndex]

	stageLogger.Infof("Checking that the model can match a task to %q using its description", targetSkill.Name)

	matchTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      topics[targetIndex].Question,
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: targetSkill.Name,
		},
	}

	return matchTestCase.Run(stageHarness)
}
