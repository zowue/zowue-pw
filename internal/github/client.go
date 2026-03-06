package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v58/github"
	"github.com/zarazaex69/zowue-analyzer/internal/ai"
	"github.com/zarazaex69/zowue-analyzer/internal/types"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
}

// NewClient creates github api client
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return &Client{
		client: github.NewClient(tc),
	}
}

// CreateIssue creates github issue with analysis report
func (c *Client) CreateIssue(ctx context.Context, repoFullName string, info *types.CommitInfo, report *ai.AnalysisReport) error {
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository name: %s", repoFullName)
	}

	owner := parts[0]
	repo := parts[1]

	commitShort := info.CommitHash
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}

	// format issue title
	title := fmt.Sprintf("%s (%s)", report.Title, commitShort)
	if report.Title == "" {
		title = fmt.Sprintf("Analysis: %s (%s)", truncateCommitMsg(info.CommitMsg), commitShort)
	}

	// format issue body
	body := c.formatIssueBody(info, report)

	// create labels
	labels := c.determineLabels(report)

	issueReq := &github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	}

	issue, _, err := c.client.Issues.Create(ctx, owner, repo, issueReq)
	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	fmt.Printf("created issue #%d: %s\n", *issue.Number, *issue.HTMLURL)
	return nil
}

// formatIssueBody creates formatted issue body
func (c *Client) formatIssueBody(info *types.CommitInfo, report *ai.AnalysisReport) string {
	var sb strings.Builder

	commitShort := info.CommitHash
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}

	sb.WriteString("## Commit Information\n\n")
	sb.WriteString(fmt.Sprintf("- **Commit**: [`%s`](%s/commit/%s)\n", commitShort, info.RepoHTMLURL, info.CommitHash))
	sb.WriteString(fmt.Sprintf("- **Message**: %s\n", info.CommitMsg))
	sb.WriteString(fmt.Sprintf("- **Author**: %s <%s>\n", info.AuthorName, info.AuthorEmail))
	sb.WriteString(fmt.Sprintf("- **Time**: %s\n\n", info.CommitTime.Format("2006-01-02 15:04:05")))

	sb.WriteString("---\n\n")
	sb.WriteString(report.FormatMarkdown())

	return sb.String()
}

// determineLabels assigns labels based on report content
func (c *Client) determineLabels(report *ai.AnalysisReport) []string {
	labels := []string{"zowue", "automated-analysis"}

	if len(report.CriticalIssues) > 0 {
		labels = append(labels, "critical", "bug")
	}

	if len(report.SecurityIssues) > 0 {
		labels = append(labels, "security")
	}

	if len(report.Warnings) > 0 {
		labels = append(labels, "warning")
	}

	if len(report.CriticalIssues) == 0 && len(report.SecurityIssues) == 0 {
		labels = append(labels, "passed")
	}

	return labels
}

// truncateCommitMsg truncates commit message for title
func truncateCommitMsg(msg string) string {
	// remove (w) suffix
	msg = strings.TrimSpace(msg)
	msg = strings.TrimSuffix(msg, "(w)")
	msg = strings.TrimSpace(msg)

	// take first line only
	lines := strings.Split(msg, "\n")
	msg = lines[0]

	// truncate if too long
	if len(msg) > 60 {
		msg = msg[:57] + "..."
	}

	return msg
}
