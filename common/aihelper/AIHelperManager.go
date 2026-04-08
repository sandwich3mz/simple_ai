package aihelper

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 用于管理用户与会话之间的 AIHelper 映射关系。
// 结构为：userName -> sessionID -> *AIHelper。
type Manager struct {
	mutex   sync.RWMutex
	helpers map[string]map[string]*AIHelper
}

// newAIHelperManager 创建 AIHelper 管理器实例，仅用于单例初始化。
func newAIHelperManager() *Manager {
	return &Manager{
		helpers: make(map[string]map[string]*AIHelper),
	}
}

// GetOrCreateAIHelper 获取或创建指定用户、指定会话的 AIHelper。
// 当会话不存在时，会基于 modelType 和 config 通过工厂创建 AIModel。
func (m *Manager) GetOrCreateAIHelper(userName, sessionID, modelType string, config map[string]interface{}) (*AIHelper, error) {
	if userName == "" {
		return nil, fmt.Errorf("userName 不能为空")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID 不能为空")
	}
	if modelType == "" {
		return nil, fmt.Errorf("modelType 不能为空")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	userHelpers, exists := m.helpers[userName]
	if !exists {
		userHelpers = make(map[string]*AIHelper)
		m.helpers[userName] = userHelpers
	}

	if helper, exists := userHelpers[sessionID]; exists {
		return helper, nil
	}

	factoryConfig, err := buildFactoryConfig(modelType, config)
	if err != nil {
		return nil, err
	}

	aiModel, err := GetGlobalAIModelFactory().Create(context.Background(), factoryConfig)
	if err != nil {
		return nil, err
	}

	helper := NewAIHelper(aiModel)
	helper.SessionID = sessionID
	userHelpers[sessionID] = helper

	return helper, nil
}

// GetAIHelper 获取指定用户的指定会话 AIHelper。
// 返回值中的 bool 表示是否命中。
func (m *Manager) GetAIHelper(userName, sessionID string) (*AIHelper, bool) {
	if userName == "" || sessionID == "" {
		return nil, false
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	userHelpers, ok := m.helpers[userName]
	if !ok {
		return nil, false
	}

	helper, exists := userHelpers[sessionID]
	return helper, exists
}

// RemoveAIHelper 移除指定用户的指定会话 AIHelper。
// 返回 true 表示已删除，false 表示目标不存在。
func (m *Manager) RemoveAIHelper(userName, sessionID string) bool {
	if userName == "" || sessionID == "" {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	userHelpers, ok := m.helpers[userName]
	if !ok {
		return false
	}

	if _, exists := userHelpers[sessionID]; !exists {
		return false
	}

	delete(userHelpers, sessionID)
	if len(userHelpers) == 0 {
		delete(m.helpers, userName)
	}

	return true
}

// GetUserSessionIDs 获取指定用户下的所有会话 ID。
// 返回结果按字典序排序，便于调用方稳定展示。
func (m *Manager) GetUserSessionIDs(userName string) []string {
	if userName == "" {
		return []string{}
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	userHelpers, ok := m.helpers[userName]
	if !ok {
		return []string{}
	}

	sessionIDs := make([]string, 0, len(userHelpers))
	for sessionID := range userHelpers {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)
	return sessionIDs
}

var (
	globalAIHelperManager     *Manager
	globalAIHelperManagerOnce sync.Once
)

// GetGlobalManager 返回全局唯一的 AIHelper 管理器实例（单例）。
func GetGlobalManager() *Manager {
	globalAIHelperManagerOnce.Do(func() {
		globalAIHelperManager = newAIHelperManager()
	})
	return globalAIHelperManager
}

func buildFactoryConfig(modelType string, config map[string]interface{}) (*AIModelFactoryConfig, error) {
	provider, err := normalizeProvider(modelType)
	if err != nil {
		return nil, err
	}

	factoryConfig := &AIModelFactoryConfig{
		Provider: provider,
		APIKey:   getString(config, "apiKey"),
		Model:    getString(config, "model"),
		BaseURL:  getString(config, "baseURL"),
	}

	if timeoutSeconds, ok := getInt(config, "timeoutSeconds"); ok && timeoutSeconds > 0 {
		factoryConfig.Timeout = time.Duration(timeoutSeconds) * time.Second
	}
	if maxTokens, ok := getInt(config, "maxTokens"); ok && maxTokens > 0 {
		factoryConfig.MaxTokens = &maxTokens
	}

	if factoryConfig.Model == "" {
		switch provider {
		case AIModelProviderQwen:
			factoryConfig.Model = "qwen-turbo"
		case AIModelProviderDeepSeek:
			factoryConfig.Model = "deepseek-chat"
		}
	}

	return factoryConfig, nil
}

func normalizeProvider(modelType string) (AIModelProvider, error) {
	switch strings.ToLower(strings.TrimSpace(modelType)) {
	case string(AIModelProviderQwen), "qwen-turbo", "qwen-plus", "qwen-max":
		return AIModelProviderQwen, nil
	case string(AIModelProviderDeepSeek), "deepseek-chat", "deepseek-reasoner":
		return AIModelProviderDeepSeek, nil
	default:
		return "", fmt.Errorf("不支持的模型类型: %s", modelType)
	}
}

func getString(config map[string]interface{}, key string) string {
	if config == nil {
		return ""
	}
	raw, ok := config[key]
	if !ok || raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func getInt(config map[string]interface{}, key string) (int, bool) {
	if config == nil {
		return 0, false
	}
	raw, ok := config[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch value := raw.(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}
