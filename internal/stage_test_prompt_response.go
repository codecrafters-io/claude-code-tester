package internal

import (
	"fmt"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testPromptResponse(stageHarness *test_case_harness.TestCaseHarness) error {
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeSettings(stageHarness, utils.GetBypassPermissionsSettings())

	operand1 := random.RandomInt(1, 11)
	operand2 := random.RandomInt(1, 11)
	operator := random.RandomElementFromArray([]string{"+", "-", "*"})
	result := getOperationResult(operator, operand1, operand2)

	prompt := utils.GetPromptWithGuardRailPrompt(
		[]string{
			fmt.Sprintf("What is the result of %d%s%d?", operand1, operator, operand2),
			fmt.Sprintf("Calculate %d %s %d and tell me the answer.", operand1, operator, operand2),
			fmt.Sprintf("Can you solve this math problem: %d%s%d?", operand1, operator, operand2),
			fmt.Sprintf("I need help with arithmetic. What does %d%s%d equal?", operand1, operator, operand2),
			fmt.Sprintf("Please compute %d%s%d and provide the result.", operand1, operator, operand2),
			fmt.Sprintf("What's the answer to %d%s%d?", operand1, operator, operand2),
		},
		"Respond ONLY with the final result in number. No backticks or punctuations.",
	)

	promptTestCase := test_cases.PrintFlagTestCase{
		InputPrompt: prompt,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: fmt.Sprintf("%d", result),
		},
	}

	return promptTestCase.Run(stageHarness)
}

// getOperationResult returns the integer result of (operand1 <operator> operand2)
func getOperationResult(operator string, operand1, operand2 int) int {
	switch operator {
	case "+":
		return operand1 + operand2
	case "-":
		return operand1 - operand2
	case "*":
		return operand1 * operand2
	default:
		panic(fmt.Sprintf("Codecrafters Internal Error - Operation %s not implemented", operator))
	}
}
