package aihelper

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"simple_ai/config"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPEnhancer 在底座模型回答前，按需通过 MCP 工具增强用户消息。
type MCPEnhancer struct {
	llm        model.ToolCallingChatModel
	mcpClient  *client.Client
	mcpBaseURL string
	mutex      sync.Mutex
}

// AIToolCall 描述 MCP 路由模型返回的工具调用决策。
type AIToolCall struct {
	IsToolCall bool                   `json:"isToolCall"`
	ToolName   string                 `json:"toolName"`
	Args       map[string]interface{} `json:"args"`
}

// NewMCPEnhancer 根据 config.toml 中的配置创建 MCP 增强器。
func NewMCPEnhancer(ctx context.Context) (*MCPEnhancer, error) {
	cfg := config.GetConfig().MCPConfig
	if !cfg.Enabled {
		return nil, fmt.Errorf("mcp is disabled in config")
	}
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("mcp serverURL is required")
	}
	if cfg.APIKey == "" || cfg.Model == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("mcp apiKey, model and baseURL are required")
	}

	chatConfig := &openai.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		APIKey:  cfg.APIKey,
	}
	if cfg.TimeoutSeconds > 0 {
		chatConfig.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	if cfg.MaxTokens > 0 {
		chatConfig.MaxTokens = &cfg.MaxTokens
	}

	llm, err := openai.NewChatModel(ctx, chatConfig)
	if err != nil {
		return nil, fmt.Errorf("create mcp routing model failed: %w", err)
	}

	return &MCPEnhancer{
		llm:        llm,
		mcpBaseURL: cfg.ServerURL,
	}, nil
}

// EnhanceMessages 在需要时调用 MCP 工具，并将工具结果注入最后一条消息。
func (m *MCPEnhancer) EnhanceMessages(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
	if m == nil || m.llm == nil {
		return nil, fmt.Errorf("mcp enhancer is not initialized")
	}
	if len(messages) == 0 {
		return messages, nil
	}

	query := messages[len(messages)-1].Content
	firstMessages := cloneMessagesWithLast(messages, m.buildFirstPrompt(query))
	firstResp, err := m.llm.Generate(ctx, firstMessages)
	if err != nil {
		return nil, fmt.Errorf("mcp routing generate failed: %w", err)
	}
	if firstResp == nil {
		return messages, nil
	}

	toolCall, err := m.parseAIResponse(firstResp.Content)
	if err != nil {
		log.Printf("failed to parse mcp routing response: %v", err)
		return messages, nil
	}
	if !toolCall.IsToolCall {
		return messages, nil
	}

	mcpClient, err := m.getMCPClient(ctx)
	if err != nil {
		return nil, err
	}

	toolResult, err := m.callMCPTool(ctx, mcpClient, toolCall.ToolName, toolCall.Args)
	if err != nil {
		return nil, err
	}

	return cloneMessagesWithLast(messages, m.buildSecondPrompt(query, toolCall.ToolName, toolCall.Args, toolResult)), nil
}

func (m *MCPEnhancer) getMCPClient(ctx context.Context) (*client.Client, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.mcpClient != nil {
		return m.mcpClient, nil
	}

	httpTransport, err := transport.NewStreamableHTTP(m.mcpBaseURL)
	if err != nil {
		return nil, fmt.Errorf("create mcp transport failed: %w", err)
	}

	m.mcpClient = client.NewClient(httpTransport)
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "simple_ai",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	if _, err := m.mcpClient.Initialize(ctx, initRequest); err != nil {
		m.mcpClient = nil
		return nil, fmt.Errorf("mcp client initialize failed: %w", err)
	}

	return m.mcpClient, nil
}

func (m *MCPEnhancer) buildFirstPrompt(query string) string {
	return fmt.Sprintf(`你是一个智能助手，可以判断用户问题是否需要调用 MCP 工具。
如果需要调用工具，严格返回 JSON：
{
  "isToolCall": true,
  "toolName": "工具名称",
  "args": {"参数名": "参数值"}
}
如果不需要调用工具，返回：
{
  "isToolCall": false
}

用户问题：%s`, query)
}

func (m *MCPEnhancer) buildSecondPrompt(query, toolName string, args map[string]interface{}, toolResult string) string {
	return fmt.Sprintf(`请结合 MCP 工具结果回答用户问题。

工具名称：%s
工具参数：%v
工具结果：%s

用户问题：%s`, toolName, args, toolResult, query)
}

func (m *MCPEnhancer) parseAIResponse(response string) (*AIToolCall, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var toolCall AIToolCall
	if err := json.Unmarshal([]byte(response), &toolCall); err != nil {
		return nil, err
	}
	return &toolCall, nil
}

func (m *MCPEnhancer) callMCPTool(ctx context.Context, client *client.Client, toolName string, args map[string]interface{}) (string, error) {
	callToolRequest := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	result, err := client.CallTool(ctx, callToolRequest)
	if err != nil {
		return "", fmt.Errorf("mcp tool call failed: %w", err)
	}

	var text strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			text.WriteString(textContent.Text)
			text.WriteString("\n")
		}
	}

	return text.String(), nil
}

// Close 释放缓存的 MCP 客户端连接。
func (m *MCPEnhancer) Close() {
	if m != nil && m.mcpClient != nil {
		_ = m.mcpClient.Close()
	}
}

func cloneMessagesWithLast(messages []*schema.Message, lastContent string) []*schema.Message {
	enhanced := make([]*schema.Message, len(messages))
	copy(enhanced, messages)
	last := *messages[len(messages)-1]
	last.Content = lastContent
	enhanced[len(enhanced)-1] = &last
	return enhanced
}
