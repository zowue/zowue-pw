package ai

// Message represents chat message
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents ai tool definition
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines tool function
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents ai tool invocation
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains function call details
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResult contains tool execution result
type ToolResult struct {
	ToolCallID string
	Content    string
}

// ChatResponse represents ai response
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
}

// AnalysisReport contains final analysis results
type AnalysisReport struct {
	Summary         string
	TestResults     string
	CoverageResults string
	BuildResults    string
	LintResults     string
	SecurityIssues  []string
	CriticalIssues  []string
	Warnings        []string
	FalsePositives  []string
	Recommendations []string
}
