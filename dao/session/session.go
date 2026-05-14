package session

import (
	"simple_ai/common/mysql"
	"simple_ai/model"
)

func GetSessionsByUserName(userName string) ([]model.Session, error) {
	var sessions []model.Session
	// 会话列表以数据库为准，避免进程重启后内存 helper 丢失导致旧会话不可见。
	err := mysql.DB.Where("user_name = ?", userName).Order("created_at desc").Find(&sessions).Error
	return sessions, err
}

func CreateSession(session *model.Session) (*model.Session, error) {
	err := mysql.DB.Create(session).Error
	return session, err
}

func GetSessionByID(sessionID string) (*model.Session, error) {
	var session model.Session
	err := mysql.DB.Where("id = ?", sessionID).First(&session).Error
	return &session, err
}

func GetSessionByIDAndUserName(sessionID string, userName string) (*model.Session, error) {
	var session model.Session
	// 同时限定 sessionID 和 userName，用于校验当前用户是否拥有该会话。
	err := mysql.DB.Where("id = ? AND user_name = ?", sessionID, userName).First(&session).Error
	return &session, err
}
