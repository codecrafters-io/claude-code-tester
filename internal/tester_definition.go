package internal

import (
	"time"

	"github.com/codecrafters-io/tester-utils/tester_definition"
)

var testerDefinition = tester_definition.TesterDefinition{
	AntiCheatTestCases: []tester_definition.TestCase{},
	ExecutableFileName: "your_claude_code.sh",
	TestCases: []tester_definition.TestCase{
		{
			Slug:     "yy2",
			TestFunc: testPromptResponse,
			Timeout:  30 * time.Second,
		},
		{
			Slug:     "aq1",
			TestFunc: testAdvertiseReadTool,
		},
		{
			Slug:     "md6",
			TestFunc: testExecuteReadTool,
		},
		{
			Slug:     "yp5",
			TestFunc: testExecuteMultipleReadTools,
		},
		{
			Slug:     "ff2",
			TestFunc: testConversationalLoop,
		},
		{
			Slug:     "oz7",
			TestFunc: testWriteTool,
		},
		{
			Slug:     "bp2",
			TestFunc: testEditTool,
		},
		{
			Slug:     "oq5",
			TestFunc: testBashTool,
		},
	},
}
