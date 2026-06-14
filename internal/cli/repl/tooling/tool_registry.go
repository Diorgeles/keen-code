package tooling

import (
	"path/filepath"

	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/filesystem"
	"github.com/user/keen-code/internal/llm"
	keenmcp "github.com/user/keen-code/internal/mcp"
	"github.com/user/keen-code/internal/subagents"
	"github.com/user/keen-code/internal/tools"
)

func SetupToolRegistry(
	workingDir string,
	appState *replappstate.AppState,
	permissionRequester *replpermissions.Requester,
	diffEmitter *DiffEmitter,
	mcpRuntime keenmcp.Runtime,
	cfg *config.ResolvedConfig,
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

	webFetchTool := tools.NewWebFetchTool()
	appState.RegisterTool(webFetchTool)

	if mcpRuntime != nil {
		appState.RegisterTool(tools.NewCallMCPTool(mcpRuntime, permissionRequester))
	}

	runner := &subagents.Runner{
		WorkingDir: workingDir,
		Config:     cfg,
		GetProfiles: func() []subagents.Profile {
			return appState.GetSubagents().Profiles
		},
		NewClient: llm.NewClient,
		Registry:  appState.GetToolRegistry(),
	}
	appState.RegisterTool(tools.NewDelegateTool(runner))
}
