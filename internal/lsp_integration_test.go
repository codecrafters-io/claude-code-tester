package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Reference-agent feedback and document opening obey the client contract.
func TestLSPReferenceClientBehavior(t *testing.T) {
	// Arrange
	command := exec.Command("python3", "-m", "unittest", "discover", "-s", "test_helpers/scenarios/lsp_stages/users_code_pass_all/app", "-p", "test_*.py")
	// Act
	output, err := command.CombinedOutput()
	// Assert
	require.NoError(t, err, string(output))
}

// LSP grading requires the real provider instead of falling back to canned model responses.
func TestLSPRequiresModelCredentials(t *testing.T) {
	// Arrange
	binary := lspFixtureBinary(t)
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "codecrafters.yml"), []byte("debug: false\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "your_program.sh"), []byte("#!/bin/sh\nexit 99\n"), 0755))
	command := exec.Command(binary)
	command.Env = append(os.Environ(),
		"CODECRAFTERS_SECRET_OPENROUTER_API_KEY=",
		"CODECRAFTERS_REPOSITORY_DIR="+directory,
		`CODECRAFTERS_TEST_CASES_JSON=[{"slug":"dz5","tester_log_prefix":"lsp-test","title":"LSP connection"}]`,
	)
	// Act
	output, err := command.CombinedOutput()
	// Assert
	require.Error(t, err, string(output))
	require.Contains(t, string(output), "CODECRAFTERS_SECRET_OPENROUTER_API_KEY environment variable not set")
}
