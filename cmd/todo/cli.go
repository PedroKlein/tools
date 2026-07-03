package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PedroKlein/tools/pkg/gitrepo"
)

// CLI handles non-interactive command execution.
type CLI struct {
	store  *Store
	repoID string
	stdout io.Writer
	stderr io.Writer
}

// NewCLI creates a CLI with the given store and repo context.
func NewCLI(store *Store, repoID string) *CLI {
	return &CLI{
		store:  store,
		repoID: repoID,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// SetOutput overrides stdout/stderr (for testing).
func (c *CLI) SetOutput(stdout, stderr io.Writer) {
	c.stdout = stdout
	c.stderr = stderr
}

// ExitCode constants.
const (
	ExitOK       = 0
	ExitError    = 1
	ExitNotFound = 2
)

// RunArgs holds parsed CLI arguments.
type RunArgs struct {
	Command string
	Args    []string
	JSON    bool
	Quick   bool
	Global  bool
	Work    bool
	Repo    string
}

// ParseRunArgs parses command-line arguments.
func ParseRunArgs(args []string) RunArgs {
	if len(args) == 0 {
		return RunArgs{Command: "tui"}
	}

	result := RunArgs{}

	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			result.JSON = true
		case "--quick", "-q":
			result.Quick = true
		case "-g", "--global":
			result.Global = true
		case "-w", "--work":
			result.Work = true
		case "--repo", "-r":
			if i+1 < len(args) {
				i++
				result.Repo = args[i]
			}
		case "-h", "--help":
			result.Command = "help"
			return result
		default:
			remaining = append(remaining, arg)
		}
	}

	if len(remaining) == 0 {
		result.Command = "tui"
	} else {
		result.Command = remaining[0]
		result.Args = remaining[1:]
	}

	return result
}

// Run executes a CLI command and returns exit code.
func (c *CLI) Run(cmd string, args []string, jsonOut bool) int {
	switch cmd {
	case "add":
		return c.cmdAdd(args, jsonOut)
	case "list", "ls":
		return c.cmdList(args, jsonOut)
	case "done":
		return c.cmdDone(args, jsonOut)
	case "update":
		return c.cmdUpdate(args, jsonOut)
	case "delete", "rm":
		return c.cmdDelete(args, jsonOut)
	case "help", "-h", "--help":
		c.printUsage()
		return ExitOK
	default:
		_, _ = fmt.Fprintf(c.stderr, "unknown command: %s\nRun 'todo help' for usage.\n", cmd)
		return ExitError
	}
}

func (c *CLI) cmdAdd(args []string, jsonOut bool) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	taskType := fs.String("t", "", "task type (feature/bug/chore/research/review/personal)")
	priority := fs.String("p", "", "priority (low/medium/high)")
	dueDate := fs.String("due", "", "due date (YYYY-MM-DD)")
	desc := fs.String("d", "", "description")
	global := fs.Bool("g", false, "add to personal (global) scope")
	work := fs.Bool("w", false, "add to work scope")

	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		_, _ = fmt.Fprintln(c.stderr, "Usage: todo add [flags] <title>")
		return ExitError
	}

	title := strings.Join(remaining, " ")

	if code, handled := c.handlePRAdd(title, jsonOut); handled {
		return code
	}

	tt, ok := c.resolveTaskType(*taskType)
	if !ok {
		return ExitError
	}

	pr, ok := c.resolveTaskPriority(*priority)
	if !ok {
		return ExitError
	}

	// Validate due date
	if *dueDate != "" && !dateRegex.MatchString(*dueDate) {
		_, _ = fmt.Fprintf(c.stderr, "invalid date format (use YYYY-MM-DD): %s\n", *dueDate)
		return ExitError
	}

	effectiveRepoID := c.effectiveScope(*global, *work)
	scope := GetTaskRepoID(tt, effectiveRepoID)

	// Get next ID
	nextID, err := c.store.NextID(scope)
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "reading store: %v\n", err)
		return ExitError
	}

	task := NewTask(nextID, title, effectiveRepoID, tt)

	task.Priority = pr
	if *dueDate != "" {
		task.DueDate = *dueDate
	}

	if *desc != "" {
		task.Description = *desc
	}

	// Load scope, append, save
	tasks, err := c.store.LoadScope(scope)
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "loading tasks: %v\n", err)
		return ExitError
	}

	tasks = append(tasks, task)
	if err := c.store.SaveScope(scope, tasks); err != nil {
		_, _ = fmt.Fprintf(c.stderr, "saving task: %v\n", err)
		return ExitError
	}

	if jsonOut {
		c.printJSON(task)
	} else {
		_, _ = fmt.Fprintf(c.stdout, "✓ Task #%d created: \"%s\" [%s/%s]\n", task.ID, task.Title, task.Type, task.Priority)
	}

	return ExitOK
}

func (c *CLI) cmdList(args []string, jsonOut bool) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	all := fs.Bool("a", false, "show all repos")
	status := fs.String("s", "active", "status filter (active/open/blocked/done/all)")
	taskType := fs.String("t", "", "filter by type")
	repo := fs.String("r", "", "filter by repo")

	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	var (
		tasks []Task
		err   error
	)

	if *all {
		tasks, err = c.store.LoadAll()
	} else if *repo != "" {
		allTasks, e := c.store.LoadAll()
		if e != nil {
			_, _ = fmt.Fprintf(c.stderr, "loading tasks: %v\n", e)
			return ExitError
		}

		tasks = GetTasksForRepo(allTasks, *repo)
		err = nil
	} else {
		allTasks, e := c.store.LoadAll()
		if e != nil {
			_, _ = fmt.Fprintf(c.stderr, "loading tasks: %v\n", e)
			return ExitError
		}

		tasks = GetTasksForRepo(allTasks, c.repoID)
		err = nil
	}

	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "loading tasks: %v\n", err)
		return ExitError
	}

	// Apply filters
	tasks = FilterByStatus(tasks, StatusFilter(*status))
	if *taskType != "" {
		tasks = FilterByType(tasks, TaskType(*taskType))
	}

	SortByUrgency(tasks)

	if jsonOut {
		c.printJSON(tasks)
	} else {
		c.printTaskTable(tasks)
	}

	return ExitOK
}

func (c *CLI) cmdDone(args []string, jsonOut bool) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(c.stderr, "Usage: todo done <id>")
		return ExitError
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "invalid ID: %s\n", args[0])
		return ExitError
	}

	task, err := c.findAndUpdate(id, func(t *Task) {
		t.Status = StatusDone
		t.UpdatedAt = time.Now().UnixMilli()
	})
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "%v\n", err)
		return ExitNotFound
	}

	if jsonOut {
		c.printJSON(task)
	} else {
		_, _ = fmt.Fprintf(c.stdout, "✓ Task #%d completed: \"%s\"\n", task.ID, task.Title)
	}

	return ExitOK
}

func (c *CLI) cmdUpdate(args []string, jsonOut bool) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	title := fs.String("title", "", "new title")
	taskType := fs.String("t", "", "new type")
	priority := fs.String("p", "", "new priority")
	status := fs.String("s", "", "new status")
	dueDate := fs.String("due", "", "new due date")
	desc := fs.String("d", "", "new description")

	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		_, _ = fmt.Fprintln(c.stderr, "Usage: todo update <id> [flags]")
		return ExitError
	}

	id, err := strconv.Atoi(remaining[0])
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "invalid ID: %s\n", remaining[0])
		return ExitError
	}

	task, err := c.findAndUpdate(id, func(t *Task) {
		if *title != "" {
			t.Title = *title
		}

		if *taskType != "" {
			if IsValidType(TaskType(*taskType)) {
				t.Type = TaskType(*taskType)
			}
		}

		if *priority != "" {
			if IsValidPriority(TaskPriority(*priority)) {
				t.Priority = TaskPriority(*priority)
			}
		}

		if *status != "" {
			if IsValidStatus(TaskStatus(*status)) {
				t.Status = TaskStatus(*status)
			}
		}

		if *dueDate != "" {
			t.DueDate = *dueDate
		}

		if *desc != "" {
			t.Description = *desc
		}

		t.UpdatedAt = time.Now().UnixMilli()
	})
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "%v\n", err)
		return ExitNotFound
	}

	if jsonOut {
		c.printJSON(task)
	} else {
		_, _ = fmt.Fprintf(c.stdout, "✓ Task #%d updated: \"%s\"\n", task.ID, task.Title)
	}

	return ExitOK
}

func (c *CLI) cmdDelete(args []string, jsonOut bool) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(c.stderr, "Usage: todo delete <id>")
		return ExitError
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "invalid ID: %s\n", args[0])
		return ExitError
	}

	// Search all scopes for the task
	scopes, err := c.store.ListScopes()
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "listing scopes: %v\n", err)
		return ExitError
	}

	for _, scope := range scopes {
		tasks, err := c.store.LoadScope(scope)
		if err != nil {
			continue
		}

		for i, t := range tasks {
			if t.ID != id {
				continue
			}

			deleted := tasks[i]

			tasks = append(tasks[:i], tasks[i+1:]...)
			if err := c.store.SaveScope(scope, tasks); err != nil {
				_, _ = fmt.Fprintf(c.stderr, "saving: %v\n", err)
				return ExitError
			}

			if jsonOut {
				c.printJSON(deleted)
			} else {
				_, _ = fmt.Fprintf(c.stdout, "✓ Task #%d deleted: \"%s\"\n", deleted.ID, deleted.Title)
			}

			return ExitOK
		}
	}

	_, _ = fmt.Fprintf(c.stderr, "task #%d not found\n", id)

	return ExitNotFound
}

// findAndUpdate locates a task by ID across all scopes and applies the update function.
func (c *CLI) findAndUpdate(id int, update func(*Task)) (*Task, error) {
	scopes, err := c.store.ListScopes()
	if err != nil {
		return nil, fmt.Errorf("listing scopes: %w", err)
	}

	for _, scope := range scopes {
		tasks, err := c.store.LoadScope(scope)
		if err != nil {
			continue
		}

		for i := range tasks {
			if tasks[i].ID == id {
				update(&tasks[i])

				if err := c.store.SaveScope(scope, tasks); err != nil {
					return nil, fmt.Errorf("saving: %w", err)
				}

				return &tasks[i], nil
			}
		}
	}

	return nil, fmt.Errorf("task #%d not found", id)
}

// handlePRAdd detects a PR URL in title and creates a review task if matched.
// Returns (exitCode, true) when handled, (0, false) when not a PR URL.
func (c *CLI) handlePRAdd(title string, jsonOut bool) (int, bool) {
	pr := ParsePrURL(title)
	if pr == nil {
		return 0, false
	}

	task, err := CreateReviewTask(pr, c.store)
	if err != nil {
		_, _ = fmt.Fprintf(c.stderr, "creating review task: %v\n", err)
		return ExitError, true
	}

	if jsonOut {
		c.printJSON(task)
	} else {
		_, _ = fmt.Fprintf(c.stdout, "✓ Review #%d added: %s/%s #%d\n", task.ID, pr.Owner, pr.Repo, pr.Number)
	}

	return ExitOK, true
}

// resolveTaskType validates the -t flag and returns the task type.
// Returns (type, true) on success; writes to stderr and returns ("", false) on invalid input.
func (c *CLI) resolveTaskType(s string) (TaskType, bool) {
	if s == "" {
		return TypeChore, true
	}

	if !IsValidType(TaskType(s)) {
		_, _ = fmt.Fprintf(c.stderr, "invalid type: %s\n", s)
		return "", false
	}

	return TaskType(s), true
}

// resolveTaskPriority validates the -p flag and returns the priority.
// Returns (priority, true) on success; writes to stderr and returns ("", false) on invalid input.
func (c *CLI) resolveTaskPriority(s string) (TaskPriority, bool) {
	if s == "" {
		return PriorityMedium, true
	}

	if !IsValidPriority(TaskPriority(s)) {
		_, _ = fmt.Fprintf(c.stderr, "invalid priority: %s\n", s)
		return "", false
	}

	return TaskPriority(s), true
}

// effectiveScope returns the repo ID to use based on the global/work flags.
func (c *CLI) effectiveScope(global, work bool) string {
	if global {
		return GlobalRepoID
	}

	if work {
		return WorkRepoID
	}

	return c.repoID
}

func (c *CLI) printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	_, _ = fmt.Fprintln(c.stdout, string(data))
}

func (c *CLI) printTaskTable(tasks []Task) {
	if len(tasks) == 0 {
		_, _ = fmt.Fprintln(c.stdout, "No tasks found.")
		return
	}

	for _, t := range tasks {
		status := statusIcon(t.Status)

		line := fmt.Sprintf("%s #%-3d [%s/%s] %s", status, t.ID, t.Type, t.Priority, t.Title)
		if t.DueDate != "" {
			line += fmt.Sprintf(" (due: %s)", t.DueDate)
		}

		_, _ = fmt.Fprintln(c.stdout, line)

		if t.Description != "" {
			_, _ = fmt.Fprintf(c.stdout, "       %s\n", t.Description)
		}
	}
}

func statusIcon(s TaskStatus) string {
	switch s {
	case StatusOpen:
		return "○"
	case StatusBlocked:
		return "◌"
	case StatusDone:
		return "●"
	default:
		return "?"
	}
}

// DetectRepoID determines the repo slug from CWD or returns GlobalRepoID.
func DetectRepoID() string {
	slug, err := gitrepo.DetectFromCWD()
	if err != nil {
		return GlobalRepoID
	}

	return slug
}

func (c *CLI) printUsage() {
	_, _ = fmt.Fprintf(c.stdout, `todo — terminal task manager

Usage:
  todo                              Launch TUI board
  todo add [flags] <title>          Add a task
  todo list [flags]                 List tasks
  todo done <id>                    Mark task complete
  todo update <id> [flags]          Update task fields
  todo delete <id>                  Delete a task

Add flags:
  -t <type>         Type: feature, bug, chore, research, review, personal
  -p <priority>     Priority: low, medium, high
  -due <date>       Due date: YYYY-MM-DD
  -d <description>  Task description

List flags:
  -a                Show all repos
  -s <status>       Filter: active (default), open, blocked, done, all
  -t <type>         Filter by type
  -r <repo>         Filter by repo slug

Update flags:
  -title <text>     New title
  -t <type>         New type
  -p <priority>     New priority
  -s <status>       New status (open, blocked, done)
  -due <date>       New due date
  -d <description>  New description

Global flags:
  --json            Output as JSON (for scripts/agents)
  --quick, -q       Skip TUI review on add

Repo detection:
  Automatically detects repo from git remote in CWD.
  Falls back to "global" if not in a git repo.
  Override with --repo or -r flag.
`)
}
