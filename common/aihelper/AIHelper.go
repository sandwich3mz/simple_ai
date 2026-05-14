package aihelper

import (
	"context"
	"fmt"
	"log"
	"simple_ai/common/rabbitmq"
	"simple_ai/model"
	"simple_ai/utils"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// AIHelper 管理单个聊天会话的内存上下文。
type AIHelper struct {
	aiModel  AIModel
	messages []*model.Message
	mutex    sync.RWMutex
	// generationMutex 保证同一会话内的模型生成串行执行，避免并发请求打乱上下文顺序。
	generationMutex sync.Mutex
	SessionID       string
	saveFunc        func(*model.Message) (*model.Message, error)
	enhancers       []MessageEnhancer
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
			// 默认通过 RabbitMQ 异步持久化消息；MQ 未初始化时返回错误但不阻断聊天流程。
			if rabbitmq.RMQMessage == nil {
				return msg, fmt.Errorf("rabbitmq message publisher is not initialized")
			}
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
	a.mutex.Lock()
	a.messages = append(a.messages, &userMsg)
	saveFunc := a.saveFunc
	a.mutex.Unlock()

	// 持久化失败只记录日志，避免 MQ 短暂故障导致用户对话请求直接失败。
	if save && saveFunc != nil {
		if _, err := saveFunc(&userMsg); err != nil {
			log.Printf("failed to persist chat message: %v", err)
		}
	}
}

// SetSaveFunc 替换消息保存时使用的持久化回调。
func (a *AIHelper) SetSaveFunc(saveFunc func(*model.Message) (*model.Message, error)) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.saveFunc = saveFunc
}

// SetMessageEnhancers 替换模型生成前执行的增强器链。
func (a *AIHelper) SetMessageEnhancers(enhancers ...MessageEnhancer) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.enhancers = a.enhancers[:0]
	for _, enhancer := range enhancers {
		if enhancer != nil {
			a.enhancers = append(a.enhancers, enhancer)
		}
	}
}

// LoadMessages 用持久化存储中恢复出的消息替换当前内存历史。
func (a *AIHelper) LoadMessages(messages []model.Message) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.messages = a.messages[:0]
	for i := range messages {
		msg := messages[i]
		a.messages = append(a.messages, &msg)
	}
}

// GetMessages 返回当前内存消息历史的快照。
func (a *AIHelper) GetMessages() []*model.Message {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	out := make([]*model.Message, len(a.messages))
	for i, msg := range a.messages {
		if msg == nil {
			continue
		}
		// 返回深拷贝快照，避免调用方修改内部消息指针。
		msgCopy := *msg
		out[i] = &msgCopy
	}
	return out
}

// GenerateResponse 追加用户消息，执行增强器，并返回非流式模型响应。
func (a *AIHelper) GenerateResponse(userName string, ctx context.Context, userQuestion string) (*model.Message, error) {
	a.generationMutex.Lock()
	defer a.generationMutex.Unlock()

	a.AddMessage(userQuestion, userName, true, true)

	messages := utils.ConvertToSchemaMessages(a.GetMessages())

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
	a.generationMutex.Lock()
	defer a.generationMutex.Unlock()

	a.AddMessage(userQuestion, userName, true, true)

	messages := utils.ConvertToSchemaMessages(a.GetMessages())

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
	a.mutex.RLock()
	// 复制增强器切片后再执行，避免增强过程中长时间持有读锁。
	enhancers := append([]MessageEnhancer(nil), a.enhancers...)
	a.mutex.RUnlock()

	for _, enhancer := range enhancers {
		next, err := enhancer.EnhanceMessages(ctx, enhanced)
		if err != nil {
			return nil, err
		}
		enhanced = next
	}
	return enhanced, nil
}
