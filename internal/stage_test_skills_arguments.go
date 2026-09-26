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

const noExtraTextInstruction = "Do not add any other text, punctuation, or formatting."

func testSkillsArguments(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	names := skills_manager.RandomNames(2)
	topics := skills_manager.RandomDescriptionTopics(2)

	argumentsSkill := skills_manager.Skill{
		Name:        names[0],
		Description: topics[0].Description,
		Body:        fmt.Sprintf("Respond with exactly: $ARGUMENTS\n\n%s", noExtraTextInstruction),
	}

	// Positional placeholders are zero-based, and $1 comes before $0 on purpose:
	// substituting in the order the arguments arrive, rather than by index,
	// produces the wrong answer here.
	positionalSkill := skills_manager.Skill{
		Name:        names[1],
		Description: topics[1].Description,
		Body:        fmt.Sprintf("Respond with exactly: $1 $0\n\n%s", noExtraTextInstruction),
	}

	skills_manager.Seed(workspaceManager, []skills_manager.Skill{argumentsSkill, positionalSkill}, stageLogger)

	// Assertion 1: $ARGUMENTS receives the whole argument string.
	argumentValue := random.RandomInt(1000, 10000)
	stageLogger.Infof("Checking that $ARGUMENTS is substituted")

	// Substituted or not is what separates a passing submission from a failing
	// one, and that shows in whether the value appears at all. Insisting the
	// output is nothing but the value fails submissions over the model's prose.
	argumentsTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      fmt.Sprintf("/%s %d", argumentsSkill.Name, argumentValue),
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ContainsAllAssertion{
			ExpectedValues: []string{fmt.Sprintf("%d", argumentValue)},
		},
	}

	if err := argumentsTestCase.Run(stageHarness); err != nil {
		return err
	}

	// Assertion 2: $0 and $1 resolve positionally, in the order the body asks for.
	positionalArguments := skills_manager.RandomTokens(2)
	stageLogger.Infof("Checking that $0 and $1 are substituted positionally")

	// The pair in the order the body asks for, which a submission that
	// substitutes by arrival rather than by index never produces.
	positionalTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      fmt.Sprintf("/%s %s %s", positionalSkill.Name, positionalArguments[0], positionalArguments[1]),
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ContainsAllAssertion{
			ExpectedValues: []string{fmt.Sprintf("%s %s", positionalArguments[1], positionalArguments[0])},
		},
	}

	return positionalTestCase.Run(stageHarness)
}
