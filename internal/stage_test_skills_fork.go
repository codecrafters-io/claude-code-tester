package internal

import (
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
	// The skill is matched by description rather than invoked by name. A named
	// invocation is resolved without asking the model, so the main conversation
	// never calls it and never receives the subagent's answer.
	topics := skills_manager.RandomInvocationTopics(seededSkillCount)

	forkedSkill := skills_manager.Skill{
		Name:               names[0],
		Description:        topics[0].Description,
		Body:               skills_manager.RespondWithTokenBody(tokens[0]),
		RunInForkedContext: true,
	}

	// Seeded but never matched, so the catalog holds more than the skill under test.
	catalogOnlySkill := skills_manager.Skill{
		Name:        names[1],
		Description: topics[1].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[1]),
	}

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{forkedSkill, catalogOnlySkill}, stageLogger)

	stageLogger.Debugf("Seeded %q alongside it, which stays in the catalog and is never matched", catalogOnlySkill.Name)
	stageLogger.Infof("Sending a request that matches %q, expecting its body to be handled by a subagent", forkedSkill.Name)

	forkTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      topics[0].Question,
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
