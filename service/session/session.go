package session

import (
	"context"
	"log"
	"net/http"
	"simple_ai/common/aihelper"
	"simple_ai/common/code"
	myredis "simple_ai/common/redis"
	appconfig "simple_ai/config"
	"simple_ai/dao/session"
	"simple_ai/model"
	"strings"

	"github.com/google/uuid"
)

var ctx = context.Background()

func buildAIConfig(modelType string) map[string]interface{} {
	conf := getAIConfig()
	helperConfig := map[string]interface{}{}

	normalizedModelType := strings.ToLower(strings.TrimSpace(modelType))
	switch normalizedModelType {
	case "qwen", "qwen-turbo", "qwen-plus", "qwen-max":
		helperConfig["apiKey"] = conf.QwenAPIKey
		if conf.QwenModel != "" {
			helperConfig["model"] = conf.QwenModel
		}
		if conf.QwenBaseURL != "" {
			helperConfig["baseURL"] = conf.QwenBaseURL
		}
	case "deepseek", "deepseek-chat", "deepseek-reasoner":
		helperConfig["apiKey"] = conf.DeepseekAPIKey
		if conf.DeepseekModel != "" {
			helperConfig["model"] = conf.DeepseekModel
		}
		if conf.DeepseekBaseURL != "" {
			helperConfig["baseURL"] = conf.DeepseekBaseURL
		}
	}

	if conf.TimeoutSeconds > 0 {
		helperConfig["timeoutSeconds"] = conf.TimeoutSeconds
	}
	if conf.MaxTokens > 0 {
		helperConfig["maxTokens"] = conf.MaxTokens
	}

	return helperConfig
}

func getAIConfig() appconfig.AIConfig {
	localCfg := appconfig.GetConfig().AIConfig
	cfg, err := myredis.EnsureAIConfig(localCfg)
	if err != nil {
		log.Printf("get ai config from redis failed, fallback to local config: %v", err)
		return localCfg
	}
	return cfg
}

func GetUserSessionsByUserName(userName string) ([]model.SessionInfo, error) {
	manager := aihelper.GetGlobalManager()
	sessions := manager.GetUserSessionIDs(userName)

	var sessionInfos []model.SessionInfo
	for _, eachSession := range sessions {
		sessionInfos = append(sessionInfos, model.SessionInfo{
			SessionID: eachSession,
			Title:     eachSession,
		})
	}

	return sessionInfos, nil
}

func CreateSessionAndSendMessage(userName string, userQuestion string, modelType string) (string, string, code.Code) {
	newSession := &model.Session{
		ID:       uuid.New().String(),
		UserName: userName,
		Title:    userQuestion,
	}
	createdSession, err := session.CreateSession(newSession)
	if err != nil {
		log.Println("CreateSessionAndSendMessage CreateSession error:", err)
		return "", "", code.CodeServerBusy
	}

	manager := aihelper.GetGlobalManager()
	helperConfig := buildAIConfig(modelType)
	helper, err := manager.GetOrCreateAIHelper(userName, createdSession.ID, modelType, helperConfig)
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

func CreateStreamSessionOnly(userName string, userQuestion string) (string, code.Code) {
	newSession := &model.Session{
		ID:       uuid.New().String(),
		UserName: userName,
		Title:    userQuestion,
	}
	createdSession, err := session.CreateSession(newSession)
	if err != nil {
		log.Println("CreateStreamSessionOnly CreateSession error:", err)
		return "", code.CodeServerBusy
	}
	return createdSession.ID, code.CodeSuccess
}

func StreamMessageToExistingSession(userName string, sessionID string, userQuestion string, modelType string, writer http.ResponseWriter) code.Code {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		log.Println("StreamMessageToExistingSession: streaming unsupported")
		return code.CodeServerBusy
	}

	manager := aihelper.GetGlobalManager()
	helperConfig := buildAIConfig(modelType)
	helper, err := manager.GetOrCreateAIHelper(userName, sessionID, modelType, helperConfig)
	if err != nil {
		log.Println("StreamMessageToExistingSession GetOrCreateAIHelper error:", err)
		return code.AIModelFail
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

	_, err = helper.StreamResponse(userName, ctx, cb, userQuestion)
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

func CreateStreamSessionAndSendMessage(userName string, userQuestion string, modelType string, writer http.ResponseWriter) (string, code.Code) {
	sessionID, code_ := CreateStreamSessionOnly(userName, userQuestion)
	if code_ != code.CodeSuccess {
		return "", code_
	}

	code_ = StreamMessageToExistingSession(userName, sessionID, userQuestion, modelType, writer)
	if code_ != code.CodeSuccess {
		return sessionID, code_
	}

	return sessionID, code.CodeSuccess
}

func ChatSend(userName string, sessionID string, userQuestion string, modelType string) (string, code.Code) {
	manager := aihelper.GetGlobalManager()
	helperConfig := buildAIConfig(modelType)
	helper, err := manager.GetOrCreateAIHelper(userName, sessionID, modelType, helperConfig)
	if err != nil {
		log.Println("ChatSend GetOrCreateAIHelper error:", err)
		return "", code.AIModelFail
	}

	aiResponse, err := helper.GenerateResponse(userName, ctx, userQuestion)
	if err != nil {
		log.Println("ChatSend GenerateResponse error:", err)
		return "", code.AIModelFail
	}

	return aiResponse.Content, code.CodeSuccess
}

func GetChatHistory(userName string, sessionID string) ([]model.History, code.Code) {
	manager := aihelper.GetGlobalManager()
	helper, exists := manager.GetAIHelper(userName, sessionID)
	if !exists {
		return nil, code.CodeServerBusy
	}

	messages := helper.GetMessages()
	history := make([]model.History, 0, len(messages))

	for i, msg := range messages {
		isUser := i%2 == 0
		history = append(history, model.History{
			IsUser:  isUser,
			Content: msg.Content,
		})
	}

	return history, code.CodeSuccess
}

func ChatStreamSend(userName string, sessionID string, userQuestion string, modelType string, writer http.ResponseWriter) code.Code {
	return StreamMessageToExistingSession(userName, sessionID, userQuestion, modelType, writer)
}
