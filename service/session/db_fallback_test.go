package session

import (
	"simple_ai/common/code"
	"simple_ai/model"
	"testing"
	"time"
)

func TestGetUserSessionsByUserNameUsesSessionStore(t *testing.T) {
	restore := replaceSessionStoreForTest(sessionStoreFuncs{
		listByUserName: func(userName string) ([]model.Session, error) {
			if userName != "alice" {
				t.Fatalf("userName = %q, want alice", userName)
			}
			return []model.Session{
				{ID: "s2", Title: "second", CreatedAt: time.Unix(2, 0)},
				{ID: "s1", Title: "first", CreatedAt: time.Unix(1, 0)},
			}, nil
		},
	})
	defer restore()

	sessions, err := GetUserSessionsByUserName("alice")
	if err != nil {
		t.Fatalf("GetUserSessionsByUserName returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("session count = %d, want 2", len(sessions))
	}
	if sessions[0].SessionID != "s2" || sessions[0].Title != "second" {
		t.Fatalf("first session = %#v", sessions[0])
	}
}

func TestGetChatHistoryFallsBackToMessageStoreWhenHelperMissing(t *testing.T) {
	restore := replaceSessionStoreForTest(sessionStoreFuncs{
		messagesBySession: func(userName, sessionID string) ([]model.Message, error) {
			if userName != "alice" || sessionID != "s1" {
				t.Fatalf("userName/sessionID = %q/%q, want alice/s1", userName, sessionID)
			}
			return []model.Message{
				{Content: "hi", IsUser: true},
				{Content: "hello", IsUser: false},
			}, nil
		},
	})
	defer restore()

	history, got := GetChatHistory("alice", "s1")
	if got != code.CodeSuccess {
		t.Fatalf("code = %v, want %v", got, code.CodeSuccess)
	}
	if len(history) != 2 || !history[0].IsUser || history[1].Content != "hello" {
		t.Fatalf("history = %#v", history)
	}
}

func TestChatSendRejectsSessionsOwnedByAnotherUserBeforeCreatingHelper(t *testing.T) {
	restore := replaceSessionStoreForTest(sessionStoreFuncs{
		sessionBelongsToUser: func(userName, sessionID string) (bool, error) {
			if userName != "alice" || sessionID != "s1" {
				t.Fatalf("userName/sessionID = %q/%q, want alice/s1", userName, sessionID)
			}
			return false, nil
		},
	})
	defer restore()

	_, got := ChatSend("alice", "s1", "hello", "qwen", ChatFeatureOptions{})
	if got == code.CodeSuccess {
		t.Fatal("ChatSend returned success for a session owned by another user")
	}
}
