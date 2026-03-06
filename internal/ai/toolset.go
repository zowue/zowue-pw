package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Toolset struct {
	tools []Tool
}

// NewToolset creates toolset with all available tools
func NewToolset() *Toolset {
	return &Toolset{
		tools: []Tool{
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "cat",
					Description: "Read entire file content",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "Relative file path from repository root",
							},
						},
						"required": []string{"path"},
					},
				},
			},
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "catl",
					Description: "Read file from line X to line N (inclusive)",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "Relative file path from repository root",
							},
							"start": map[string]interface{}{
								"type":        "integer",
								"description": "Start line number (1-indexed)",
							},
							"end": map[string]interface{}{
								"type":        "integer",
								"description": "End line number (1-indexed)",
							},
						},
						"required": []string{"path", "start", "end"},
					},
				},
			},
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "wc",
					Description: "Count lines in file",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "Relative file path from repository root",
							},
						},
						"required": []string{"path"},
					},
				},
			},
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "grep",
					Description: "Search for pattern in files using grep",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"pattern": map[string]interface{}{
								"type":        "string",
								"description": "Search pattern (regex)",
							},
							"path": map[string]interface{}{
								"type":        "string",
								"description": "File or directory path to search in (use . for current directory)",
							},
							"recursive": map[string]interface{}{
								"type":        "boolean",
								"description": "Search recursively in directories",
							},
						},
						"required": []string{"pattern", "path"},
					},
				},
			},
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "run",
					Description: "Execute shell command with timeout. Returns stdout and stderr.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"command": map[string]interface{}{
								"type":        "string",
								"description": "Shell command to execute",
							},
							"timeout": map[string]interface{}{
								"type":        "integer",
								"description": "Timeout in seconds (default: 300)",
							},
						},
						"required": []string{"command"},
					},
				},
			},
			{
				Type: "function",
				Function: FunctionDef{
					Name:        "summary",
					Description: "Return final analysis report. Call this when analysis is complete.",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"summary": map[string]interface{}{
								"type":        "string",
								"description": "Overall summary of analysis",
							},
							"test_results": map[string]interface{}{
								"type":        "string",
								"description": "Test execution results",
							},
							"coverage_results": map[string]interface{}{
								"type":        "string",
								"description": "Test coverage results",
							},
							"build_results": map[string]interface{}{
								"type":        "string",
								"description": "Build verification results",
							},
							"lint_results": map[string]interface{}{
								"type":        "string",
								"description": "Linter results",
							},
							"security_issues": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "List of security issues found",
							},
							"critical_issues": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "List of critical issues that must be fixed",
							},
							"warnings": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "List of warnings",
							},
							"false_positives": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "List of false positives identified",
							},
							"recommendations": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "List of recommendations for improvement",
							},
						},
						"required": []string{"summary"},
					},
				},
			},
		},
	}
}

// GetTools returns all available tools
func (t *Toolset) GetTools() []Tool {
	return t.tools
}

// Execute runs tool based on tool call
func (t *Toolset) Execute(ctx context.Context, toolCall ToolCall, repoDir string) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	switch toolCall.Function.Name {
	case "cat":
		return t.executeCat(repoDir, args)
	case "catl":
		return t.executeCatL(repoDir, args)
	case "wc":
		return t.executeWC(repoDir, args)
	case "grep":
		return t.executeGrep(ctx, repoDir, args)
	case "run":
		return t.executeRun(ctx, repoDir, args)
	case "summary":
		return t.executeSummary(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}

// executeCat reads entire file
func (t *Toolset) executeCat(repoDir string, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(repoDir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// executeCatL reads file from line X to line N
func (t *Toolset) executeCatL(repoDir string, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	start, ok := args["start"].(float64)
	if !ok {
		return "", fmt.Errorf("start line is required")
	}

	end, ok := args["end"].(float64)
	if !ok {
		return "", fmt.Errorf("end line is required")
	}

	fullPath := filepath.Join(repoDir, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 1

	for scanner.Scan() {
		if lineNum >= int(start) && lineNum <= int(end) {
			lines = append(lines, scanner.Text())
		}
		if lineNum > int(end) {
			break
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return strings.Join(lines, "\n"), nil
}

// executeWC counts lines in file
func (t *Toolset) executeWC(repoDir string, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Join(repoDir, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return fmt.Sprintf("%d lines", lines), nil
}

// executeGrep searches for pattern
func (t *Toolset) executeGrep(ctx context.Context, repoDir string, args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern is required")
	}

	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	recursive := false
	if r, ok := args["recursive"].(bool); ok {
		recursive = r
	}

	fullPath := filepath.Join(repoDir, path)

	cmdArgs := []string{"-n"}
	if recursive {
		cmdArgs = append(cmdArgs, "-r")
	}
	cmdArgs = append(cmdArgs, pattern, fullPath)

	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	output, err := cmd.CombinedOutput()

	// grep returns exit code 1 if no matches found
	if err != nil && len(output) == 0 {
		return "no matches found", nil
	}

	return string(output), nil
}

// executeRun executes shell command
func (t *Toolset) executeRun(ctx context.Context, repoDir string, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	timeout := 300 * time.Second
	if t, ok := args["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)
	cmd.Dir = repoDir

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return result + fmt.Sprintf("\n[TIMEOUT after %v]", timeout), nil
		}
		return result + fmt.Sprintf("\n[EXIT CODE: %v]", err), nil
	}

	return result, nil
}

// executeSummary formats final report
func (t *Toolset) executeSummary(args map[string]interface{}) (string, error) {
	// convert args to json for structured output
	jsonData, err := json.MarshalIndent(args, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal summary: %w", err)
	}

	return string(jsonData), nil
}
