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
