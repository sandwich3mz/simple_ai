package aihelper

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/schema"
)

const defaultQwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

// QwenAIModel 是基于 eino-ext qwen.ChatModel 的 AIModel 适配实现。
type QwenAIModel struct {
	chatModel *qwen.ChatModel
}

// NewQwenAIModel 使用完整的 qwen 配置创建模型实例。
func NewQwenAIModel(ctx context.Context, config *qwen.ChatModelConfig) (*QwenAIModel, error) {
	chatModel, err := qwen.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return &QwenAIModel{
		chatModel: chatModel,
	}, nil
}

// NewDashScopeQwenAIModel 使用 DashScope 兼容地址创建 qwen 模型实例。
func NewDashScopeQwenAIModel(ctx context.Context, apiKey, modelName string) (*QwenAIModel, error) {
	return NewQwenAIModel(ctx, &qwen.ChatModelConfig{
		BaseURL: defaultQwenBaseURL,
		APIKey:  apiKey,
		Model:   modelName,
	})
}

// GenerateResponse 执行非流式对话并返回文本结果。
func (q *QwenAIModel) GenerateResponse(ctx context.Context, messages *[]schema.Message) string {
	if q == nil || q.chatModel == nil {
		return ""
	}

	resp, err := q.chatModel.Generate(ctx, toMessagePointers(messages))
	if err != nil || resp == nil {
		return ""
	}

	return resp.Content
}

// StreamResponse 执行流式对话；每个分片通过回调返回，最终返回合并后的完整文本。
func (q *QwenAIModel) StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error) {
	if q == nil || q.chatModel == nil {
		return "", errors.New("qwen model is not initialized")
	}

	sr, err := q.chatModel.Stream(ctx, toMessagePointers(messages))
	if err != nil {
		return "", err
	}
	defer sr.Close()

	msgs := make([]*schema.Message, 0, 16)
	var content strings.Builder

	for {
		msg, recvErr := sr.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return content.String(), recvErr
		}
		if msg == nil {
			continue
		}

		msgs = append(msgs, msg)
		if len(msg.Content) > 0 {
			content.WriteString(msg.Content)
			if cb != nil {
				cb(msg.Content)
			}
		}
	}

	merged, mergeErr := schema.ConcatMessages(msgs)
	if mergeErr == nil && merged != nil && len(merged.Content) > 0 {
		return merged.Content, nil
	}

	return content.String(), nil
}

func (q *QwenAIModel) GetModelType() string {
	return "Qwen"
}

// toMessagePointers 将 []schema.Message 转为 []*schema.Message（不拷贝消息内容）。
func toMessagePointers(messages *[]schema.Message) []*schema.Message {
	if messages == nil || len(*messages) == 0 {
		return nil
	}

	result := make([]*schema.Message, 0, len(*messages))
	for i := range *messages {
		result = append(result, &(*messages)[i])
	}

	return result
}
