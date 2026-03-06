package ai

import (
	"context"
	"fmt"
	"log"
	"time"

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

Your task is to thoroughly analyze Go projects for:
1. Test coverage and test execution
2. Build verification
3. Code quality issues (golangci-lint)
4. Security vulnerabilities
5. Best practices violations

WORKFLOW:
1. First, read Makefile or README to understand project structure
2. Discover and run all available tests
3. Check test coverage
4. Run build process
5. Execute linters and security scanners
6. Analyze results and identify real issues vs false positives
7. Provide comprehensive summary

AVAILABLE TOOLS:
- cat: read entire file content
- catl: read file from line X to line N
- wc: count lines in file
- grep: search pattern in files
- run: execute shell command with timeout
- summary: return final analysis report

IMPORTANT:
- Always set appropriate timeout for commands (use seconds)
- Start with lightweight operations (wc, grep) before reading large files
- Use catl for large files to read specific sections
- Verify test commands exist before running
- Distinguish between critical issues and false positives
- Be thorough but efficient`,
	}
}

// Analyze performs complete commit analysis
func (a *Agent) Analyze(ctx context.Context, info *types.CommitInfo, repoDir string) (*AnalysisReport, error) {
	log.Printf("starting ai analysis for commit %s", info.CommitHash[:7])

	// prepare initial context
	initialPrompt := fmt.Sprintf(`Analyze this Go project commit:

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
		info.CommitHash[:7],
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
	messages := []Message{
		{Role: "system", Content: a.systemPrompt},
		{Role: "user", Content: initialPrompt},
	}

	maxIterations := 50
	iteration := 0

	log.Printf("=== starting ai analysis loop ===")
	log.Printf("initial prompt length: %d chars", len(initialPrompt))

	for iteration < maxIterations {
		iteration++
		log.Printf("\n=== ai iteration %d/%d ===", iteration, maxIterations)
		log.Printf("messages in history: %d", len(messages))

		// call ai with tools
		log.Printf("calling ai api...")
		response, err := a.client.Chat(ctx, messages, a.toolset.GetTools())
		if err != nil {
			log.Printf("ERROR: ai chat failed: %v", err)
			return nil, fmt.Errorf("ai chat failed: %w", err)
		}

		log.Printf("ai response received:")
		log.Printf("  content length: %d chars", len(response.Content))
		log.Printf("  tool calls: %d", len(response.ToolCalls))
		if response.Content != "" {
			log.Printf("  content preview: %s", truncate(response.Content, 200))
		}

		// add assistant response to history
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		// check if ai wants to call tools
		if len(response.ToolCalls) == 0 {
			log.Println("ai finished without tool calls")
			break
		}

		// execute tool calls
		log.Printf("executing %d tool calls...", len(response.ToolCalls))
		toolResults := make([]ToolResult, 0, len(response.ToolCalls))
		for i, toolCall := range response.ToolCalls {
			log.Printf("\ntool call %d/%d:", i+1, len(response.ToolCalls))
			log.Printf("  id: %s", toolCall.ID)
			log.Printf("  function: %s", toolCall.Function.Name)
			log.Printf("  arguments: %s", toolCall.Function.Arguments)

			startTime := time.Now()
			result, err := a.toolset.Execute(ctx, toolCall, repoDir)
			duration := time.Since(startTime)

			if err != nil {
				log.Printf("  ERROR: tool execution failed after %v: %v", duration, err)
				result = fmt.Sprintf("ERROR: %v", err)
			} else {
				log.Printf("  SUCCESS: completed in %v", duration)
				log.Printf("  result length: %d chars", len(result))
				log.Printf("  result preview: %s", truncate(result, 200))
			}

			// check if summary was called
			if toolCall.Function.Name == "summary" {
				log.Println("\n=== ai called summary, analysis complete ===")
				log.Printf("summary content:\n%s", result)
				return parseAnalysisReport(result), nil
			}

			toolResults = append(toolResults, ToolResult{
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}

		// add tool results to history
		log.Printf("\nadding %d tool results to history", len(toolResults))
		for _, tr := range toolResults {
			messages = append(messages, Message{
				Role:       "tool",
				Content:    tr.Content,
				ToolCallID: tr.ToolCallID,
			})
		}
	}

	log.Printf("\nERROR: max iterations (%d) reached without summary", maxIterations)
	return nil, fmt.Errorf("max iterations reached without summary")
}

// truncate limits string length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
