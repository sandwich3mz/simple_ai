package session

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"simple_ai/common/aihelper"
	"simple_ai/common/code"
	appconfig "simple_ai/config"
	messageDAO "simple_ai/dao/message"
	sessionDAO "simple_ai/dao/session"
	"simple_ai/model"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ctx = context.Background()

// sessionStoreFuncs 抽象会话持久化依赖，便于从数据库恢复会话并在单测中替换实现。
type sessionStoreFuncs struct {
	createSession        func(*model.Session) (*model.Session, error)
	listByUserName       func(string) ([]model.Session, error)
	sessionBelongsToUser func(string, string) (bool, error)
	messagesBySession    func(string, string) ([]model.Message, error)
}

var sessionStore = sessionStoreFuncs{
	createSession:  sessionDAO.CreateSession,
	listByUserName: sessionDAO.GetSessionsByUserName,
	sessionBelongsToUser: func(userName, sessionID string) (bool, error) {
		// 发送已有会话前校验归属，防止用户伪造 sessionId 访问他人上下文。
		_, err := sessionDAO.GetSessionByIDAndUserName(sessionID, userName)
		if err != nil {
			return false, err
		}
		return true, nil
	},
	messagesBySession: messageDAO.GetMessagesByUserNameAndSessionID,
}

// ChatFeatureOptions 控制叠加到底座模型上的可选能力。
type ChatFeatureOptions struct {
	EnableRAG bool
	EnableMCP bool
}

// ChatRuntimeConfig 表示单次请求解析后的底座模型和增强器配置。
type ChatRuntimeConfig struct {
	BaseModelType string
	EnableRAG     bool
	EnableMCP     bool
}

// ResolveChatRuntimeConfig 将请求中的 modelType 和能力开关解析为底座模型加增强器。
func ResolveChatRuntimeConfig(modelType string, options ChatFeatureOptions) (ChatRuntimeConfig, error) {
	normalizedModelType := strings.ToLower(strings.TrimSpace(modelType))
	runtime := ChatRuntimeConfig{
		EnableRAG: options.EnableRAG,
		EnableMCP: options.EnableMCP,
	}

	switch normalizedModelType {
	case "qwen", "qwen-turbo", "qwen-plus", "qwen-max":
		runtime.BaseModelType = string(aihelper.AIModelProviderQwen)
	case "deepseek", "deepseek-chat", "deepseek-reasoner":
		runtime.BaseModelType = string(aihelper.AIModelProviderDeepSeek)
	case "rag", "2", "ali-rag", "alirag":
		runtime.BaseModelType = string(aihelper.AIModelProviderQwen)
		runtime.EnableRAG = true
	case "mcp", "3":
		runtime.BaseModelType = string(aihelper.AIModelProviderQwen)
		runtime.EnableMCP = true
	default:
		return ChatRuntimeConfig{}, fmt.Errorf("unsupported model type: %s", modelType)
	}

	return runtime, nil
}

func buildFactoryConfig(runtime ChatRuntimeConfig) (*aihelper.AIModelFactoryConfig, error) {
	providerConfig := appconfig.GetConfig().GetLLMProviderConfig(runtime.BaseModelType)

	provider := aihelper.AIModelProvider(runtime.BaseModelType)
	if providerConfig.Model == "" {
		switch provider {
		case aihelper.AIModelProviderQwen:
			providerConfig.Model = "qwen-turbo"
		case aihelper.AIModelProviderDeepSeek:
			providerConfig.Model = "deepseek-chat"
		}
	}

	factoryConfig := &aihelper.AIModelFactoryConfig{
		Provider: provider,
		APIKey:   providerConfig.APIKey,
		Model:    providerConfig.Model,
		BaseURL:  providerConfig.BaseURL,
	}
	if providerConfig.TimeoutSeconds > 0 {
		factoryConfig.Timeout = time.Duration(providerConfig.TimeoutSeconds) * time.Second
	}
	if providerConfig.MaxTokens > 0 {
		factoryConfig.MaxTokens = &providerConfig.MaxTokens
	}

	return factoryConfig, nil
}

func buildMessageEnhancers(ctx context.Context, userName string, runtime ChatRuntimeConfig) ([]aihelper.MessageEnhancer, error) {
	enhancers := make([]aihelper.MessageEnhancer, 0, 2)
	if runtime.EnableRAG {
		enhancers = append(enhancers, aihelper.NewRAGEnhancer(userName))
	}
	if runtime.EnableMCP {
		mcpEnhancer, err := aihelper.NewMCPEnhancer(ctx)
		if err != nil {
			return nil, err
		}
		enhancers = append(enhancers, mcpEnhancer)
	}
	return enhancers, nil
}

func ensureSessionBelongsToUser(userName string, sessionID string) code.Code {
	if sessionStore.sessionBelongsToUser == nil {
		return code.CodeServerBusy
	}

	ok, err := sessionStore.sessionBelongsToUser(userName, sessionID)
	if err != nil {
		log.Println("ensureSessionBelongsToUser error:", err)
		return code.CodeInvalidParams
	}
	if !ok {
		return code.CodeInvalidParams
	}
	return code.CodeSuccess
}

func restoreHelperMessages(userName string, sessionID string, helper *aihelper.AIHelper) code.Code {
	if sessionStore.messagesBySession == nil {
		return code.CodeServerBusy
	}

	messages, err := sessionStore.messagesBySession(userName, sessionID)
	if err != nil {
		log.Println("restoreHelperMessages error:", err)
		return code.CodeServerBusy
	}
	// 进程重启或 helper 被回收后，用数据库历史恢复模型上下文。
	helper.LoadMessages(messages)
	return code.CodeSuccess
}

func getOrCreateExistingSessionHelper(userName string, sessionID string, modelType string, options ChatFeatureOptions) (*aihelper.AIHelper, code.Code) {
	if codeValue := ensureSessionBelongsToUser(userName, sessionID); codeValue != code.CodeSuccess {
		return nil, codeValue
	}

	manager := aihelper.GetGlobalManager()
	_, alreadyInMemory := manager.GetAIHelper(userName, sessionID)

	// 每次请求按当前 modelType 与开关重建 runtime；历史只恢复消息，不恢复旧增强器配置。
	runtimeConfig, err := ResolveChatRuntimeConfig(modelType, options)
	if err != nil {
		log.Println("getOrCreateExistingSessionHelper ResolveChatRuntimeConfig error:", err)
		return nil, code.AIModelFail
	}
	factoryConfig, err := buildFactoryConfig(runtimeConfig)
	if err != nil {
		log.Println("getOrCreateExistingSessionHelper buildFactoryConfig error:", err)
		return nil, code.AIModelFail
	}
	enhancers, err := buildMessageEnhancers(ctx, userName, runtimeConfig)
	if err != nil {
		log.Println("getOrCreateExistingSessionHelper buildMessageEnhancers error:", err)
		return nil, code.AIModelFail
	}
	helper, err := manager.GetOrCreateAIHelper(userName, sessionID, factoryConfig, enhancers...)
	if err != nil {
		log.Println("getOrCreateExistingSessionHelper GetOrCreateAIHelper error:", err)
		return nil, code.AIModelFail
	}
	if !alreadyInMemory {
		// 只有新建 helper 时才恢复历史，避免覆盖当前进程中已追加但尚未持久化的消息。
		if codeValue := restoreHelperMessages(userName, sessionID, helper); codeValue != code.CodeSuccess {
			return nil, codeValue
		}
	}

	return helper, code.CodeSuccess
}

func historyFromMessages(messages []*model.Message) []model.History {
	history := make([]model.History, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		history = append(history, model.History{
			IsUser:  msg.IsUser,
			Content: msg.Content,
		})
	}
	return history
}

func historyFromStoredMessages(messages []model.Message) []model.History {
	history := make([]model.History, 0, len(messages))
	for i := range messages {
		history = append(history, model.History{
			IsUser:  messages[i].IsUser,
			Content: messages[i].Content,
		})
	}
	return history
}

// GetUserSessionsByUserName 从数据库返回指定用户的会话列表。
func GetUserSessionsByUserName(userName string) ([]model.SessionInfo, error) {
	sessions, err := sessionStore.listByUserName(userName)
	if err != nil {
		return nil, err
	}

	var sessionInfos []model.SessionInfo
	for _, eachSession := range sessions {
		title := eachSession.Title
		if title == "" {
			title = eachSession.ID
		}
		sessionInfos = append(sessionInfos, model.SessionInfo{
			SessionID: eachSession.ID,
			Title:     title,
		})
	}

	return sessionInfos, nil
}

// CreateSessionAndSendMessage 创建新会话，发送一条消息，并返回 AI 响应。
func CreateSessionAndSendMessage(userName string, userQuestion string, modelType string, options ChatFeatureOptions) (string, string, code.Code) {
	newSession := &model.Session{
		ID:       uuid.New().String(),
		UserName: userName,
		Title:    userQuestion,
	}
	createdSession, err := sessionStore.createSession(newSession)
	if err != nil {
		log.Println("CreateSessionAndSendMessage CreateSession error:", err)
		return "", "", code.CodeServerBusy
	}

	manager := aihelper.GetGlobalManager()
	runtimeConfig, err := ResolveChatRuntimeConfig(modelType, options)
	if err != nil {
		log.Println("CreateSessionAndSendMessage ResolveChatRuntimeConfig error:", err)
		return "", "", code.AIModelFail
	}
	factoryConfig, err := buildFactoryConfig(runtimeConfig)
	if err != nil {
		log.Println("CreateSessionAndSendMessage buildFactoryConfig error:", err)
		return "", "", code.AIModelFail
	}
	enhancers, err := buildMessageEnhancers(ctx, userName, runtimeConfig)
	if err != nil {
		log.Println("CreateSessionAndSendMessage buildMessageEnhancers error:", err)
		return "", "", code.AIModelFail
	}
	helper, err := manager.GetOrCreateAIHelper(userName, createdSession.ID, factoryConfig, enhancers...)
	if err != nil {
		log.Println("CreateSessionAndSendMessage GetOrCreateAIHelper error:", err)
		return "", "", code.AIModelFail
	}

	aiResponse, err := helper.GenerateResponse(userName, ctx, userQuestion)
	if err != nil {
		log.Println("CreateSessionAndSendMessage GenerateResponse error:", err)
		return "", "", code.AIModelFail
	}

	return createdSession.ID, aiResponse.Content, code.CodeSuccess
}

// CreateStreamSessionOnly 在流式响应开始前先创建会话。
func CreateStreamSessionOnly(userName string, userQuestion string) (string, code.Code) {
	newSession := &model.Session{
		ID:       uuid.New().String(),
		UserName: userName,
		Title:    userQuestion,
	}
	createdSession, err := sessionStore.createSession(newSession)
	if err != nil {
		log.Println("CreateStreamSessionOnly CreateSession error:", err)
		return "", code.CodeServerBusy
	}
	return createdSession.ID, code.CodeSuccess
}

// StreamMessageToExistingSession 为已有会话流式返回模型响应。
func StreamMessageToExistingSession(userName string, sessionID string, userQuestion string, modelType string, options ChatFeatureOptions, writer http.ResponseWriter) code.Code {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		log.Println("StreamMessageToExistingSession: streaming unsupported")
		return code.CodeServerBusy
	}

	helper, codeValue := getOrCreateExistingSessionHelper(userName, sessionID, modelType, options)
	if codeValue != code.CodeSuccess {
		return codeValue
	}

	cb := func(msg string) {
		log.Printf("[SSE] Sending chunk: %s (len=%d)\n", msg, len(msg))
		_, writeErr := writer.Write([]byte("data: " + msg + "\n\n"))
		if writeErr != nil {
			log.Println("[SSE] Write error:", writeErr)
			return
		}
		flusher.Flush()
		log.Println("[SSE] Flushed")
	}

	_, err := helper.StreamResponse(userName, ctx, cb, userQuestion)
	if err != nil {
		log.Println("StreamMessageToExistingSession StreamResponse error:", err)
		return code.AIModelFail
	}

	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	if err != nil {
		log.Println("StreamMessageToExistingSession write DONE error:", err)
		return code.AIModelFail
	}
	flusher.Flush()

	return code.CodeSuccess
}

// CreateStreamSessionAndSendMessage 创建会话并流式返回首条模型响应。
func CreateStreamSessionAndSendMessage(userName string, userQuestion string, modelType string, options ChatFeatureOptions, writer http.ResponseWriter) (string, code.Code) {
	sessionID, code_ := CreateStreamSessionOnly(userName, userQuestion)
	if code_ != code.CodeSuccess {
		return "", code_
	}

	code_ = StreamMessageToExistingSession(userName, sessionID, userQuestion, modelType, options, writer)
	if code_ != code.CodeSuccess {
		return sessionID, code_
	}

	return sessionID, code.CodeSuccess
}

// ChatSend 向已有会话发送非流式消息。
func ChatSend(userName string, sessionID string, userQuestion string, modelType string, options ChatFeatureOptions) (string, code.Code) {
	helper, codeValue := getOrCreateExistingSessionHelper(userName, sessionID, modelType, options)
	if codeValue != code.CodeSuccess {
		return "", codeValue
	}

	aiResponse, err := helper.GenerateResponse(userName, ctx, userQuestion)
	if err != nil {
		log.Println("ChatSend GenerateResponse error:", err)
		return "", code.AIModelFail
	}

	return aiResponse.Content, code.CodeSuccess
}

// GetChatHistory 优先返回内存历史；内存没有时回退到数据库历史。
func GetChatHistory(userName string, sessionID string) ([]model.History, code.Code) {
	manager := aihelper.GetGlobalManager()
	helper, exists := manager.GetAIHelper(userName, sessionID)
	if exists {
		return historyFromMessages(helper.GetMessages()), code.CodeSuccess
	}

	if sessionStore.messagesBySession == nil {
		return nil, code.CodeServerBusy
	}
	messages, err := sessionStore.messagesBySession(userName, sessionID)
	if err != nil {
		log.Println("GetChatHistory messagesBySession error:", err)
		return nil, code.CodeServerBusy
	}
	return historyFromStoredMessages(messages), code.CodeSuccess
}

// ChatStreamSend 为已有会话流式返回消息响应。
func ChatStreamSend(userName string, sessionID string, userQuestion string, modelType string, options ChatFeatureOptions, writer http.ResponseWriter) code.Code {
	return StreamMessageToExistingSession(userName, sessionID, userQuestion, modelType, options, writer)
}
