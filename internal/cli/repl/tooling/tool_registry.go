package tooling

import (
	"path/filepath"

	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	"github.com/user/keen-code/internal/filesystem"
	"github.com/user/keen-code/internal/tools"
)

func SetupToolRegistry(
	workingDir string,
	appState *replappstate.AppState,
	permissionRequester *replpermissions.Requester,
	diffEmitter *DiffEmitter,
) {
	gitAwareness := filesystem.NewGitAwareness()
	_ = gitAwareness.LoadGitignore(filepath.Join(workingDir, ".gitignore"))
	guard := filesystem.NewGuard(workingDir, gitAwareness)

	readFileTool := tools.NewReadFileTool(guard, permissionRequester)
	appState.RegisterTool(readFileTool)

	globTool := tools.NewGlobTool(guard, permissionRequester)
	appState.RegisterTool(globTool)

	grepTool := tools.NewGrepTool(guard, permissionRequester)
	appState.RegisterTool(grepTool)

	writeFileTool := tools.NewWriteFileTool(guard, diffEmitter, permissionRequester)
	appState.RegisterTool(writeFileTool)

	editFileTool := tools.NewEditFileTool(guard, diffEmitter, permissionRequester)
	appState.RegisterTool(editFileTool)

	bashTool := tools.NewBashTool(guard, permissionRequester)
	appState.RegisterTool(bashTool)
}
