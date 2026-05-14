// Benchmark runner that compares Keen and OpenCode on a target repo.
// Run with: go run bench/run.go --repo /path/to/repo --run-id bench-YYYYMMDD-NNN
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const defaultOpencodeModel = "opencode-go/kimi-k2.6"
const defaultKeenProvider = "opencode-go"
const defaultKeenModel = "kimi-k2.6"
const defaultTurnTimeout = 10 * time.Minute

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Config struct {
	TargetRepo    string
	RunID         string
	TasksFile     string
	OpencodeModel string
	KeenProvider  string
	KeenModel     string
	Smoke         bool
	TurnTimeout   time.Duration

	BenchDir     string
	RunDir       string
	ResultDir    string
	WorktreeDir  string
	KeenWorktree string
	OcWorktree   string

	KeenCommit string
	OcCommit   string
}

type TasksFile struct {
	Tasks []Task `json:"tasks"`
}

type Task struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Turns []string `json:"turns"`
}

type TargetRepo struct {
	Path   string `json:"path"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

type Usage struct {
	Input      int     `json:"input"`
	Output     int     `json:"output"`
	Reasoning  int     `json:"reasoning"`
	CacheRead  int     `json:"cache_read"`
	CacheWrite int     `json:"cache_write"`
	Total      int     `json:"total"`
	Cost       float64 `json:"cost"`
}

type Record struct {
	RunID       string     `json:"run_id"`
	TaskID      string     `json:"task_id"`
	TaskTitle   string     `json:"task_title"`
	Tool        string     `json:"tool"`
	Turn        int        `json:"turn"`
	Prompt      string     `json:"prompt"`
	TargetRepo  TargetRepo `json:"target_repo"`
	Command     []string   `json:"command"`
	CWD         string     `json:"cwd"`
	StartedAt   string     `json:"started_at"`
	FinishedAt  string     `json:"finished_at"`
	DurationMs  int64      `json:"duration_ms"`
	ExitCode    int        `json:"exit_code"`
	SessionID   string     `json:"session_id"`
	Text        string     `json:"text"`
	Usage       Usage      `json:"usage"`
	RawStdout   string     `json:"raw_stdout"`
	RawStderr   string     `json:"raw_stderr"`
	DirtyStatus string     `json:"dirty_status"`
	Status      string     `json:"status"`
	Error       string     `json:"error"`
}

type TaskSummary struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	TurnsCompleted  int    `json:"turns_completed"`
	TurnsTotal      int    `json:"turns_total"`
	KeenSession     string `json:"keen_session"`
	OpencodeSession string `json:"opencode_session"`
}

type Summary struct {
	RunID           string            `json:"run_id"`
	Status          string            `json:"status"`
	TargetRepo      string            `json:"target_repo"`
	Ref             string            `json:"ref"`
	TasksFile       string            `json:"tasks_file"`
	TaskCount       int               `json:"task_count"`
	OpencodeModel   string            `json:"opencode_model"`
	KeenProvider    string            `json:"keen_provider"`
	KeenModel       string            `json:"keen_model"`
	WorktreeCommits map[string]string `json:"worktree_commits"`
	ToolVersions    map[string]string `json:"tool_versions"`
	StartedAt       string            `json:"started_at"`
	FinishedAt      string            `json:"finished_at"`
	Tasks           []TaskSummary     `json:"tasks"`
}

type keenOutput struct {
	SessionID string     `json:"session_id"`
	Text      string     `json:"text"`
	Usage     *keenUsage `json:"usage"`
}

type keenUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
	TotalTokens     int `json:"total_tokens"`
	CachedTokens    int `json:"cached_tokens"`
}

type opencodeParsed struct {
	SessionID           string
	Text                string
	Usage               Usage
	PermissionRequested bool
	SessionError        string
	ParseErr            error
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseFlags()
	if err != nil {
		return err
	}
	if err := validateEnvironment(cfg); err != nil {
		return err
	}
	tasks, err := loadAndValidateTasks(cfg.TasksFile, cfg.Smoke)
	if err != nil {
		return err
	}
	if err := prepareDirs(cfg); err != nil {
		return err
	}
	defer cleanup(cfg)

	runTasksFile := filepath.Join(cfg.RunDir, "tasks.json")
	if err := copyFile(cfg.TasksFile, runTasksFile); err != nil {
		return fmt.Errorf("copy tasks file: %w", err)
	}

	keenCommit, ocCommit, err := setupWorktrees(cfg)
	if err != nil {
		return err
	}
	cfg.KeenCommit = keenCommit
	cfg.OcCommit = ocCommit

	startedAt := time.Now().UTC()

	fmt.Printf("Running benchmark %s\n", cfg.RunID)
	fmt.Printf("Target repo:  %s\n", cfg.TargetRepo)
	fmt.Printf("Tasks:        %s\n", cfg.TasksFile)
	fmt.Printf("Run dir:      %s\n", cfg.RunDir)
	fmt.Printf("Results:      %s\n\n", cfg.ResultDir)

	taskSummaries := make([]TaskSummary, 0, len(tasks.Tasks))
	overallStatus := "ok"

	for i, task := range tasks.Tasks {
		ts, taskErr := runTask(cfg, task, i)
		taskSummaries = append(taskSummaries, ts)
		if taskErr != nil {
			overallStatus = "failed"
			fmt.Fprintln(os.Stderr, taskErr)
			break
		}
	}

	finishedAt := time.Now().UTC()

	summary := Summary{
		RunID:         cfg.RunID,
		Status:        overallStatus,
		TargetRepo:    cfg.TargetRepo,
		Ref:           "main",
		TasksFile:     runTasksFile,
		TaskCount:     len(tasks.Tasks),
		OpencodeModel: cfg.OpencodeModel,
		KeenProvider:  cfg.KeenProvider,
		KeenModel:     cfg.KeenModel,
		WorktreeCommits: map[string]string{
			"keen":     cfg.KeenCommit,
			"opencode": cfg.OcCommit,
		},
		ToolVersions: collectToolVersions(),
		StartedAt:    isoTime(startedAt),
		FinishedAt:   isoTime(finishedAt),
		Tasks:        taskSummaries,
	}

	if err := writeJSONFile(filepath.Join(cfg.ResultDir, "summary.json"), summary); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	sessionsText := buildSessionsText(cfg, taskSummaries)
	if err := os.WriteFile(filepath.Join(cfg.ResultDir, "sessions.txt"), []byte(sessionsText), 0o644); err != nil {
		return fmt.Errorf("write sessions.txt: %w", err)
	}
	fmt.Print(sessionsText)

	if overallStatus != "ok" {
		return errors.New("benchmark failed")
	}
	return nil
}

func parseFlags() (*Config, error) {
	benchDir := deriveBenchDir()
	defaultTasks := filepath.Join(benchDir, "tasks.json")

	repo := flag.String("repo", "", "Path to target git repository (required)")
	runID := flag.String("run-id", "", "Run ID, e.g. bench-20260512-001 (required)")
	tasks := flag.String("tasks", defaultTasks, "Tasks file path")
	opencodeModel := flag.String("opencode-model", defaultOpencodeModel, "OpenCode model")
	keenProvider := flag.String("keen-provider", defaultKeenProvider, "Keen provider")
	keenModel := flag.String("keen-model", defaultKeenModel, "Keen model")
	smoke := flag.Bool("smoke", false, "Allow a non-standard task file for smoke testing")
	turnTimeout := flag.Duration("turn-timeout", defaultTurnTimeout, "Per-turn timeout")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: go run bench/run.go --repo /path/to/repo --run-id bench-YYYYMMDD-NNN [options]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *repo == "" {
		return nil, errors.New("--repo is required")
	}
	if *runID == "" {
		return nil, errors.New("--run-id is required")
	}
	if !idPattern.MatchString(*runID) {
		return nil, errors.New("--run-id may only contain letters, numbers, dots, underscores, and dashes")
	}
	if *turnTimeout <= 0 {
		return nil, errors.New("--turn-timeout must be greater than zero")
	}

	tasksAbs, err := filepath.Abs(*tasks)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		TargetRepo:    *repo,
		RunID:         *runID,
		TasksFile:     tasksAbs,
		OpencodeModel: *opencodeModel,
		KeenProvider:  *keenProvider,
		KeenModel:     *keenModel,
		Smoke:         *smoke,
		TurnTimeout:   *turnTimeout,
		BenchDir:      benchDir,
	}
	cfg.RunDir = filepath.Join(benchDir, cfg.RunID)
	cfg.ResultDir = filepath.Join(cfg.RunDir, "results")
	cfg.WorktreeDir = filepath.Join(cfg.RunDir, "worktrees")
	cfg.KeenWorktree = filepath.Join(cfg.WorktreeDir, "keen")
	cfg.OcWorktree = filepath.Join(cfg.WorktreeDir, "opencode")
	return cfg, nil
}

func deriveBenchDir() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		dir := filepath.Dir(file)
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	abs, _ := filepath.Abs("bench")
	return abs
}

func validateEnvironment(cfg *Config) error {
	for _, name := range []string{"git", "keen", "opencode"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("missing required command: %s", name)
		}
	}
	abs, err := filepath.Abs(cfg.TargetRepo)
	if err != nil {
		return err
	}
	cfg.TargetRepo = abs
	info, err := os.Stat(cfg.TargetRepo)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("repo does not exist: %s", cfg.TargetRepo)
	}
	if err := exec.Command("git", "-C", cfg.TargetRepo, "rev-parse", "--git-dir").Run(); err != nil {
		return fmt.Errorf("not a git repo: %s", cfg.TargetRepo)
	}
	if err := exec.Command("git", "-C", cfg.TargetRepo, "rev-parse", "--verify", "main^{commit}").Run(); err != nil {
		return fmt.Errorf("target repo has no main branch: %s", cfg.TargetRepo)
	}
	return nil
}

func loadAndValidateTasks(path string, smoke bool) (*TasksFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tasks file not found: %s", path)
	}
	var tf TasksFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("invalid tasks file: %w", err)
	}
	if len(tf.Tasks) == 0 {
		return nil, errors.New("invalid tasks file: tasks must be a non-empty array")
	}
	if !smoke && len(tf.Tasks) != 5 {
		return nil, fmt.Errorf("invalid tasks file: expected 5 tasks, got %d (use --smoke for smaller test files)", len(tf.Tasks))
	}
	seen := map[string]bool{}
	for i, task := range tf.Tasks {
		if !idPattern.MatchString(task.ID) {
			return nil, fmt.Errorf("invalid tasks file: tasks[%d].id %q invalid", i, task.ID)
		}
		if seen[task.ID] {
			return nil, fmt.Errorf("invalid tasks file: duplicate task id %q", task.ID)
		}
		seen[task.ID] = true
		if strings.TrimSpace(task.Title) == "" {
			return nil, fmt.Errorf("invalid tasks file: tasks[%d].title empty", i)
		}
		if len(task.Turns) == 0 {
			return nil, fmt.Errorf("invalid tasks file: tasks[%d].turns empty", i)
		}
		if !smoke && (len(task.Turns) < 5 || len(task.Turns) > 10) {
			return nil, fmt.Errorf("invalid tasks file: tasks[%d].turns must contain 5 to 10 turns, got %d (use --smoke for smaller test files)", i, len(task.Turns))
		}
		for j, turn := range task.Turns {
			if strings.TrimSpace(turn) == "" {
				return nil, fmt.Errorf("invalid tasks file: tasks[%d].turns[%d] empty", i, j)
			}
		}
	}
	return &tf, nil
}

func prepareDirs(cfg *Config) error {
	if _, err := os.Stat(cfg.RunDir); err == nil {
		return fmt.Errorf("run directory already exists: %s", cfg.RunDir)
	}
	if err := os.Mkdir(cfg.RunDir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(cfg.WorktreeDir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(cfg.ResultDir, 0o755); err != nil {
		return err
	}
	return nil
}

func setupWorktrees(cfg *Config) (string, string, error) {
	fmt.Println("Creating worktrees from main...")
	for _, wt := range []string{cfg.KeenWorktree, cfg.OcWorktree} {
		out, err := exec.Command("git", "-C", cfg.TargetRepo, "worktree", "add", "--detach", wt, "main").CombinedOutput()
		if err != nil {
			return "", "", fmt.Errorf("create worktree %s: %w: %s", wt, err, strings.TrimSpace(string(out)))
		}
	}
	keenCommit, err := revParse(cfg.KeenWorktree)
	if err != nil {
		return "", "", fmt.Errorf("read keen worktree commit: %w", err)
	}
	ocCommit, err := revParse(cfg.OcWorktree)
	if err != nil {
		return "", "", fmt.Errorf("read opencode worktree commit: %w", err)
	}
	return keenCommit, ocCommit, nil
}

func cleanup(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.KeenWorktree != "" && pathExists(cfg.KeenWorktree) {
		_ = exec.Command("git", "-C", cfg.TargetRepo, "worktree", "remove", "--force", cfg.KeenWorktree).Run()
	}
	if cfg.OcWorktree != "" && pathExists(cfg.OcWorktree) {
		_ = exec.Command("git", "-C", cfg.TargetRepo, "worktree", "remove", "--force", cfg.OcWorktree).Run()
	}
	if cfg.WorktreeDir != "" && pathExists(cfg.WorktreeDir) {
		_ = os.RemoveAll(cfg.WorktreeDir)
	}
	if cfg.TargetRepo != "" {
		_ = exec.Command("git", "-C", cfg.TargetRepo, "worktree", "prune").Run()
	}
}

func runTask(cfg *Config, task Task, taskIndex int) (TaskSummary, error) {
	outputPath := filepath.Join(cfg.ResultDir, task.ID+".jsonl")
	f, err := os.Create(outputPath)
	if err != nil {
		return TaskSummary{ID: task.ID, Title: task.Title, Status: "failed", TurnsTotal: len(task.Turns)},
			fmt.Errorf("create %s: %w", outputPath, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	ts := TaskSummary{
		ID:         task.ID,
		Title:      task.Title,
		TurnsTotal: len(task.Turns),
		Status:     "ok",
	}

	fmt.Printf("Task %s: %s\n", task.ID, task.Title)

	keenSession := ""
	ocSession := ""
	for i, prompt := range task.Turns {
		turn := i + 1

		runKeen := func() error {
			fmt.Printf("  turn %d keen\n", turn)
			keenRec := runKeenTurn(cfg, task, turn, prompt, keenSession)
			if err := enc.Encode(keenRec); err != nil {
				ts.Status = "failed"
				return fmt.Errorf("write keen record: %w", err)
			}
			if keenRec.SessionID != "" {
				keenSession = keenRec.SessionID
				ts.KeenSession = keenSession
			}
			if keenRec.Status != "ok" {
				ts.Status = "failed"
				return fmt.Errorf("Keen failed on %s turn %d: %s", task.ID, turn, keenRec.Error)
			}
			return nil
		}

		runOpenCode := func() error {
			fmt.Printf("  turn %d opencode\n", turn)
			ocRec := runOpencodeTurn(cfg, task, turn, prompt, ocSession)
			if err := enc.Encode(ocRec); err != nil {
				ts.Status = "failed"
				return fmt.Errorf("write opencode record: %w", err)
			}
			if ocRec.SessionID != "" {
				ocSession = ocRec.SessionID
				ts.OpencodeSession = ocSession
			}
			if ocRec.Status != "ok" {
				ts.Status = "failed"
				return fmt.Errorf("OpenCode failed on %s turn %d: %s", task.ID, turn, ocRec.Error)
			}
			return nil
		}

		var err error
		if taskIndex%2 == 0 {
			err = runKeen()
			if err == nil {
				err = runOpenCode()
			}
		} else {
			err = runOpenCode()
			if err == nil {
				err = runKeen()
			}
		}
		if err != nil {
			return ts, err
		}

		ts.TurnsCompleted = turn
	}
	return ts, nil
}

func runKeenTurn(cfg *Config, task Task, turn int, prompt, sessionID string) Record {
	args := []string{"run", "--format", "json"}
	if cfg.KeenProvider != "" {
		args = append(args, "--provider", cfg.KeenProvider)
	}
	if cfg.KeenModel != "" {
		args = append(args, "--model", cfg.KeenModel)
	}
	if sessionID != "" {
		args = append(args, "--session", sessionID)
	}
	args = append(args, "--", prompt)

	rec, stdout, stderr := runTool(cfg, "keen", args, cfg.KeenWorktree, cfg.KeenCommit, task, turn, prompt)

	var errs []string
	if rec.Error != "" {
		errs = append(errs, rec.Error)
	}
	parsed, parseErr := parseKeenOutput(stdout)
	if parseErr != nil {
		rec.Status = "failed"
		errs = append(errs, "failed to parse Keen JSON output: "+parseErr.Error())
	} else {
		rec.SessionID = parsed.SessionID
		rec.Text = parsed.Text
		if parsed.Usage != nil {
			rec.Usage = Usage{
				Input:     parsed.Usage.InputTokens,
				Output:    parsed.Usage.OutputTokens,
				Reasoning: parsed.Usage.ReasoningTokens,
				CacheRead: parsed.Usage.CachedTokens,
				Total:     parsed.Usage.TotalTokens,
			}
		}
	}

	if rec.ExitCode != 0 {
		rec.Status = "failed"
		errs = append(errs, fmt.Sprintf("keen exited with code %d", rec.ExitCode))
	}
	if rec.SessionID == "" {
		rec.Status = "failed"
		errs = append(errs, "Keen session id missing")
	}

	dirty := gitStatusShort(cfg.KeenWorktree)
	rec.DirtyStatus = dirty
	if dirty != "" {
		rec.Status = "failed"
		errs = append(errs, "Keen worktree became dirty")
	}

	if rec.Status != "ok" {
		rec.RawStdout = stdout
		rec.RawStderr = stderr
	}
	rec.Error = strings.Join(errs, "\n")
	return rec
}

func runOpencodeTurn(cfg *Config, task Task, turn int, prompt, sessionID string) Record {
	args := []string{"run", "--format", "json", "--thinking", "--model", cfg.OpencodeModel}
	if sessionID != "" {
		args = append(args, "--session", sessionID)
	} else {
		args = append(args, "--title", cfg.RunID+"-"+task.ID)
	}
	args = append(args, "--", prompt)

	rec, stdout, stderr := runTool(cfg, "opencode", args, cfg.OcWorktree, cfg.OcCommit, task, turn, prompt)

	var errs []string
	if rec.Error != "" {
		errs = append(errs, rec.Error)
	}
	parsed := parseOpencodeStream(stdout)
	if parsed.ParseErr != nil {
		rec.Status = "failed"
		errs = append(errs, "failed to parse OpenCode NDJSON: "+parsed.ParseErr.Error())
	} else {
		rec.SessionID = parsed.SessionID
		rec.Text = parsed.Text
		rec.Usage = parsed.Usage
	}

	if rec.ExitCode != 0 {
		rec.Status = "failed"
		errs = append(errs, fmt.Sprintf("opencode exited with code %d", rec.ExitCode))
	}
	if rec.SessionID == "" {
		rec.Status = "failed"
		errs = append(errs, "OpenCode session id missing")
	}
	if parsed.PermissionRequested {
		rec.Status = "failed"
		errs = append(errs, "OpenCode requested permission")
	}
	if parsed.SessionError != "" {
		rec.Status = "failed"
		errs = append(errs, "OpenCode session error: "+parsed.SessionError)
	}

	dirty := gitStatusShort(cfg.OcWorktree)
	rec.DirtyStatus = dirty
	if dirty != "" {
		rec.Status = "failed"
		errs = append(errs, "OpenCode worktree became dirty")
	}

	if rec.Status != "ok" {
		rec.RawStdout = stdout
		rec.RawStderr = stderr
	}
	rec.Error = strings.Join(errs, "\n")
	return rec
}

// runTool invokes a CLI in a worktree and returns a partially populated Record
// plus captured stdout and stderr. The caller is responsible for parsing,
// status, and error fields.
func runTool(cfg *Config, tool string, args []string, cwd, commit string, task Task, turn int, prompt string) (Record, string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.TurnTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now().UTC()
	start := time.Now()
	err := cmd.Run()
	durationMs := time.Since(start).Milliseconds()
	finishedAt := time.Now().UTC()
	exitCode := exitCodeFor(cmd, err)
	status := "ok"
	runError := ""
	if ctx.Err() == context.DeadlineExceeded {
		status = "failed"
		runError = fmt.Sprintf("%s timed out after %s", tool, cfg.TurnTimeout)
		if exitCode == 0 {
			exitCode = -1
		}
	}

	rec := Record{
		RunID:     cfg.RunID,
		TaskID:    task.ID,
		TaskTitle: task.Title,
		Tool:      tool,
		Turn:      turn,
		Prompt:    prompt,
		TargetRepo: TargetRepo{
			Path:   cfg.TargetRepo,
			Ref:    "main",
			Commit: commit,
		},
		Command:    append([]string{tool}, args...),
		CWD:        cwd,
		StartedAt:  isoTime(startedAt),
		FinishedAt: isoTime(finishedAt),
		DurationMs: durationMs,
		ExitCode:   exitCode,
		Status:     status,
		Error:      runError,
	}
	return rec, stdout.String(), stderr.String()
}

func parseKeenOutput(stdout string) (*keenOutput, error) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, errors.New("empty stdout")
	}
	var out keenOutput
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func parseOpencodeStream(stdout string) opencodeParsed {
	var result opencodeParsed
	var textParts []string
	var sessionErrors []string

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			result.ParseErr = err
			return result
		}
		evType, _ := ev["type"].(string)

		if result.SessionID == "" {
			if s := getString(ev, "sessionID"); s != "" {
				result.SessionID = s
			} else if part := getMap(ev, "part"); part != nil {
				if s := getString(part, "sessionID"); s != "" {
					result.SessionID = s
				}
			}
		}

		switch evType {
		case "text":
			if part := getMap(ev, "part"); part != nil {
				if t := getString(part, "text"); t != "" {
					textParts = append(textParts, t)
				}
			}
		case "step_finish":
			if part := getMap(ev, "part"); part != nil {
				if tok := getMap(part, "tokens"); tok != nil {
					result.Usage.Input += getInt(tok, "input")
					result.Usage.Output += getInt(tok, "output")
					result.Usage.Reasoning += getInt(tok, "reasoning")
					result.Usage.Total += getInt(tok, "total")
					if cache := getMap(tok, "cache"); cache != nil {
						result.Usage.CacheRead += getInt(cache, "read")
						result.Usage.CacheWrite += getInt(cache, "write")
					}
				}
				result.Usage.Cost += getFloat(part, "cost")
			}
		case "permission.asked":
			result.PermissionRequested = true
		case "session.error":
			msg := describeOpenCodeError(ev["error"])
			if msg == "" {
				if props := getMap(ev, "properties"); props != nil {
					msg = describeOpenCodeError(props["error"])
				}
			}
			if msg != "" {
				sessionErrors = append(sessionErrors, msg)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		result.ParseErr = err
		return result
	}
	result.Text = strings.Join(textParts, "\n")
	result.SessionError = strings.Join(sessionErrors, "\n")
	return result
}

func collectToolVersions() map[string]string {
	versions := map[string]string{}
	for tool, args := range map[string][]string{
		"keen":     {"--version"},
		"opencode": {"--version"},
		"git":      {"--version"},
	} {
		out, err := exec.Command(tool, args...).Output()
		if err == nil {
			versions[tool] = strings.TrimSpace(string(out))
		}
	}
	return versions
}

func buildSessionsText(cfg *Config, summaries []TaskSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Benchmark run: %s\n\n", cfg.RunID)
	b.WriteString("Keen sessions:\n")
	for _, ts := range summaries {
		if ts.KeenSession != "" {
			fmt.Fprintf(&b, "%s  %s\n", ts.ID, ts.KeenSession)
		}
	}
	b.WriteString("\nOpenCode sessions:\n")
	for _, ts := range summaries {
		if ts.OpencodeSession != "" {
			fmt.Fprintf(&b, "%s  %s\n", ts.ID, ts.OpencodeSession)
		}
	}
	fmt.Fprintf(&b, "\nResults:\n%s\n", cfg.ResultDir)
	return b.String()
}

// helpers

func revParse(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitStatusShort(dir string) string {
	out, _ := exec.Command("git", "-C", dir, "status", "--short").Output()
	return strings.TrimRight(string(out), "\n")
}

func exitCodeFor(cmd *exec.Cmd, runErr error) int {
	if runErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitCode()
	}
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	return -1
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func isoTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func writeJSONFile(path string, data any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(data)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func getFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func describeOpenCodeError(v any) string {
	switch errVal := v.(type) {
	case nil:
		return ""
	case string:
		return errVal
	case map[string]any:
		if data := getMap(errVal, "data"); data != nil {
			if msg := getString(data, "message"); msg != "" {
				return msg
			}
		}
		if msg := getString(errVal, "message"); msg != "" {
			return msg
		}
		if name := getString(errVal, "name"); name != "" {
			return name
		}
		data, marshalErr := json.Marshal(errVal)
		if marshalErr == nil {
			return string(data)
		}
	default:
		return fmt.Sprint(errVal)
	}
	return ""
}
