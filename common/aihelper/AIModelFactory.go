package aihelper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
)

// AIModelProvider 表示已注册的底座模型提供方名称。
type AIModelProvider string

const (
	AIModelProviderQwen     AIModelProvider = "qwen"
	AIModelProviderDeepSeek AIModelProvider = "deepseek"
	AIModelProviderRAG      AIModelProvider = "rag"
)

// AIModelFactoryConfig 保存创建 AIModel 所需的提供方配置。
type AIModelFactoryConfig struct {
	Provider AIModelProvider

	APIKey    string
	Model     string
	BaseURL   string
	Timeout   time.Duration
	MaxTokens *int
	UserName  string

	QwenConfig     *qwen.ChatModelConfig
	DeepSeekConfig *deepseek.ChatModelConfig
	OpenAIConfig   *openai.ChatModelConfig
}

// AIModelBuilder 为已注册的提供方创建 AIModel。
type AIModelBuilder func(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error)

// AIModelFactory 通过提供方注册表创建底座 LLM 模型。
type AIModelFactory struct {
	mutex    sync.RWMutex
	builders map[AIModelProvider]AIModelBuilder
}

func newAIModelFactory() *AIModelFactory {
	factory := &AIModelFactory{
		builders: make(map[AIModelProvider]AIModelBuilder),
	}
	factory.RegisterProvider(AIModelProviderQwen, buildQwenModel)
	factory.RegisterProvider(AIModelProviderDeepSeek, buildDeepSeekModel)
	return factory
}

var (
	globalAIModelFactory     *AIModelFactory
	globalAIModelFactoryOnce sync.Once
)

// GetGlobalAIModelFactory 返回已注册内置提供方的全局模型工厂。
func GetGlobalAIModelFactory() *AIModelFactory {
	globalAIModelFactoryOnce.Do(func() {
		globalAIModelFactory = newAIModelFactory()
	})
	return globalAIModelFactory
}

// Create 使用 config.Provider 对应的构造器创建 AIModel。
func (f *AIModelFactory) Create(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error) {
	if config == nil {
		return nil, fmt.Errorf("ai model factory config is nil")
	}

	builder, ok := f.getProvider(config.Provider)
	if !ok {
		return nil, fmt.Errorf("unsupported ai provider: %s", config.Provider)
	}

	return builder(ctx, config)
}

// RegisterProvider 在工厂注册表中新增或替换提供方构造器。
func (f *AIModelFactory) RegisterProvider(provider AIModelProvider, builder AIModelBuilder) {
	if provider == "" || builder == nil {
		return
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.builders[provider] = builder
}

func (f *AIModelFactory) getProvider(provider AIModelProvider) (AIModelBuilder, bool) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	builder, ok := f.builders[provider]
	return builder, ok
}

// CreateAIModel 通过全局提供方注册表创建 AIModel。
func CreateAIModel(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error) {
	return GetGlobalAIModelFactory().Create(ctx, config)
}

func dereferenceIntOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func buildQwenModel(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error) {
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
}

func buildDeepSeekModel(ctx context.Context, config *AIModelFactoryConfig) (AIModel, error) {
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
}
