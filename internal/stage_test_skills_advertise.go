package internal

import (
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/request_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testSkillsAdvertise(stageHarness *test_case_harness.TestCaseHarness) error {
	requestRecorder := proxy_server.StartProxyServer(stageHarness)
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

	// Assertion 1: the model can use a description it was never given the name for.
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

	if err := matchTestCase.Run(stageHarness); err != nil {
		return err
	}

	// Assertion 2: every skill was advertised, not just the one that matched.
	stageLogger.Infof("Checking that all %d skills reached the request", skillCount)

	return catalogAssertion(skills).Run(requestRecorder.RequestBodies(), stageLogger)
}

// catalogAssertion separates a submission that advertises the whole catalog from
// one that advertises part of it. Matching a task only proves the skill it
// matched was there, and a program that finds one skill and stops would pass
// that whenever the one it found is the one asked about.
//
// This reads the request rather than the answer, because the model has no reason
// to recite a catalog it wasn't asked about. Asking it to count them instead puts
// the stage at the mercy of how it words a number.
func catalogAssertion(skills []skills_manager.Skill) request_assertion.SomeRequestAssertion {
	expectedValues := make([]string, 0, len(skills)*2)

	for _, skill := range skills {
		expectedValues = append(expectedValues, skill.Name, skill.Description)
	}

	return request_assertion.SomeRequestAssertion{ExpectedValues: expectedValues}
}
