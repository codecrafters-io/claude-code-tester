package test_cases

import (
	"fmt"
	"path/filepath"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

type NonInteractiveTestCase struct {
	InputPrompt      string
	StdoutAssertion  string_assertion.StringAssertion
	ExpectedExitCode int
}

func (t *NonInteractiveTestCase) Run(stageHarness *test_case_harness.TestCaseHarness) error {
	executable := stageHarness.Executable
	logger := stageHarness.Logger
	shellEscapedPrompt := shellescape.Quote(t.InputPrompt)
	logger.Infof("$ ./%s -p %s", filepath.Base(executable.Path), shellEscapedPrompt)
	result, err := executable.Run("-p", t.InputPrompt)

	if err != nil {
		return err
	}

	if result.ExitCode != t.ExpectedExitCode {
		return fmt.Errorf("Expected program to exit with exit code %d, got %d instead", t.ExpectedExitCode, result.ExitCode)
	}

	stdoutContent := strings.TrimSpace(string(result.Stdout))

	if err := t.StdoutAssertion.Run(stdoutContent); err != nil {
		return err
	}

	return nil
}
