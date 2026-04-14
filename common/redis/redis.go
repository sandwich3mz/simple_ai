package redis

import (
	"context"
	"errors"
	"fmt"
	"simple_ai/config"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

var Rdb *redis.Client

var ctx = context.Background()
var ErrAIConfigNotFound = errors.New("ai config not found")

func Init() {
	conf := config.GetConfig()
	host := conf.RedisConfig.RedisHost
	port := conf.RedisConfig.RedisPort
	password := conf.RedisConfig.RedisPassword
	db := conf.RedisDb
	addr := host + ":" + strconv.Itoa(port)

	Rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

}

func SetCaptchaForEmail(email, captcha string) error {
	key := GenerateCaptcha(email)
	expire := 2 * time.Minute
	return Rdb.Set(ctx, key, captcha, expire).Err()
}

func CheckCaptchaForEmail(email, userInput string) (bool, error) {
	key := GenerateCaptcha(email)

	storedCaptcha, err := Rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {

			return false, nil
		}

		return false, err
	}

	if strings.EqualFold(storedCaptcha, userInput) {

		// 验证成功后删除 key
		if err := Rdb.Del(ctx, key).Err(); err != nil {

		} else {

		}
		return true, nil
	}

	return false, nil
}

func SaveAIConfig(cfg config.AIConfig) error {
	key := GenerateAIConfigKey()
	values := map[string]interface{}{
		"qwenApiKey":      cfg.QwenAPIKey,
		"qwenModel":       cfg.QwenModel,
		"qwenBaseURL":     cfg.QwenBaseURL,
		"deepseekApiKey":  cfg.DeepseekAPIKey,
		"deepseekModel":   cfg.DeepseekModel,
		"deepseekBaseURL": cfg.DeepseekBaseURL,
		"timeoutSeconds":  cfg.TimeoutSeconds,
		"maxTokens":       cfg.MaxTokens,
	}
	return Rdb.HSet(ctx, key, values).Err()
}

func GetAIConfig() (config.AIConfig, error) {
	key := GenerateAIConfigKey()
	values, err := Rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return config.AIConfig{}, err
	}
	if len(values) == 0 {
		return config.AIConfig{}, ErrAIConfigNotFound
	}

	timeoutSeconds, err := parseRedisInt(values["timeoutSeconds"])
	if err != nil {
		return config.AIConfig{}, fmt.Errorf("invalid timeoutSeconds in redis: %w", err)
	}
	maxTokens, err := parseRedisInt(values["maxTokens"])
	if err != nil {
		return config.AIConfig{}, fmt.Errorf("invalid maxTokens in redis: %w", err)
	}

	return config.AIConfig{
		QwenAPIKey:      values["qwenApiKey"],
		QwenModel:       values["qwenModel"],
		QwenBaseURL:     values["qwenBaseURL"],
		DeepseekAPIKey:  values["deepseekApiKey"],
		DeepseekModel:   values["deepseekModel"],
		DeepseekBaseURL: values["deepseekBaseURL"],
		TimeoutSeconds:  timeoutSeconds,
		MaxTokens:       maxTokens,
	}, nil
}

func EnsureAIConfig(defaultCfg config.AIConfig) (config.AIConfig, error) {
	cfg, err := GetAIConfig()
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, ErrAIConfigNotFound) {
		if saveErr := SaveAIConfig(defaultCfg); saveErr != nil {
			return defaultCfg, saveErr
		}
		return defaultCfg, nil
	}
	return defaultCfg, err
}

func parseRedisInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
