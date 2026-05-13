package session

import (
	"fmt"
	"net/http"
	"simple_ai/common/code"
	"simple_ai/controller"
	"simple_ai/model"
	"simple_ai/service/session"

	"github.com/gin-gonic/gin"
)

type (
	GetUserSessionsResponse struct {
		controller.Response
		Sessions []model.SessionInfo `json:"sessions,omitempty"`
	}

	CreateSessionAndSendMessageRequest struct {
		UserQuestion string `json:"question" binding:"required"`
		ModelType    string `json:"modelType" binding:"required"`
		EnableRAG    bool   `json:"enableRag,omitempty"`
		EnableMCP    bool   `json:"enableMcp,omitempty"`
	}

	CreateSessionAndSendMessageResponse struct {
		AiInformation string `json:"Information,omitempty"`
		SessionID     string `json:"sessionId,omitempty"`
		controller.Response
	}

	ChatSendRequest struct {
		UserQuestion string `json:"question" binding:"required"`
		ModelType    string `json:"modelType" binding:"required"`
		SessionID    string `json:"sessionId,omitempty" binding:"required"`
		EnableRAG    bool   `json:"enableRag,omitempty"`
		EnableMCP    bool   `json:"enableMcp,omitempty"`
	}

	ChatSendResponse struct {
		AiInformation string `json:"Information,omitempty"`
		controller.Response
	}

	ChatHistoryRequest struct {
		SessionID string `json:"sessionId,omitempty" binding:"required"`
	}

	ChatHistoryResponse struct {
		History []model.History `json:"history"`
		controller.Response
	}
)

func GetUserSessionsByUserName(c *gin.Context) {
	res := new(GetUserSessionsResponse)
	userName := c.GetString("userName")

	userSessions, err := session.GetUserSessionsByUserName(userName)
	if err != nil {
		c.JSON(http.StatusOK, res.CodeOf(code.CodeServerBusy))
		return
	}

	res.Success()
	res.Sessions = userSessions
	c.JSON(http.StatusOK, res)
}

func CreateSessionAndSendMessage(c *gin.Context) {
	req := new(CreateSessionAndSendMessageRequest)
	res := new(CreateSessionAndSendMessageResponse)
	userName := c.GetString("userName")
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, res.CodeOf(code.CodeInvalidParams))
		return
	}

	options := session.ChatFeatureOptions{EnableRAG: req.EnableRAG, EnableMCP: req.EnableMCP}
	sessionID, aiInformation, codeValue := session.CreateSessionAndSendMessage(userName, req.UserQuestion, req.ModelType, options)
	if codeValue != code.CodeSuccess {
		c.JSON(http.StatusOK, res.CodeOf(codeValue))
		return
	}

	res.Success()
	res.AiInformation = aiInformation
	res.SessionID = sessionID
	c.JSON(http.StatusOK, res)
}

func CreateStreamSessionAndSendMessage(c *gin.Context) {
	req := new(CreateSessionAndSendMessageRequest)
	userName := c.GetString("userName")
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "Invalid parameters"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	sessionID, codeValue := session.CreateStreamSessionOnly(userName, req.UserQuestion)
	if codeValue != code.CodeSuccess {
		c.SSEvent("error", gin.H{"message": "Failed to create session"})
		return
	}

	c.Writer.WriteString(fmt.Sprintf("data: {\"sessionId\": \"%s\"}\n\n", sessionID))
	c.Writer.Flush()

	options := session.ChatFeatureOptions{EnableRAG: req.EnableRAG, EnableMCP: req.EnableMCP}
	codeValue = session.StreamMessageToExistingSession(userName, sessionID, req.UserQuestion, req.ModelType, options, http.ResponseWriter(c.Writer))
	if codeValue != code.CodeSuccess {
		c.SSEvent("error", gin.H{"message": "Failed to send message"})
		return
	}
}

func ChatSend(c *gin.Context) {
	req := new(ChatSendRequest)
	res := new(ChatSendResponse)
	userName := c.GetString("userName")
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, res.CodeOf(code.CodeInvalidParams))
		return
	}

	options := session.ChatFeatureOptions{EnableRAG: req.EnableRAG, EnableMCP: req.EnableMCP}
	aiInformation, codeValue := session.ChatSend(userName, req.SessionID, req.UserQuestion, req.ModelType, options)
	if codeValue != code.CodeSuccess {
		c.JSON(http.StatusOK, res.CodeOf(codeValue))
		return
	}

	res.Success()
	res.AiInformation = aiInformation
	c.JSON(http.StatusOK, res)
}

func ChatStreamSend(c *gin.Context) {
	req := new(ChatSendRequest)
	userName := c.GetString("userName")
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "Invalid parameters"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	options := session.ChatFeatureOptions{EnableRAG: req.EnableRAG, EnableMCP: req.EnableMCP}
	codeValue := session.ChatStreamSend(userName, req.SessionID, req.UserQuestion, req.ModelType, options, http.ResponseWriter(c.Writer))
	if codeValue != code.CodeSuccess {
		c.SSEvent("error", gin.H{"message": "Failed to send message"})
		return
	}
}

func ChatHistory(c *gin.Context) {
	req := new(ChatHistoryRequest)
	res := new(ChatHistoryResponse)
	userName := c.GetString("userName")
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, res.CodeOf(code.CodeInvalidParams))
		return
	}

	history, codeValue := session.GetChatHistory(userName, req.SessionID)
	if codeValue != code.CodeSuccess {
		c.JSON(http.StatusOK, res.CodeOf(codeValue))
		return
	}

	res.Success()
	res.History = history
	c.JSON(http.StatusOK, res)
}
