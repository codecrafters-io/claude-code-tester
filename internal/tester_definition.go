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
			Timeout:  30 * time.Second,
		},
		{
			Slug:     "oz7",
			TestFunc: testWriteTool,
			// This one occassionally times out for 30s
			Timeout: 45 * time.Second,
		},
		{
			Slug:     "bp2",
			TestFunc: testGlobTool,
			// This one occassionally times out for 30s
			Timeout: 45 * time.Second,
		},
		{
			Slug:     "oq5",
			TestFunc: testBashTool,
			Timeout:  30 * time.Second,
		},
	},
}
