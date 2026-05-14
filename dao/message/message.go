package message

import (
	"simple_ai/common/mysql"
	"simple_ai/model"
)

func CreateMessage(message *model.Message) (*model.Message, error) {
	err := mysql.DB.Create(message).Error
	return message, err
}

func GetAllMessages() ([]model.Message, error) {
	var msgs []model.Message
	err := mysql.DB.Order("created_at asc").Find(&msgs).Error
	return msgs, err
}

func GetMessagesByUserNameAndSessionID(userName string, sessionID string) ([]model.Message, error) {
	var msgs []model.Message
	// 按创建时间和自增 ID 排序，保证从数据库恢复上下文时消息顺序稳定。
	err := mysql.DB.
		Where("user_name = ? AND session_id = ?", userName, sessionID).
		Order("created_at asc, id asc").
		Find(&msgs).Error
	return msgs, err
}
