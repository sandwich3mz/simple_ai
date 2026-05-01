package redis

import (
	"fmt"
	"simple_ai/config"
)

func GenerateCaptcha(email string) string {
	return fmt.Sprintf(config.DefaultRedisKeyConfig.CaptchaPrefix, email)
}

func GenerateAIConfigKey() string {
	return config.DefaultRedisKeyConfig.AIConfigKey
}

func GenerateIndexNamePrefix(filename string) string {
	prefix := fmt.Sprintf(config.DefaultRedisKeyConfig.IndexNamePrefix, filename)
	return prefix
}

func GenerateIndexName(filename string) string {
	indexName := fmt.Sprintf(config.DefaultRedisKeyConfig.IndexName, filename)
	return indexName
}
