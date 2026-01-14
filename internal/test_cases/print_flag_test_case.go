package test_cases

import (
	"path/filepath"
	"strings"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

type PrintFlagTestCase struct {
	InputPrompt     string
	StdoutAssertion string_assertion.StringAssertion
}

func (t *PrintFlagTestCase) Run(stageHarness *test_case_harness.TestCaseHarness) error {
	executable := stageHarness.Executable
	logger := stageHarness.Logger
	logger.Infof("$ ./%s -p \"%s\"", filepath.Base(executable.Path), t.InputPrompt)
	result, err := executable.Run("-p", t.InputPrompt)

	if err != nil {
		return err
	}

	stdoutContent := strings.TrimSpace(string(result.Stdout))

	if err := t.StdoutAssertion.Run(stdoutContent); err != nil {
		return err
	}

	return nil
}
