package internal

import "github.com/codecrafters-io/tester-utils/test_case_harness"

func testLSPMigration(stageHarness *test_case_harness.TestCaseHarness) error {
	return testLSP(stageHarness, "ql9")
}
