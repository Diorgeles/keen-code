package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	keenauth "github.com/user/keen-code/internal/auth"
	"github.com/user/keen-code/internal/cli/repl"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/providers"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keen",
		Short: "Keen - A coding agent CLI",
		Long:  `Keen is a terminal-based coding agent that provides AI-assisted code editing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, loader, globalCfg, resolvedCfg, needsSetup, err := loadRootRuntime()
			if err != nil {
				return err
			}
			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}

			return repl.RunREPL(version, wd, resolvedCfg, loader, globalCfg, registry, needsSetup)
		},
	}

	cmd.Version = version
	cmd.AddCommand(newRunCommand())
	return cmd
}

func newRunCommand() *cobra.Command {
	var sessionID string
	var format string

	runCmd := &cobra.Command{
		Use:   "run [flags] <message...>",
		Short: "Run one non-interactive Keen turn",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, resolvedCfg, needsSetup, err := loadRootRuntime()
			if err != nil {
				return err
			}
			if needsSetup {
				return fmt.Errorf("LLM client not initialized. Run keen to configure a provider")
			}

			stdin := ""
			if shouldReadStdin(os.Stdin) {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				stdin = string(data)
			}
			prompt := buildRunPrompt(args, stdin)
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}
			client, err := llm.NewClient(resolvedCfg)
			if err != nil {
				return err
			}
			_, err = repl.RunHeadless(context.Background(), repl.HeadlessRunOptions{
				WorkingDir: wd,
				Config:     resolvedCfg,
				Client:     client,
				SessionID:  sessionID,
				Prompt:     prompt,
				Format:     format,
				Out:        cmd.OutOrStdout(),
			})
			return err
		},
	}
	runCmd.Flags().StringVar(&sessionID, "session", "", "resume an existing Keen session")
	runCmd.Flags().StringVar(&format, "format", repl.HeadlessFormatText, "output format: text or json")
	return runCmd
}

func loadRootRuntime() (*providers.Registry, *config.Loader, *config.GlobalConfig, *config.ResolvedConfig, bool, error) {
	registry, err := providers.Load()
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("failed to load provider registry: %w", err)
	}
	loader := config.NewLoader()
	globalCfg, err := loader.Load()
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("failed to load config: %w", err)
	}

	if globalCfg.ActiveProvider == "" {
		return registry, loader, globalCfg, &config.ResolvedConfig{}, true, nil
	}

	_, ok := registry.GetProvider(globalCfg.ActiveProvider)
	if !ok {
		return nil, nil, nil, nil, false, fmt.Errorf("configured provider %q not found in registry", globalCfg.ActiveProvider)
	}
	providerCfg, ok := globalCfg.GetProviderConfig(globalCfg.ActiveProvider)
	if !ok {
		return nil, nil, nil, nil, false, fmt.Errorf("failed to get provider config for %q", globalCfg.ActiveProvider)
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider:       globalCfg.ActiveProvider,
		Model:          globalCfg.ActiveModel,
		APIKey:         providerCfg.APIKey,
		ThinkingEffort: globalCfg.ThinkingEffort,
		BaseURL:        providerCfg.BaseURL,
		AuthMode:       config.AuthModeForProvider(globalCfg.ActiveProvider),
	}
	needsSetup := resolvedCfg.AuthMode == config.AuthModeOAuth && !keenauth.NewOAuthManager(nil).HasCredential(globalCfg.ActiveProvider)
	return registry, loader, globalCfg, resolvedCfg, needsSetup, nil
}

func buildRunPrompt(args []string, stdin string) string {
	argText := strings.TrimSpace(strings.Join(args, " "))
	stdin = strings.TrimSpace(stdin)
	switch {
	case argText != "" && stdin != "":
		return argText + "\n" + stdin
	case argText != "":
		return argText
	default:
		return stdin
	}
}

func shouldReadStdin(stdin *os.File) bool {
	info, err := stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
