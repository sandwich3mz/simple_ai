package rabbitmq

import (
	"encoding/json"
	"fmt"
	"simple_ai/dao/message"
	"simple_ai/model"

	"github.com/streadway/amqp"
)

type MessageMQParam struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
	UserName  string `json:"user_name"`
	IsUser    bool   `json:"is_user"`
}

func GenerateMessageMQParam(sessionID string, content string, userName string, IsUser bool) []byte {
	param := MessageMQParam{
		SessionID: sessionID,
		Content:   content,
		UserName:  userName,
		IsUser:    IsUser,
	}
	data, _ := json.Marshal(param)
	return data
}

type consumedDelivery interface {
	Body() []byte
	Ack(multiple bool) error
	Nack(multiple, requeue bool) error
}

// amqpConsumedDelivery 适配 amqp.Delivery，方便消费确认逻辑用单元测试覆盖。
type amqpConsumedDelivery struct {
	delivery *amqp.Delivery
}

func (d amqpConsumedDelivery) Body() []byte {
	return d.delivery.Body
}

func (d amqpConsumedDelivery) Ack(multiple bool) error {
	return d.delivery.Ack(multiple)
}

func (d amqpConsumedDelivery) Nack(multiple, requeue bool) error {
	return d.delivery.Nack(multiple, requeue)
}

func MQMessage(param MessageMQParam) error {
	newMsg := &model.Message{
		SessionID: param.SessionID,
		Content:   param.Content,
		UserName:  param.UserName,
		IsUser:    param.IsUser,
	}
	_, err := message.CreateMessage(newMsg)
	return err
}

// handleConsumedDelivery 统一处理消息确认：成功 Ack，可恢复失败重新入队，坏 JSON 直接丢弃。
func handleConsumedDelivery(delivery consumedDelivery, handle func(param MessageMQParam) error) error {
	var param MessageMQParam
	if err := json.Unmarshal(delivery.Body(), &param); err != nil {
		// 无法解析的消息重试也不会成功，避免反复进入队列造成死循环。
		_ = delivery.Nack(false, false)
		return fmt.Errorf("invalid message json: %w", err)
	}

	if err := handle(param); err != nil {
		// 数据库等临时故障保留消息，等待后续消费者再次处理。
		_ = delivery.Nack(false, true)
		return err
	}

	if err := delivery.Ack(false); err != nil {
		return fmt.Errorf("ack message failed: %w", err)
	}
	return nil
}
