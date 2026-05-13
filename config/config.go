package config

import (
	"log"

	"github.com/BurntSushi/toml"
)

type MainConfig struct {
	Port    int    `toml:"port"`
	AppName string `toml:"appName"`
	Host    string `toml:"host"`
}

type EmailConfig struct {
	Authcode string `toml:"authcode"`
	Email    string `toml:"email" `
}

type RedisConfig struct {
	RedisPort     int    `toml:"port"`
	RedisDb       int    `toml:"db"`
	RedisHost     string `toml:"host"`
	RedisPassword string `toml:"password"`
}

type MysqlConfig struct {
	MysqlPort         int    `toml:"port"`
	MysqlHost         string `toml:"host"`
	MysqlUser         string `toml:"user"`
	MysqlPassword     string `toml:"password"`
	MysqlDatabaseName string `toml:"databaseName"`
	MysqlCharset      string `toml:"charset"`
}

type JwtConfig struct {
	ExpireDuration int    `toml:"expire_duration"`
	Issuer         string `toml:"issuer"`
	Subject        string `toml:"subject"`
	Key            string `toml:"key"`
}

type Rabbitmq struct {
	RabbitmqPort     int    `toml:"port"`
	RabbitmqHost     string `toml:"host"`
	RabbitmqUsername string `toml:"username"`
	RabbitmqPassword string `toml:"password"`
	RabbitmqVhost    string `toml:"vhost"`
}

// LLMProviderConfig 描述一个可配置的底座模型提供方。
type LLMProviderConfig struct {
	APIKey         string `toml:"apiKey"`
	Model          string `toml:"model"`
	BaseURL        string `toml:"baseURL"`
	TimeoutSeconds int    `toml:"timeoutSeconds"`
	MaxTokens      int    `toml:"maxTokens"`
}

// LLMConfig 汇总底座模型提供方配置和默认提供方。
type LLMConfig struct {
	DefaultProvider string            `toml:"defaultProvider"`
	Qwen            LLMProviderConfig `toml:"qwen"`
	DeepSeek        LLMProviderConfig `toml:"deepseek"`
}

type RagModelConfig struct {
	RagAPIKey         string `toml:"apiKey"`
	RagEmbeddingModel string `toml:"embeddingModel"`
	RagChatModelName  string `toml:"chatModelName"`
	RagDocDir         string `toml:"docDir"`
	RagBaseUrl        string `toml:"baseUrl"`
	RagDimension      int    `toml:"dimension"`
}

type VoiceServiceConfig struct {
	VoiceServiceApiKey    string `toml:"voiceServiceApiKey"`
	VoiceServiceSecretKey string `toml:"voiceServiceSecretKey"`
}

// MCPConfig 描述 MCP 增强器及其路由模型配置。
type MCPConfig struct {
	Enabled        bool   `toml:"enabled"`
	ServerURL      string `toml:"serverURL"`
	APIKey         string `toml:"apiKey"`
	Model          string `toml:"model"`
	BaseURL        string `toml:"baseURL"`
	TimeoutSeconds int    `toml:"timeoutSeconds"`
	MaxTokens      int    `toml:"maxTokens"`
}

var DefaultRedisKeyConfig = RedisKeyConfig{
	CaptchaPrefix:   "captcha:%s",
	IndexName:       "rag_docs:%s:idx",
	IndexNamePrefix: "rag_docs:%s:",
}

type Config struct {
	EmailConfig        `toml:"emailConfig"`
	RedisConfig        `toml:"redisConfig"`
	MysqlConfig        `toml:"mysqlConfig"`
	JwtConfig          `toml:"jwtConfig"`
	MainConfig         `toml:"mainConfig"`
	Rabbitmq           `toml:"rabbitmqConfig"`
	LLMConfig          `toml:"llm"`
	RagModelConfig     `toml:"ragModelConfig"`
	MCPConfig          `toml:"mcpConfig"`
	VoiceServiceConfig `toml:"voiceServiceConfig"`
}

type RedisKeyConfig struct {
	CaptchaPrefix   string
	IndexName       string
	IndexNamePrefix string
}

var config *Config

// InitConfig 初始化项目配置
func InitConfig() error {
	// 设置配置文件路径（相对于 main.go 所在的目录）
	if _, err := toml.DecodeFile("config/config.toml", config); err != nil {
		log.Fatal(err.Error())
		return err
	}
	return nil
}

func GetConfig() *Config {
	if config == nil {
		config = new(Config)
		_ = InitConfig()
	}
	return config
}

// GetDefaultProvider 返回配置中的默认底座模型提供方。
func (c *Config) GetDefaultProvider() string {
	if c != nil && c.LLMConfig.DefaultProvider != "" {
		return c.LLMConfig.DefaultProvider
	}
	return "qwen"
}

// GetLLMProviderConfig 返回指定底座模型提供方配置。
func (c *Config) GetLLMProviderConfig(provider string) LLMProviderConfig {
	if c == nil {
		return LLMProviderConfig{}
	}

	switch provider {
	case "qwen":
		return c.LLMConfig.Qwen
	case "deepseek":
		return c.LLMConfig.DeepSeek
	default:
		return LLMProviderConfig{}
	}
}
