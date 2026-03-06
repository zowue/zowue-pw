package types

import "time"

// CommitInfo contains extracted commit information for analysis
type CommitInfo struct {
	RepoFullName string
	RepoCloneURL string
	RepoHTMLURL  string
	CommitHash   string
	CommitMsg    string
	CommitTime   time.Time
	AuthorName   string
	AuthorEmail  string
	Diff         string
	FileTree     string
}
