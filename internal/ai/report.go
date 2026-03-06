package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseAnalysisReport converts summary json to structured report
func parseAnalysisReport(summaryJSON string) *AnalysisReport {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(summaryJSON), &data); err != nil {
		// fallback to plain text
		return &AnalysisReport{
			Summary: summaryJSON,
		}
	}

	report := &AnalysisReport{}

	if v, ok := data["summary"].(string); ok {
		report.Summary = v
	}

	if v, ok := data["test_results"].(string); ok {
		report.TestResults = v
	}

	if v, ok := data["coverage_results"].(string); ok {
		report.CoverageResults = v
	}

	if v, ok := data["build_results"].(string); ok {
		report.BuildResults = v
	}

	if v, ok := data["lint_results"].(string); ok {
		report.LintResults = v
	}

	report.SecurityIssues = extractStringArray(data, "security_issues")
	report.CriticalIssues = extractStringArray(data, "critical_issues")
	report.Warnings = extractStringArray(data, "warnings")
	report.FalsePositives = extractStringArray(data, "false_positives")
	report.Recommendations = extractStringArray(data, "recommendations")

	return report
}

// extractStringArray safely extracts string array from map
func extractStringArray(data map[string]interface{}, key string) []string {
	if v, ok := data[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// FormatMarkdown converts report to markdown format
func (r *AnalysisReport) FormatMarkdown() string {
	var sb strings.Builder

	sb.WriteString("## 🤖 Zowue AI Analysis Report\n\n")

	if r.Summary != "" {
		sb.WriteString("### Summary\n\n")
		sb.WriteString(r.Summary)
		sb.WriteString("\n\n")
	}

	if len(r.CriticalIssues) > 0 {
		sb.WriteString("### 🚨 Critical Issues\n\n")
		for _, issue := range r.CriticalIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	if len(r.SecurityIssues) > 0 {
		sb.WriteString("### 🔒 Security Issues\n\n")
		for _, issue := range r.SecurityIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	if len(r.Warnings) > 0 {
		sb.WriteString("### ⚠️ Warnings\n\n")
		for _, warning := range r.Warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		sb.WriteString("\n")
	}

	if r.TestResults != "" {
		sb.WriteString("### 🧪 Test Results\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.TestResults)
		sb.WriteString("\n```\n\n")
	}

	if r.CoverageResults != "" {
		sb.WriteString("### 📊 Coverage Results\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.CoverageResults)
		sb.WriteString("\n```\n\n")
	}

	if r.BuildResults != "" {
		sb.WriteString("### 🔨 Build Results\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.BuildResults)
		sb.WriteString("\n```\n\n")
	}

	if r.LintResults != "" {
		sb.WriteString("### 🔍 Lint Results\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.LintResults)
		sb.WriteString("\n```\n\n")
	}

	if len(r.Recommendations) > 0 {
		sb.WriteString("### 💡 Recommendations\n\n")
		for _, rec := range r.Recommendations {
			sb.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		sb.WriteString("\n")
	}

	if len(r.FalsePositives) > 0 {
		sb.WriteString("### ✅ False Positives Identified\n\n")
		for _, fp := range r.FalsePositives {
			sb.WriteString(fmt.Sprintf("- %s\n", fp))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*Analyzed by Zowue AI*\n")

	return sb.String()
}
