package aihelper

import (
	"context"
	"fmt"
	"simple_ai/model"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 按 userName -> sessionID -> helper 的结构管理会话助手。
type Manager struct {
	mutex   sync.RWMutex
	helpers map[string]map[string]*AIHelper
}

func newAIHelperManager() *Manager {
	return &Manager{
		helpers: make(map[string]map[string]*AIHelper),
	}
}

// GetOrCreateAIHelper 返回已有会话助手，或用指定底座模型和增强器创建新助手。
func (m *Manager) GetOrCreateAIHelper(userName, sessionID string, factoryConfig *AIModelFactoryConfig, enhancers ...MessageEnhancer) (*AIHelper, error) {
	if userName == "" {
		return nil, fmt.Errorf("userName cannot be empty")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}
	if factoryConfig == nil {
		return nil, fmt.Errorf("ai model factory config is nil")
	}
	if factoryConfig.Provider == "" {
		return nil, fmt.Errorf("model provider cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	userHelpers, exists := m.helpers[userName]
	if !exists {
		userHelpers = make(map[string]*AIHelper)
		m.helpers[userName] = userHelpers
	}

	if helper, exists := userHelpers[sessionID]; exists {
		if strings.EqualFold(helper.GetModelType(), string(factoryConfig.Provider)) {
			helper.SetMessageEnhancers(enhancers...)
			return helper, nil
		}
		messages := helper.GetMessages()
		nextHelper, err := m.createAIHelper(userName, sessionID, factoryConfig, enhancers...)
		if err != nil {
			return nil, err
		}
		nextHelper.LoadMessages(messageSnapshotsToValues(messages))
		userHelpers[sessionID] = nextHelper
		return nextHelper, nil
	}

	helper, err := m.createAIHelper(userName, sessionID, factoryConfig, enhancers...)
	if err != nil {
		return nil, err
	}
	userHelpers[sessionID] = helper

	return helper, nil
}

func (m *Manager) createAIHelper(userName, sessionID string, factoryConfig *AIModelFactoryConfig, enhancers ...MessageEnhancer) (*AIHelper, error) {
	factoryConfig.UserName = userName
	aiModel, err := GetGlobalAIModelFactory().Create(context.Background(), factoryConfig)
	if err != nil {
		return nil, err
	}

	return NewAIHelper(aiModel, sessionID, WithMessageEnhancers(enhancers...)), nil
}

// GetAIHelper 在会话仍驻留内存时返回对应助手。
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

// RemoveAIHelper 删除指定用户的一个内存会话助手。
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

// GetUserSessionIDs 返回指定用户的内存会话 ID 列表，并按字典序排序。
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

// GetGlobalManager 返回全局单例会话助手管理器。
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
		return "", fmt.Errorf("unsupported model provider: %s", modelType)
	}
}

func messageSnapshotsToValues(messages []*model.Message) []model.Message {
	values := make([]model.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		values = append(values, *msg)
	}
	return values
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
