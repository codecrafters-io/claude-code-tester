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

// testSkillsFork is the one skills stage that can't be judged from stdout.
//
// Honouring context: fork doesn't change the answer, only where the work
// happened, so a correct submission and one that ignores the field print the
// same thing. The recorded requests are the only place the difference exists.
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

	// Seeded but never invoked. Its name reaches the model only through the
	// skill catalog, which is what makes it the marker that tells the two
	// conversations apart: the main one carries the catalog, the forked one is
	// built from the skill's body alone.
	//
	// Forked skills can't be stacked behind another invocation, so a second
	// skill's body is not available as a marker the way it is in the stacking
	// stage.
	catalogOnlySkill := skills_manager.Skill{
		Name:        names[1],
		Description: topics[1].Description,
		Body:        skills_manager.RespondWithTokenBody(tokens[1]),
	}

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{forkedSkill, catalogOnlySkill}, stageLogger)

	stageLogger.Debugf("Seeded %q alongside it, whose name appears only in the skill catalog", catalogOnlySkill.Name)
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

	// A request carrying the forked body without the catalog is a conversation
	// that didn't inherit the main one. A program that ignores the field puts
	// both in a single request, so no request satisfies this.
	forkedSkillRanInIsolationAssertion := request_assertion.SomeRequestAssertion{
		ExpectedValues:   []string{tokens[0]},
		UnexpectedValues: []string{catalogOnlySkill.Name},
	}

	// The mirror: the main conversation has to receive what the fork produced.
	// A program that forks and then prints the result without going back to the
	// main conversation satisfies the assertion above but not this one.
	forkResultReachedMainConversationAssertion := request_assertion.SomeRequestAssertion{
		ExpectedValues: []string{catalogOnlySkill.Name, tokens[0]},
	}

	recordedRequestBodies := requestRecorder.RequestBodies()
	stageLogger.Debugf("User's program sent %d request(s) to the LLM", len(recordedRequestBodies))

	if err := forkedSkillRanInIsolationAssertion.Run(recordedRequestBodies, stageLogger); err != nil {
		return err
	}

	return forkResultReachedMainConversationAssertion.Run(recordedRequestBodies, stageLogger)
}
