package redis

import (
	"context"
	"errors"
	"fmt"
	"simple_ai/config"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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

// InitRedisIndex 初始化 Redis 索引，支持按文件名区分
func InitRedisIndex(ctx context.Context, filename string, dimension int) error {
	indexName := GenerateIndexName(filename)

	// 检查索引是否存在
	_, err := Rdb.Do(ctx, "FT.INFO", indexName).Result()
	if err == nil {
		fmt.Println("索引已存在，跳过创建")
		return nil
	}

	// 如果索引不存在，创建新索引
	if !strings.Contains(err.Error(), "Unknown index name") {
		return fmt.Errorf("检查索引失败: %w", err)
	}

	fmt.Println("正在创建 Redis 索引...")

	prefix := GenerateIndexNamePrefix(filename)

	// 创建索引
	createArgs := []interface{}{
		"FT.CREATE", indexName,
		"ON", "HASH",
		"PREFIX", "1", prefix,
		"SCHEMA",
		"content", "TEXT",
		"metadata", "TEXT",
		"vector", "VECTOR", "FLAT",
		"6",
		"TYPE", "FLOAT32",
		"DIM", dimension,
		"DISTANCE_METRIC", "COSINE",
	}

	if err := Rdb.Do(ctx, createArgs...).Err(); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	fmt.Println("索引创建成功！")
	return nil
}

// DeleteRedisIndex 删除 Redis 索引，支持按文件名区分
func DeleteRedisIndex(ctx context.Context, filename string) error {
	indexName := GenerateIndexName(filename)

	// 删除索引
	if err := Rdb.Do(ctx, "FT.DROPINDEX", indexName).Err(); err != nil {
		return fmt.Errorf("删除索引失败: %w", err)
	}

	fmt.Println("索引删除成功！")
	return nil
}
