package user

import (
	"errors"
	"simple_ai/common/code"
	"simple_ai/model"
	"testing"
)

func TestRegisterRollsBackCreatedUserWhenAccountEmailFails(t *testing.T) {
	originalIsExistUser := isExistUser
	originalCheckCaptcha := checkCaptchaForEmail
	originalRegisterUser := registerUser
	originalSendEmail := sendCaptchaEmail
	originalDeleteUser := deleteRegisteredUser
	defer func() {
		isExistUser = originalIsExistUser
		checkCaptchaForEmail = originalCheckCaptcha
		registerUser = originalRegisterUser
		sendCaptchaEmail = originalSendEmail
		deleteRegisteredUser = originalDeleteUser
	}()

	isExistUser = func(account string) (bool, *model.User) {
		return false, nil
	}
	checkCaptchaForEmail = func(email, captcha string) (bool, error) {
		return true, nil
	}
	registerUser = func(username, email, password string) (*model.User, bool) {
		return &model.User{ID: 42, Username: username, Email: email}, true
	}
	sendCaptchaEmail = func(email, content, msg string) error {
		return errors.New("smtp failed")
	}

	var rolledBackID int64
	deleteRegisteredUser = func(id int64) error {
		rolledBackID = id
		return nil
	}

	_, got := Register("alice@example.com", "password", "123456")
	if got != code.CodeServerBusy {
		t.Fatalf("Register code = %v, want %v", got, code.CodeServerBusy)
	}
	if rolledBackID != 42 {
		t.Fatalf("rolledBackID = %d, want 42", rolledBackID)
	}
}
