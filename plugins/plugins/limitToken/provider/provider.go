package provider

type LLMAdapter interface {
	// 请求：厂商 -> HTNN
	ConvertRequestToHTNN(reqBody []byte) (*HTNNRequest, error)

	// 响应：厂商 -> HTNN（非流式）
	ConvertResponseFromHTNN(respBody []byte) (*HTNNResponse, error)

	// 响应：厂商 -> HTNN（流式）,如果有历史请求记录需要存储
	ConvertStreamChunkFromHTNN(delta *HTNNStreamDelta, chunk []byte) (*HTNNStreamDelta, error)
}

type HTNNRequest struct {
	Model       string `json:"model"`
	PromptToken int    `json:"prompt_token"`
	Stream      bool   `json:"stream,omitempty"`
	MaxTokens   int    `json:"max_tokens,omitempty"`
}

type HTNNResponse struct {
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
}

type HTNNStreamDelta struct {
	Model            string   `json:"model,omitempty"`
	Finish           bool     `json:"finish,omitempty"`
	Deltas           []*Delta `json:"delta,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
}

type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}
