package aihelper

import (
	"context"
	"fmt"
	"io"
	"log"
	"simple_ai/common/rag"
	"simple_ai/config"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type AliRAGModel struct {
	llm      model.ToolCallingChatModel
	username string
}

func NewAliRAGModel(ctx context.Context, username string) (*AliRAGModel, error) {
	conf := config.GetConfig()
	return NewConfiguredAliRAGModel(ctx, username, &openai.ChatModelConfig{
		BaseURL: conf.RagModelConfig.RagBaseUrl,
		Model:   conf.RagModelConfig.RagChatModelName,
		APIKey:  conf.RagModelConfig.RagAPIKey,
	})
}

func NewConfiguredAliRAGModel(ctx context.Context, username string, chatModelConfig *openai.ChatModelConfig) (*AliRAGModel, error) {
	if chatModelConfig == nil {
		return nil, fmt.Errorf("ali rag model config is nil")
	}

	llm, err := openai.NewChatModel(ctx, chatModelConfig)
	if err != nil {
		return nil, fmt.Errorf("create ali rag model failed: %v", err)
	}

	return &AliRAGModel{
		llm:      llm,
		username: username,
	}, nil
}

func (o *AliRAGModel) GenerateResponse(ctx context.Context, messages *[]schema.Message) string {
	if o == nil || o.llm == nil {
		return ""
	}

	resp, err := o.generateResponse(ctx, toMessagePointers(messages))
	if err != nil {
		log.Printf("ali rag generate failed: %v", err)
		return ""
	}
	if resp == nil {
		return ""
	}
	return resp.Content
}

func (o *AliRAGModel) generateResponse(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	ragMessages, err := o.buildRAGMessages(ctx, messages)
	if err != nil {
		log.Printf("Failed to build RAG messages, fallback to raw messages: %v", err)
		ragMessages = messages
	}

	resp, err := o.llm.Generate(ctx, ragMessages)
	if err != nil {
		return nil, fmt.Errorf("ali rag generate failed: %v", err)
	}
	return resp, nil
}

func (o *AliRAGModel) StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error) {
	if o == nil || o.llm == nil {
		return "", fmt.Errorf("ali rag model is not initialized")
	}

	return o.streamResponse(ctx, toMessagePointers(messages), cb)
}

func (o *AliRAGModel) streamResponse(ctx context.Context, messages []*schema.Message, cb StreamCallback) (string, error) {
	ragMessages, err := o.buildRAGMessages(ctx, messages)
	if err != nil {
		log.Printf("Failed to build RAG messages, fallback to raw messages: %v", err)
		ragMessages = messages
	}

	return o.streamMessages(ctx, ragMessages, cb)
}

func (o *AliRAGModel) buildRAGMessages(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	ragQuery, err := rag.NewRAGQuery(ctx, o.username)
	if err != nil {
		return nil, err
	}

	query := messages[len(messages)-1].Content
	docs, err := ragQuery.RetrieveDocuments(ctx, query)
	if err != nil {
		return nil, err
	}

	ragMessages := make([]*schema.Message, len(messages))
	copy(ragMessages, messages)
	ragMessages[len(ragMessages)-1] = &schema.Message{
		Role:    schema.User,
		Content: rag.BuildRAGPrompt(query, docs),
	}
	return ragMessages, nil
}

func (o *AliRAGModel) streamMessages(ctx context.Context, messages []*schema.Message, cb StreamCallback) (string, error) {
	stream, err := o.llm.Stream(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("ali rag stream failed: %v", err)
	}
	defer stream.Close()

	var fullResp strings.Builder

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("ali rag stream recv failed: %v", err)
		}
		if msg != nil && len(msg.Content) > 0 {
			fullResp.WriteString(msg.Content)
			if cb != nil {
				cb(msg.Content)
			}
		}
	}

	return fullResp.String(), nil
}

func (o *AliRAGModel) GetModelType() string { return "2" }
