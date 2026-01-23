package workspace_manager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

// WorkspaceFile represents a file inside the workspace directory
// RelatiePath is relative to the workspace directory
// Content and FileMode represents the file contents, and file mode & permissions respectively
type WorkspaceFile struct {
	RelativePath string
	Content      string
	FileMode     os.FileMode
}

type WorkspaceManager struct {
	workspaceDirPath string
}

func NewWorkspaceManager() *WorkspaceManager {
	return &WorkspaceManager{}
}

func (w *WorkspaceManager) GetWorkspaceDirPath() string {
	return w.workspaceDirPath
}

func (w *WorkspaceManager) BootstrapExecutableWorkspace(stageHarness *test_case_harness.TestCaseHarness) {
	w.workspaceDirPath = utils.CreateTempDirWithPrefix("workspace")

	stageHarness.RegisterTeardownFunc(func() {
		os.RemoveAll(w.workspaceDirPath)
	})

	// Convert path to absolute: This is done to resolve the relative path to absolute early on
	// since we set the executable's working directory to be a random one
	absolutePath, err := filepath.Abs(stageHarness.Executable.Path)

	if err != nil {
		panic("Codecrafters Internal Error - Failed to convert executable path to absolute: " + err.Error())
	}

	stageHarness.Executable.Path = absolutePath
	stageHarness.Executable.WorkingDir = w.workspaceDirPath
}

// MustCreateDir creates a directory inside the workspace
// The is treated as relative to the workspace directory
// It creates all parent directories if needed
func (w *WorkspaceManager) MustCreateDir(relPath string) {
	if filepath.IsAbs(relPath) {
		panic(fmt.Sprintf("Codecrafters Internal Error - MustCreateFiles: File path %q is absolute", relPath))
	}

	absolutePath := filepath.Join(w.workspaceDirPath, relPath)

	if err := os.MkdirAll(absolutePath, 0755); err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed to create directory %s: %s", relPath, err))
	}
}

func (w *WorkspaceManager) MustCreateFileWithLogger(file WorkspaceFile, logger *logger.Logger) {
	if filepath.IsAbs(file.RelativePath) {
		panic(fmt.Sprintf(
			"Codecrafters Internal Error - MustCreateFiles: File path %q is absolute",
			file.RelativePath,
		))
	}

	absoluteFilePath := filepath.Join(w.workspaceDirPath, file.RelativePath)

	err := os.WriteFile(absoluteFilePath, []byte(file.Content), file.FileMode)

	if err == nil {
		logger.WithAdditionalSecondaryPrefix(file.RelativePath, func() {
			logger.Plainf("%s", file.Content)
		})
		return
	}

	// If parent directory doesn't exist, try again
	if errors.Is(err, os.ErrNotExist) {
		parentDir := filepath.Dir(file.RelativePath)
		w.MustCreateDir(parentDir)
		w.MustCreateFileWithLogger(file, logger)
		return
	}

	panic(fmt.Sprintf(
		"Codecrafters Internal Error - MustCreateFiles: Could not create file %q: %s",
		absoluteFilePath,
		err,
	))
}

// MustCreateFile creates the specified files
// If the parent directory of the file does not exist, it creates it
func (w *WorkspaceManager) MustCreateFilesWithLogger(files []WorkspaceFile, logger *logger.Logger) {
	logger.Infof("Creating workspace files")
	for _, file := range files {
		w.MustCreateFileWithLogger(file, logger)
	}
}

// ConvertToAbsPath converts relative path (to workspace dir) to absolute path
func (w *WorkspaceManager) ConvertToAbsPath(relPath string) string {
	return filepath.Join(w.workspaceDirPath, relPath)
}

// GetRelPathConverter returns a function that converts absolute path to relative path (to workspace dir)
func (w *WorkspaceManager) GetRelPathConverter() func(string) string {
	return func(absolutePath string) string {
		relPath, err := filepath.Rel(w.workspaceDirPath, absolutePath)

		if err != nil {
			panic(fmt.Sprintf(
				"Codecrafters Internal Error - failed to convert %q to relative path from %q: %s",
				absolutePath,
				w.workspaceDirPath,
				err,
			))
		}

		return relPath
	}
}
