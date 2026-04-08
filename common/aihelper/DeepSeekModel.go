package aihelper

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
)

const defaultDeepSeekBaseURL = "https://api.deepseek.com"

// DeepSeekAIModel 是基于 eino-ext deepseek.ChatModel 的 AIModel 适配实现。
type DeepSeekAIModel struct {
	chatModel *deepseek.ChatModel
}

// NewDeepSeekAIModel 使用完整的 deepseek 配置创建模型实例。
func NewDeepSeekAIModel(ctx context.Context, config *deepseek.ChatModelConfig) (*DeepSeekAIModel, error) {
	chatModel, err := deepseek.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}

	return &DeepSeekAIModel{
		chatModel: chatModel,
	}, nil
}

// NewDefaultDeepSeekAIModel 使用 DeepSeek 官方地址创建模型实例。
func NewDefaultDeepSeekAIModel(ctx context.Context, apiKey, modelName string) (*DeepSeekAIModel, error) {
	return NewDeepSeekAIModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: defaultDeepSeekBaseURL,
	})
}

// GenerateResponse 执行非流式对话并返回文本结果。
func (d *DeepSeekAIModel) GenerateResponse(ctx context.Context, messages *[]schema.Message) string {
	if d == nil || d.chatModel == nil {
		return ""
	}

	resp, err := d.chatModel.Generate(ctx, toMessagePointers(messages))
	if err != nil || resp == nil {
		return ""
	}

	return resp.Content
}

// StreamResponse 执行流式对话；每个分片通过回调返回，最终返回合并后的完整文本。
func (d *DeepSeekAIModel) StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error) {
	if d == nil || d.chatModel == nil {
		return "", errors.New("deepseek model is not initialized")
	}

	sr, err := d.chatModel.Stream(ctx, toMessagePointers(messages))
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

func (d *DeepSeekAIModel) GetModelType() string {
	return "DeepSeek"
}
