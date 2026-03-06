package ai

import (
	"context"
	"fmt"
	"log"

	"github.com/zarazaex69/zowue-analyzer/internal/types"
)

type Agent struct {
	client       *Client
	toolset      *Toolset
	systemPrompt string
}

// NewAgent creates ai agent for code analysis
func NewAgent() *Agent {
	return &Agent{
		client:  NewClient(),
		toolset: NewToolset(),
		systemPrompt: `You are an expert Go code analyzer and security auditor.

Analyze Go projects for:
1. Critical bugs and errors
2. Security vulnerabilities
3. Test coverage issues
4. Build problems
5. Linter warnings

IMPORTANT:
- Focus ONLY on actionable issues
- Do NOT describe project structure
- Do NOT explain what files do
- Report ONLY problems, warnings, and recommendations
- Be concise and specific
- IGNORE: tokens in .env files (normal practice), unchecked Close() on defer (acceptable), unchecked Setenv (low risk)

WORKFLOW:
1. Run tests and check coverage
2. Verify build succeeds
3. Run linters (golangci-lint, go vet)
4. Identify real issues vs false positives
5. Call summary with ONLY issues and recommendations

AVAILABLE TOOLS:
- cat: read file content
- catl: read file lines X to N
- wc: count lines
- grep: search pattern
- run: execute command with timeout
- summary: return final report (JSON format)

OUTPUT FORMAT (summary tool):
{
  "title": "Brief summary (max 50 chars, e.g. '3 critical issues, 2 security warnings')",
  "critical_issues": ["issue1", "issue2"],
  "security_issues": ["issue1", "issue2"],
  "warnings": ["warning1", "warning2"],
  "recommendations": ["rec1", "rec2"],
  "build_results": "build output",
  "test_results": "test output",
  "lint_results": "lint output",
  "false_positives": ["fp1", "fp2"]
}`,
	}
}

// Initialize performs initial setup and authentication
func (a *Agent) Initialize(ctx context.Context) error {
	return a.client.Initialize(ctx)
}

func (a *Agent) Analyze(ctx context.Context, info *types.CommitInfo, repoDir string) (*AnalysisReport, error) {
	commitShort := info.CommitHash
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}

	// prepare initial context
	initialPrompt := fmt.Sprintf(`Analyze this Go commit:

REPOSITORY: %s
COMMIT: %s
MESSAGE: %s
AUTHOR: %s <%s>
TIME: %s

FILE TREE:
%s

DIFF:
%s

Your task:
1. Understand project structure (check Makefile, README, go.mod)
2. Run all tests and check coverage
3. Verify build succeeds
4. Run linters (golangci-lint, go vet, staticcheck if available)
5. Check for security issues
6. Analyze results and filter false positives
7. Call summary tool with comprehensive report

Start by exploring the project structure.`,
		info.RepoFullName,
		commitShort,
		info.CommitMsg,
		info.AuthorName,
		info.AuthorEmail,
		info.CommitTime.Format("2006-01-02 15:04:05"),
		truncate(info.FileTree, 5000),
		truncate(info.Diff, 10000),
	)

	// run analysis loop with tool calls
	report, err := a.runAnalysisLoop(ctx, initialPrompt, repoDir)
	if err != nil {
		return nil, fmt.Errorf("analysis loop failed: %w", err)
	}

	return report, nil
}

// runAnalysisLoop executes ai agent loop with tool calls
func (a *Agent) runAnalysisLoop(ctx context.Context, initialPrompt, repoDir string) (*AnalysisReport, error) {
	systemContent := a.systemPrompt
	userContent := initialPrompt

	messages := []Message{
		{Role: "system", Content: &systemContent},
		{Role: "user", Content: &userContent},
	}

	maxIterations := 50

	for iteration := 1; iteration <= maxIterations; iteration++ {
		// call ai with tools
		response, err := a.client.Chat(ctx, messages, a.toolset.GetTools())
		if err != nil {
			return nil, fmt.Errorf("ai chat failed: %w", err)
		}

		// add assistant response to history
		var assistantContent *string
		if response.Content != "" {
			assistantContent = &response.Content
		}

		messages = append(messages, Message{
			Role:      "assistant",
			Content:   assistantContent,
			ToolCalls: response.ToolCalls,
		})

		// check if ai wants to call tools
		if len(response.ToolCalls) == 0 {
			break
		}

		log.Printf("%d/%d %d tools", iteration, maxIterations, len(response.ToolCalls))

		// execute tool calls
		toolResults := make([]ToolResult, 0, len(response.ToolCalls))
		for _, toolCall := range response.ToolCalls {
			result, err := a.toolset.Execute(ctx, toolCall, repoDir)
			if err != nil {
				result = fmt.Sprintf("ERROR: %v", err)
			}

			// check if summary was called
			if toolCall.Function.Name == "summary" {
				return parseAnalysisReport(result), nil
			}

			toolResults = append(toolResults, ToolResult{
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}

		// add tool results to history
		for _, tr := range toolResults {
			toolContent := tr.Content
			toolCallID := tr.ToolCallID

			messages = append(messages, Message{
				Role:       "tool",
				Content:    &toolContent,
				ToolCallID: &toolCallID,
			})
		}
	}

	return nil, fmt.Errorf("max iterations reached")
}

// truncate limits string length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
