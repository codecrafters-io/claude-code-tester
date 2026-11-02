package main

import (
	"os"
	"strings"

	"github.com/codecrafters-io/claude-code-tester/internal"
	"github.com/codecrafters-io/claude-code-tester/internal/lsp_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/lsp_transport"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--lsp-transport" {
		os.Exit(lsp_transport.Run(os.Args[2:]))
	}
	if len(os.Args) == 3 && os.Args[1] == "--lsp-fixture" {
		os.Exit(lsp_manager.RunServer(os.Args[2]))
	}
	os.Exit(internal.RunCLI(envMap()))
}

func envMap() map[string]string {
	result := make(map[string]string)
	for _, keyVal := range os.Environ() {
		split := strings.SplitN(keyVal, "=", 2)
		key, val := split[0], split[1]
		result[key] = val
	}

	return result
}
