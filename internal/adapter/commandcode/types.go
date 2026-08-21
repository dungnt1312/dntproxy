package commandcode

type chatRequest struct {
	Model               string        `json:"model"`
	Messages            []chatMessage `json:"messages"`
	Temperature         *float64      `json:"temperature,omitempty"`
	MaxTokens           *int          `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int          `json:"max_completion_tokens,omitempty"`
	Stream              bool          `json:"stream,omitempty"`
	Tools               []any         `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ccToolOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ccContentPart struct {
	Type       string        `json:"type"`
	Text       *string       `json:"text,omitempty"`
	ToolCallID *string       `json:"toolCallId,omitempty"`
	ToolName   *string       `json:"toolName,omitempty"`
	Input      any           `json:"input,omitempty"`
	Output     *ccToolOutput `json:"output,omitempty"`
}

type ccMessage struct {
	Role    string          `json:"role"`
	Content []ccContentPart `json:"content"`
}

type ccChatParams struct {
	Model       string      `json:"model"`
	Messages    []ccMessage `json:"messages"`
	Tools       []any       `json:"tools"`
	System      string      `json:"system"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature"`
	Stream      bool        `json:"stream"`
}

type ccConfig struct {
	WorkingDir    string   `json:"workingDir"`
	Date          string   `json:"date"`
	Environment   string   `json:"environment"`
	Structure     []string `json:"structure"`
	IsGitRepo     bool     `json:"isGitRepo"`
	CurrentBranch string   `json:"currentBranch"`
	MainBranch    string   `json:"mainBranch"`
	GitStatus     string   `json:"gitStatus"`
	RecentCommits []string `json:"recentCommits"`
}

type ccRequestBody struct {
	Config   ccConfig     `json:"config"`
	Memory   string       `json:"memory"`
	Taste    string       `json:"taste"`
	Skills   string       `json:"skills"`
	Params   ccChatParams `json:"params"`
	ThreadID string       `json:"threadId"`
}

type ccStreamEvent struct {
	Type         string         `json:"type"`
	Text         string         `json:"text"`
	ID           string         `json:"id"`
	Delta        string         `json:"delta"`
	Input        map[string]any `json:"input"`
	ToolCallID   string         `json:"toolCallId"`
	ToolName     string         `json:"toolName"`
	FinishReason string         `json:"finishReason"`
	Error        *ccStreamError `json:"error"`
	TotalUsage   *ccUsage       `json:"totalUsage"`
	Usage        *ccUsage       `json:"usage"`
}

type ccStreamError struct {
	Message    string `json:"message"`
	StatusCode *int   `json:"statusCode"`
}

type ccUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}
