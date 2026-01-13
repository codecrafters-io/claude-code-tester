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
			Timeout:  time.Minute,
		},
		{
			Slug:     "md6",
			TestFunc: testExecuteReadTool,
			Timeout:  time.Minute,
		},
		{
			Slug:     "yp5",
			TestFunc: testExecuteMultipleReadTools,
			Timeout:  time.Minute,
		},
		{
			Slug:     "ff2",
			TestFunc: testConversationalLoop,
			Timeout:  time.Minute,
		},
		{
			Slug:     "oz7",
			TestFunc: testWriteTool,
			Timeout:  time.Minute,
		},
		{
			Slug:     "bp2",
			TestFunc: testEditTool,
			Timeout:  time.Minute,
		},
		{
			Slug:     "oq5",
			TestFunc: testBashTool,
			Timeout:  time.Minute,
		},
	},
}
