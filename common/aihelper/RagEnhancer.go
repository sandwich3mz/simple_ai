package aihelper

import (
	"context"
	"fmt"
	"simple_ai/common/rag"

	"github.com/cloudwego/eino/schema"
)

// RAGEnhancer 将检索到的用户知识库内容注入最后一条用户消息。
type RAGEnhancer struct {
	userName string
}

// NewRAGEnhancer 创建绑定到指定用户知识库文件的 RAG 增强器。
func NewRAGEnhancer(userName string) *RAGEnhancer {
	return &RAGEnhancer{userName: userName}
}

// EnhanceMessages 检索相关文档，并用 RAG 上下文改写最后一条消息。
func (r *RAGEnhancer) EnhanceMessages(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
	if r == nil || r.userName == "" {
		return messages, fmt.Errorf("rag enhancer requires userName")
	}
	if len(messages) == 0 {
		return messages, nil
	}

	ragQuery, err := rag.NewRAGQuery(ctx, r.userName)
	if err != nil {
		return nil, err
	}

	query := messages[len(messages)-1].Content
	docs, err := ragQuery.RetrieveDocuments(ctx, query)
	if err != nil {
		return nil, err
	}

	enhanced := make([]*schema.Message, len(messages))
	copy(enhanced, messages)
	enhanced[len(enhanced)-1] = &schema.Message{
		Role:    schema.User,
		Content: rag.BuildRAGPrompt(query, docs),
	}
	return enhanced, nil
}
