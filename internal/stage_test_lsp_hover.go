package internal

import "github.com/codecrafters-io/tester-utils/test_case_harness"

func testLSPHover(stageHarness *test_case_harness.TestCaseHarness) error {
	return testLSP(stageHarness, "hv2")
}
