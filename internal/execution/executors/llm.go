// Package executors 提供任务执行器实现
package executors

import (
	"context"
	"fmt"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/execution"
)

// LLMProvider LLM提供者接口
type LLMProvider interface {
	Name() string
	Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
	Complete(ctx context.Context, prompt string, options *CompletionOptions) (*CompletionResponse, error)
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`    // system/user/assistant
	Content string `json:"content"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content     string `json:"content"`
	FinishReason string `json:"finish_reason"`
	TokensUsed  int    `json:"tokens_used"`
}

// CompletionOptions 补全选项
type CompletionOptions struct {
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	Stop        []string `json:"stop"`
}

// CompletionResponse 补全响应
type CompletionResponse struct {
	Text        string `json:"text"`
	FinishReason string `json:"finish_reason"`
	TokensUsed  int    `json:"tokens_used"`
}

// LLMExecutor LLM执行器
type LLMExecutor struct {
	*execution.BaseExecutor
	providers map[string]LLMProvider
}

// NewLLMExecutor 创建LLM执行器
func NewLLMExecutor() *LLMExecutor {
	return &LLMExecutor{
		BaseExecutor: execution.NewBaseExecutor("llm", "1.0.0", []execution.JobType{execution.JobTypeLLM}),
		providers:    make(map[string]LLMProvider),
	}
}

// RegisterProvider 注册LLM提供者
func (e *LLMExecutor) RegisterProvider(provider LLMProvider) {
	e.providers[provider.Name()] = provider
}

// CanExecute 检查是否能执行
func (e *LLMExecutor) CanExecute(job *execution.ExecutionJob) bool {
	if !e.BaseExecutor.CanExecute(job) {
		return false
	}
	
	// 检查是否有可用的提供者
	provider, _ := job.Input["provider"].(string)
	if provider != "" {
		_, exists := e.providers[provider]
		return exists
	}
	
	// 至少有一个提供者可用
	return len(e.providers) > 0
}

// EstimateResources 估算资源
func (e *LLMExecutor) EstimateResources(job *execution.ExecutionJob) (*execution.ResourceEstimate, error) {
	return &execution.ResourceEstimate{
		CPUTimeMs:   100,
		MemoryBytes: 16 * 1024 * 1024, // 16MB
		DurationSec: 60, // LLM可能需要较长时间
	}, nil
}

// Execute 执行LLM任务
func (e *LLMExecutor) Execute(ctx context.Context, job *execution.ExecutionJob) (*execution.ExecutionResult, error) {
	llmType, ok := job.Input["llm_type"].(string)
	if !ok || llmType == "" {
		llmType = "chat" // 默认聊天模式
	}

	providerName, _ := job.Input["provider"].(string)
	if providerName == "" {
		// 使用第一个可用的提供者
		for name := range e.providers {
			providerName = name
			break
		}
	}

	if providerName == "" {
		return execution.NewErrorResult("no LLM provider available"), nil
	}

	provider, exists := e.providers[providerName]
	if !exists {
		return execution.NewErrorResult(fmt.Sprintf("provider not found: %s", providerName)), nil
	}

	var result any
	var err error

	switch llmType {
	case "chat":
		result, err = e.executeChat(ctx, provider, job)
	case "completion":
		result, err = e.executeCompletion(ctx, provider, job)
	case "analysis":
		result, err = e.executeAnalysis(ctx, provider, job)
	case "summary":
		result, err = e.executeSummary(ctx, provider, job)
	default:
		return execution.NewErrorResult(fmt.Sprintf("unsupported llm type: %s", llmType)), nil
	}

	if err != nil {
		return execution.NewErrorResult(err.Error()), nil
	}

	return execution.NewSuccessResult(map[string]any{
		"llm_type": llmType,
		"provider": providerName,
		"result":   result,
	}, nil), nil
}

// executeChat 执行聊天
func (e *LLMExecutor) executeChat(ctx context.Context, provider LLMProvider, job *execution.ExecutionJob) (any, error) {
	messagesRaw, ok := job.Input["messages"].([]any)
	if !ok {
		// 尝试单条消息
		content, ok := job.Input["message"].(string)
		if !ok {
			return nil, fmt.Errorf("missing messages or message parameter")
		}
		messagesRaw = []any{
			map[string]any{"role": "user", "content": content},
		}
	}

	messages := make([]ChatMessage, 0, len(messagesRaw))
	for _, m := range messagesRaw {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		if role != "" && content != "" {
			messages = append(messages, ChatMessage{Role: role, Content: content})
		}
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no valid messages provided")
	}

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"content":       response.Content,
		"finish_reason": response.FinishReason,
		"tokens_used":   response.TokensUsed,
	}, nil
}

// executeCompletion 执行补全
func (e *LLMExecutor) executeCompletion(ctx context.Context, provider LLMProvider, job *execution.ExecutionJob) (any, error) {
	prompt, ok := job.Input["prompt"].(string)
	if !ok || prompt == "" {
		return nil, fmt.Errorf("missing or invalid prompt parameter")
	}

	options := &CompletionOptions{
		MaxTokens:   256,
		Temperature: 0.7,
		TopP:        1.0,
	}

	if maxTokens, ok := job.Input["max_tokens"].(float64); ok {
		options.MaxTokens = int(maxTokens)
	}
	if temp, ok := job.Input["temperature"].(float64); ok {
		options.Temperature = temp
	}
	if topP, ok := job.Input["top_p"].(float64); ok {
		options.TopP = topP
	}

	response, err := provider.Complete(ctx, prompt, options)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"text":          response.Text,
		"finish_reason": response.FinishReason,
		"tokens_used":   response.TokensUsed,
	}, nil
}

// executeAnalysis 执行分析
func (e *LLMExecutor) executeAnalysis(ctx context.Context, provider LLMProvider, job *execution.ExecutionJob) (any, error) {
	content, ok := job.Input["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("missing or invalid content parameter")
	}

	analysisType, _ := job.Input["analysis_type"].(string)
	if analysisType == "" {
		analysisType = "general"
	}

	var systemPrompt string
	switch analysisType {
	case "sentiment":
		systemPrompt = "Analyze the sentiment of the following text. Provide: 1) Overall sentiment (positive/negative/neutral), 2) Confidence score (0-1), 3) Key phrases that indicate sentiment."
	case "entities":
		systemPrompt = "Extract named entities from the following text. Identify: people, organizations, locations, dates, and other important entities. Format as a list."
	case "keywords":
		systemPrompt = "Extract the key terms and phrases from the following text. List the most important concepts and topics."
	case "general":
		systemPrompt = "Analyze the following text and provide a comprehensive summary including: main topics, key points, and any notable observations."
	default:
		systemPrompt = "Analyze the following text: "
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: content},
	}

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"analysis_type": analysisType,
		"result":        response.Content,
		"tokens_used":   response.TokensUsed,
	}, nil
}

// executeSummary 执行摘要
func (e *LLMExecutor) executeSummary(ctx context.Context, provider LLMProvider, job *execution.ExecutionJob) (any, error) {
	content, ok := job.Input["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("missing or invalid content parameter")
	}

	maxLength, _ := job.Input["max_length"].(float64)
	if maxLength == 0 {
		maxLength = 200
	}

	messages := []ChatMessage{
		{Role: "system", Content: fmt.Sprintf("Summarize the following text in approximately %.0f words or less. Be concise but capture the key points.", maxLength)},
		{Role: "user", Content: content},
	}

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"summary":     response.Content,
		"tokens_used": response.TokensUsed,
	}, nil
}

// MockLLMProvider 模拟LLM提供者（用于测试）
type MockLLMProvider struct {
	name string
}

// NewMockLLMProvider 创建模拟LLM提供者
func NewMockLLMProvider(name string) *MockLLMProvider {
	return &MockLLMProvider{name: name}
}

// Name 返回名称
func (m *MockLLMProvider) Name() string {
	return m.name
}

// Chat 模拟聊天
func (m *MockLLMProvider) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	// 返回简单的模拟响应
	lastMsg := ""
	for _, msg := range messages {
		if msg.Role == "user" {
			lastMsg = msg.Content
		}
	}

	return &ChatResponse{
		Content:      fmt.Sprintf("Mock response to: %s", lastMsg),
		FinishReason: "stop",
		TokensUsed:   100,
	}, nil
}

// Complete 模拟补全
func (m *MockLLMProvider) Complete(ctx context.Context, prompt string, options *CompletionOptions) (*CompletionResponse, error) {
	return &CompletionResponse{
		Text:         fmt.Sprintf("Mock completion for: %s", prompt),
		FinishReason: "stop",
		TokensUsed:   50,
	}, nil
}
