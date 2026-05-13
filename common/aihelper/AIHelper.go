package aihelper

import (
	"context"
	"simple_ai/common/rabbitmq"
	"simple_ai/model"
	"simple_ai/utils"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// AIHelper 管理单个聊天会话的内存上下文。
type AIHelper struct {
	aiModel   AIModel
	messages  []*model.Message
	mutex     sync.RWMutex
	SessionID string
	saveFunc  func(*model.Message) (*model.Message, error)
	enhancers []MessageEnhancer
}

// AIHelperOption 用于在创建 AIHelper 时注入自定义配置。
type AIHelperOption func(*AIHelper)

// MessageEnhancer 在调用底座模型前增强或改写聊天消息。
type MessageEnhancer interface {
	EnhanceMessages(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error)
}

// MessageEnhancerFunc 将函数适配为 MessageEnhancer。
type MessageEnhancerFunc func(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error)

// EnhanceMessages 执行函数形式的消息增强器。
func (f MessageEnhancerFunc) EnhanceMessages(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
	return f(ctx, messages)
}

// WithMessageEnhancers 为 AIHelper 挂载一个或多个消息增强器。
func WithMessageEnhancers(enhancers ...MessageEnhancer) AIHelperOption {
	return func(helper *AIHelper) {
		helper.SetMessageEnhancers(enhancers...)
	}
}

// NewAIHelper 创建单个会话的 AIHelper，并配置默认持久化回调。
func NewAIHelper(aiModel AIModel, sessionID string, opts ...AIHelperOption) *AIHelper {
	helper := &AIHelper{
		aiModel:  aiModel,
		messages: make([]*model.Message, 0),
		saveFunc: func(msg *model.Message) (*model.Message, error) {
			data := rabbitmq.GenerateMessageMQParam(msg.SessionID, msg.Content, msg.UserName, msg.IsUser)
			err := rabbitmq.RMQMessage.Publish(data)
			return msg, err
		},
		SessionID: sessionID,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(helper)
		}
	}

	return helper
}

// AddMessage 将消息追加到内存中，并按需发布到持久化队列。
func (a *AIHelper) AddMessage(content string, userName string, isUser bool, save bool) {
	userMsg := model.Message{
		SessionID: a.SessionID,
		Content:   content,
		UserName:  userName,
		IsUser:    isUser,
	}
	a.messages = append(a.messages, &userMsg)
	if save && a.saveFunc != nil {
		_, _ = a.saveFunc(&userMsg)
	}
}

// SetSaveFunc 替换消息保存时使用的持久化回调。
func (a *AIHelper) SetSaveFunc(saveFunc func(*model.Message) (*model.Message, error)) {
	a.saveFunc = saveFunc
}

// SetMessageEnhancers 替换模型生成前执行的增强器链。
func (a *AIHelper) SetMessageEnhancers(enhancers ...MessageEnhancer) {
	a.enhancers = a.enhancers[:0]
	for _, enhancer := range enhancers {
		if enhancer != nil {
			a.enhancers = append(a.enhancers, enhancer)
		}
	}
}

// GetMessages 返回当前内存消息历史的快照。
func (a *AIHelper) GetMessages() []*model.Message {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	out := make([]*model.Message, len(a.messages))
	copy(out, a.messages)
	return out
}

// GenerateResponse 追加用户消息，执行增强器，并返回非流式模型响应。
func (a *AIHelper) GenerateResponse(userName string, ctx context.Context, userQuestion string) (*model.Message, error) {
	a.AddMessage(userQuestion, userName, true, true)

	a.mutex.RLock()
	messages := utils.ConvertToSchemaMessages(a.messages)
	a.mutex.RUnlock()

	enhancedMessages, err := a.enhanceMessages(ctx, messages)
	if err != nil {
		return nil, err
	}

	content := a.aiModel.GenerateResponse(ctx, toSchemaMessageValues(enhancedMessages))
	modelMsg := &model.Message{
		SessionID: a.SessionID,
		UserName:  userName,
		Content:   content,
		IsUser:    false,
	}

	a.AddMessage(modelMsg.Content, userName, false, true)
	return modelMsg, nil
}

// StreamResponse 追加用户消息，执行增强器，并通过回调流式返回模型响应。
func (a *AIHelper) StreamResponse(userName string, ctx context.Context, cb StreamCallback, userQuestion string) (*model.Message, error) {
	a.AddMessage(userQuestion, userName, true, true)

	a.mutex.RLock()
	messages := utils.ConvertToSchemaMessages(a.messages)
	a.mutex.RUnlock()

	enhancedMessages, err := a.enhanceMessages(ctx, messages)
	if err != nil {
		return nil, err
	}

	content, err := a.aiModel.StreamResponse(ctx, toSchemaMessageValues(enhancedMessages), cb)
	if err != nil {
		return nil, err
	}

	modelMsg := &model.Message{
		SessionID: a.SessionID,
		UserName:  userName,
		Content:   content,
		IsUser:    false,
	}

	a.AddMessage(modelMsg.Content, userName, false, true)
	return modelMsg, nil
}

// GetModelType 返回底座模型类型。
func (a *AIHelper) GetModelType() string {
	return a.aiModel.GetModelType()
}

func toSchemaMessageValues(messages []*schema.Message) *[]schema.Message {
	if len(messages) == 0 {
		empty := make([]schema.Message, 0)
		return &empty
	}

	values := make([]schema.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		values = append(values, *message)
	}

	return &values
}

func (a *AIHelper) enhanceMessages(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
	enhanced := messages
	for _, enhancer := range a.enhancers {
		next, err := enhancer.EnhanceMessages(ctx, enhanced)
		if err != nil {
			return nil, err
		}
		enhanced = next
	}
	return enhanced, nil
}
