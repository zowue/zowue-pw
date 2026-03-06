package webhook

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zarazaex69/zowue-analyzer/internal/ai"
	"github.com/zarazaex69/zowue-analyzer/internal/github"
	"github.com/zarazaex69/zowue-analyzer/internal/types"
)

type Processor struct {
	workDir       string
	githubClient  *github.Client
	aiAgent       *ai.Agent
	commandRunner *CommandRunner
}

// NewProcessor creates commit processor
func NewProcessor(workDir, githubToken string) *Processor {
	return &Processor{
		workDir:       workDir,
		githubClient:  github.NewClient(githubToken),
		aiAgent:       ai.NewAgent(),
		commandRunner: NewCommandRunner(),
	}
}

// Process handles commit analysis workflow
func (p *Processor) Process(event *PushEvent) {
	log.Printf("\n========================================")
	log.Printf("=== PROCESSING COMMIT ===")
	log.Printf("========================================")
	log.Printf("Repository: %s", event.Repository.FullName)
	log.Printf("Commit: %s", event.HeadCommit.ID)
	log.Printf("Message: %s", event.HeadCommit.Message)
	log.Printf("Author: %s <%s>", event.HeadCommit.Author.Name, event.HeadCommit.Author.Email)
	log.Printf("========================================\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// extract repository name
	repoName := p.extractRepoName(event.Repository.FullName)
	repoDir := filepath.Join(p.workDir, repoName)

	log.Printf("[SETUP] work directory: %s", repoDir)

	// cleanup old directory if exists
	if err := os.RemoveAll(repoDir); err != nil {
		log.Printf("[SETUP] WARNING: failed to cleanup old directory: %v", err)
	}

	// create work directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		log.Printf("[SETUP] ERROR: failed to create work directory: %v", err)
		return
	}

	// clone repository at specific commit
	log.Printf("[GIT] cloning repository...")
	if err := p.cloneRepo(ctx, event.Repository.CloneURL, event.HeadCommit.ID, repoDir); err != nil {
		log.Printf("[GIT] ERROR: failed to clone repository: %v", err)
		return
	}
	log.Printf("[GIT] repository cloned successfully")

	// gather commit information
	log.Printf("[DATA] gathering commit information...")
	info, err := p.gatherCommitInfo(ctx, event, repoDir)
	if err != nil {
		log.Printf("[DATA] ERROR: failed to gather commit info: %v", err)
		return
	}
	log.Printf("[DATA] diff size: %d bytes", len(info.Diff))
	log.Printf("[DATA] file tree size: %d bytes", len(info.FileTree))

	// run ai analysis
	log.Printf("\n[AI] starting analysis...")
	report, err := p.aiAgent.Analyze(ctx, info, repoDir)
	if err != nil {
		log.Printf("[AI] ERROR: failed to analyze commit: %v", err)
		return
	}
	log.Printf("[AI] analysis completed successfully")

	// create github issue with results
	log.Printf("\n[GITHUB] creating issue...")
	if err := p.githubClient.CreateIssue(ctx, event.Repository.FullName, info, report); err != nil {
		log.Printf("[GITHUB] ERROR: failed to create issue: %v", err)
		return
	}

	log.Printf("\n========================================")
	log.Printf("=== COMMIT PROCESSED SUCCESSFULLY ===")
	log.Printf("========================================\n")
}

// cloneRepo clones repository at specific commit
func (p *Processor) cloneRepo(ctx context.Context, cloneURL, commitHash, targetDir string) error {
	// git clone with depth 1
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w, output: %s", err, output)
	}

	// checkout specific commit
	cmd = exec.CommandContext(ctx, "git", "-C", targetDir, "fetch", "--depth", "1", "origin", commitHash)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w, output: %s", err, output)
	}

	cmd = exec.CommandContext(ctx, "git", "-C", targetDir, "checkout", commitHash)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %w, output: %s", err, output)
	}

	return nil
}

// gatherCommitInfo collects all necessary information about commit
func (p *Processor) gatherCommitInfo(ctx context.Context, event *PushEvent, repoDir string) (*types.CommitInfo, error) {
	// get diff
	diff, err := p.getDiff(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// get file tree
	fileTree, err := p.getFileTree(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get file tree: %w", err)
	}

	return &types.CommitInfo{
		RepoFullName: event.Repository.FullName,
		RepoCloneURL: event.Repository.CloneURL,
		RepoHTMLURL:  event.Repository.HTMLURL,
		CommitHash:   event.HeadCommit.ID,
		CommitMsg:    event.HeadCommit.Message,
		CommitTime:   event.HeadCommit.Timestamp,
		AuthorName:   event.HeadCommit.Author.Name,
		AuthorEmail:  event.HeadCommit.Author.Email,
		Diff:         diff,
		FileTree:     fileTree,
	}, nil
}

// getDiff returns git diff for last commit
func (p *Processor) getDiff(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "diff", "HEAD~1", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// if no previous commit, show all files as new
		cmd = exec.CommandContext(ctx, "git", "-C", repoDir, "show", "HEAD")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
	}
	return string(output), nil
}

// getFileTree returns recursive file listing
func (p *Processor) getFileTree(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "ls", "-R", repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// extractRepoName extracts repository name from full name
func (p *Processor) extractRepoName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullName
}
