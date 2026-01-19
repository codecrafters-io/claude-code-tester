package internal

import (
	"os"
	"testing"

	tester_utils_testing "github.com/codecrafters-io/tester-utils/testing"
)

func TestStages(t *testing.T) {
	os.Setenv("CODECRAFTERS_RANDOM_SEED", "1234567890")

	testCases := map[string]tester_utils_testing.TesterOutputTestCase{
		"base_stages_pass": {
			StageSlugs:          []string{"yy2", "aq1", "md6", "yp5", "ff2", "oz7", "bp2", "oq5"},
			CodePath:            "./test_helpers/pass_all",
			ExpectedExitCode:    0,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/success",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"stage_1_fail": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/stage_1_fail",
			ExpectedExitCode:    1,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/stage_1_fail",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"wrong_model_usage": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/wrong_model_usage",
			ExpectedExitCode:    1,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/wrong_model_usage",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
		"unauthorized_endpoint_access": {
			StageSlugs:          []string{"yy2"},
			CodePath:            "./test_helpers/scenarios/base_stages/unauthorized_endpoint_access",
			ExpectedExitCode:    1,
			StdoutFixturePath:   "./test_helpers/fixtures/base_stages/unauthorized_endpoint_access",
			NormalizeOutputFunc: normalizeTesterOutput,
		},
	}

	tester_utils_testing.TestTesterOutput(t, testerDefinition, testCases)
}

func normalizeTesterOutput(testerOutput []byte) []byte {
	return testerOutput
}
