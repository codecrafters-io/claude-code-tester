package internal

import (
	"time"

	"github.com/codecrafters-io/tester-utils/tester_definition"
)

var testerDefinition = tester_definition.TesterDefinition{
	AntiCheatTestCases: []tester_definition.TestCase{},
	ExecutableFileName: "your_program.sh",
	TestCases: []tester_definition.TestCase{
		{
			Slug:     "yy2",
			TestFunc: testPromptResponse,
			Timeout:  30 * time.Second,
		},
		{
			Slug:     "aq1",
			TestFunc: testAdvertiseReadTool,
			Timeout:  30 * time.Second,
		},
		{
			Slug:     "md6",
			TestFunc: testExecuteReadTool,
			Timeout:  30 * time.Second,
		},
		{
			Slug:     "ff2",
			TestFunc: testAgentLoop,
			Timeout:  45 * time.Second,
		},
		{
			Slug:     "oz7",
			TestFunc: testWriteTool,
			Timeout:  45 * time.Second,
		},
		{
			Slug:     "oq5",
			TestFunc: testBashTool,
			Timeout:  45 * time.Second,
		},

		// Extension: Skills
		//
		// Stages that run the user's program twice get roughly double the
		// executable timeout, plus headroom for workspace setup.
		{
			Slug:     "vh1",
			TestFunc: testSkillsAdvertise,
			Timeout:  75 * time.Second,
		},
		{
			Slug:     "jd8",
			TestFunc: testSkillsInvoke,
			Timeout:  40 * time.Second,
		},
		{
			Slug:     "wd2",
			TestFunc: testSkillsArguments,
			Timeout:  75 * time.Second,
		},
		{
			Slug:     "sk5",
			TestFunc: testSkillsStack,
			Timeout:  45 * time.Second,
		},
		{
			Slug:     "tq1",
			TestFunc: testSkillsScript,
			Timeout:  60 * time.Second,
		},
		{
			Slug:     "gq2",
			TestFunc: testSkillsModelInvoked,
			Timeout:  60 * time.Second,
		},
		{
			Slug:     "mj2",
			TestFunc: testSkillsFork,
			Timeout:  60 * time.Second,
		},
	},
}
