package user

import (
	"context"
	"errors"
	"simple_ai/common/mysql"
	"simple_ai/model"
	"simple_ai/utils"
	"strings"

	"gorm.io/gorm"
)

const (
	CodeMsg     = "SimpleAI验证码如下(验证码仅限于2分钟有效): "
	UserNameMsg = "SimpleAI的账号如下，请保留好，后续可以用账号进行登录 "
)

var ctx = context.Background()

func IsExistUser(account string) (bool, *model.User) {
	account = strings.TrimSpace(account)
	if account == "" {
		return false, nil
	}

	user := new(model.User)
	err := mysql.DB.Where("username = ? OR email = ?", account, account).First(user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) || user == nil {
		return false, nil
	}

	return true, user
}

func Register(username, email, password string) (*model.User, bool) {
	if user, err := mysql.InsertUser(&model.User{
		Email:    email,
		Name:     username,
		Username: username,
		Password: utils.MD5(password),
	}); err != nil {
		return nil, false
	} else {
		return user, true
	}
}

func DeleteUserByID(id int64) error {
	return mysql.DB.Unscoped().Delete(&model.User{}, id).Error
}
