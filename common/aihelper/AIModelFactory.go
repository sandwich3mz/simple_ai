package aihelper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/qwen"
)

type AIModelProvider string

const (
	// AIModelProviderQwen 表示使用 Qwen 模型实现。
	AIModelProviderQwen AIModelProvider = "qwen"
	// AIModelProviderDeepSeek 表示使用 DeepSeek 模型实现。
	AIModelProviderDeepSeek AIModelProvider = "deepseek"
)

// AIModelFactoryConfig 是模型工厂的创建参数。
// 当提供 QwenConfig 或 DeepSeekConfig 时，会优先使用对应专用配置。
type AIModelFactoryConfig struct {
	Provider AIModelProvider

	APIKey    string
	Model     string
	BaseURL   string
	Timeout   time.Duration
	MaxTokens *int

	QwenConfig     *qwen.ChatModelConfig
	DeepSeekConfig *deepseek.ChatModelConfig
}

// AIModelFactory 负责创建不同 Provider 的 AIModel 实例。
type AIModelFactory struct{}

// newAIModelFactory 创建工厂实例，仅供单例初始化流程内部使用。
func newAIModelFactory() *AIModelFactory {
	return &AIModelFactory{}
}

var (
	globalAIModelFactory     *AIModelFactory
	globalAIModelFactoryOnce sync.Once
)

// GetGlobalAIModelFactory 返回全局唯一的工厂实例（单例）。
func GetGlobalAIModelFactory() *AIModelFactory {
	globalAIModelFactoryOnce.Do(func() {
		globalAIModelFactory = newAIModelFactory()
	})
	return globalAIModelFactory
}

// Create 按 Provider 创建对应的 AIModel。
// 支持使用通用字段创建，也支持直接传入各模型原生配置。
func (f *AIModelFactory) Create(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error) {
	if config == nil {
		return nil, fmt.Errorf("ai model factory config is nil")
	}

	switch config.Provider {
	case AIModelProviderQwen:
		qwenConfig := config.QwenConfig
		if qwenConfig == nil {
			if config.APIKey == "" || config.Model == "" {
				return nil, fmt.Errorf("qwen apiKey and model are required")
			}
			baseURL := config.BaseURL
			if baseURL == "" {
				baseURL = defaultQwenBaseURL
			}
			qwenConfig = &qwen.ChatModelConfig{
				APIKey:    config.APIKey,
				Model:     config.Model,
				BaseURL:   baseURL,
				Timeout:   config.Timeout,
				MaxTokens: config.MaxTokens,
			}
		}

		return NewQwenAIModel(ctx, qwenConfig)

	case AIModelProviderDeepSeek:
		deepSeekConfig := config.DeepSeekConfig
		if deepSeekConfig == nil {
			if config.APIKey == "" || config.Model == "" {
				return nil, fmt.Errorf("deepseek apiKey and model are required")
			}
			baseURL := config.BaseURL
			if baseURL == "" {
				baseURL = defaultDeepSeekBaseURL
			}
			deepSeekConfig = &deepseek.ChatModelConfig{
				APIKey:    config.APIKey,
				Model:     config.Model,
				BaseURL:   baseURL,
				Timeout:   config.Timeout,
				MaxTokens: dereferenceIntOrZero(config.MaxTokens),
			}
		}

		return NewDeepSeekAIModel(ctx, deepSeekConfig)

	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", config.Provider)
	}
}

// CreateAIModel 通过全局单例工厂创建 AIModel。
func CreateAIModel(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error) {
	return GetGlobalAIModelFactory().Create(ctx, config)
}

// dereferenceIntOrZero 将可选的 int 指针转为 int，nil 时返回 0。
func dereferenceIntOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
