package yandexgpt

type completionRequest struct {
	ModelURI          string            `json:"modelUri"`
	CompletionOptions completionOptions `json:"completionOptions"`
	Messages          []message         `json:"messages"`
	JSONSchema        *jsonSchema       `json:"jsonSchema,omitempty"`
}

type completionOptions struct {
	Stream           bool              `json:"stream"`
	Temperature      float64           `json:"temperature"`
	MaxTokens        string            `json:"maxTokens"`
	ReasoningOptions *reasoningOptions `json:"reasoningOptions,omitempty"`
}

type reasoningOptions struct {
	Mode string `json:"mode"`
}

type message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type jsonSchema struct {
	Schema map[string]any `json:"schema"`
}

type completionResponse struct {
	Alternatives []alternative `json:"alternatives"`
	Usage        usage         `json:"usage"`
	ModelVersion string        `json:"modelVersion"`
}

type alternative struct {
	Message message `json:"message"`
	Status  string  `json:"status"`
}

type usage struct {
	InputTextTokens  string `json:"inputTextTokens"`
	CompletionTokens string `json:"completionTokens"`
	TotalTokens      string `json:"totalTokens"`
}
