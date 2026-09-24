package internal

import (
	"fmt"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/request_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/skills_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

// testSkillsFork can't be judged from stdout: honouring context: fork changes
// where the work happened, not the answer.
func testSkillsFork(stageHarness *test_case_harness.TestCaseHarness) error {
	requestRecorder := proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	const seededSkillCount = 2

	names := skills_manager.RandomNames(seededSkillCount)
	tokens := skills_manager.RandomTokens(seededSkillCount)
	topics := skills_manager.RandomDescriptionTopics(seededSkillCount)

	forkedSkill := skills_manager.Skill{
		Name:               names[0],
		Description:        topics[0].Description,
		Body:               skills_manager.RespondWithTokenBody(tokens[0]),
		RunInForkedContext: true,
	}

	// Seeded but never invoked, so the catalog holds more than the skill under test.
	catalogOnlySkill := skills_manager.Skill{
		Name:        names[1],
		Description: topics[1].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[1]),
	}

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{forkedSkill, catalogOnlySkill}, stageLogger)

	stageLogger.Debugf("Seeded %q alongside it, which stays in the catalog and is never invoked", catalogOnlySkill.Name)
	stageLogger.Infof("Invoking /%s, expecting its body to be handled by a subagent", forkedSkill.Name)

	forkTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      fmt.Sprintf("/%s", forkedSkill.Name),
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ContainsAllAssertion{
			ExpectedValues: []string{tokens[0]},
		},
	}

	if err := forkTestCase.Run(stageHarness); err != nil {
		return err
	}

	subagentReceivedTheBodyAssertion, mainConversationReceivedOnlyTheResultAssertion := forkAssertions(tokens[0])

	recordedRequestBodies := requestRecorder.RequestBodies()
	stageLogger.Debugf("User's program sent %d request(s) to the LLM", len(recordedRequestBodies))

	if err := subagentReceivedTheBodyAssertion.Run(recordedRequestBodies, stageLogger); err != nil {
		return err
	}

	return mainConversationReceivedOnlyTheResultAssertion.Run(recordedRequestBodies, stageLogger)
}

// forkAssertions returns the pair that separates a submission that ran the skill
// in a subagent from one that ran it inline: some conversation was handed the
// body, and some conversation was handed the answer without the body.
//
// The marker is the body rather than another skill's name because a forked
// subagent is still shown the full skill catalog.
func forkAssertions(token string) (request_assertion.SomeRequestAssertion, request_assertion.SomeRequestAssertion) {
	return request_assertion.SomeRequestAssertion{
		ExpectedValues: []string{skills_manager.RespondWithTokenBodyMarker},
	}, request_assertion.SomeRequestAssertion{
		ExpectedValues:   []string{token},
		UnexpectedValues: []string{skills_manager.RespondWithTokenBodyMarker},
	}
}
