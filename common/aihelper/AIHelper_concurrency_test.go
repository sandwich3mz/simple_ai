package aihelper

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

type concurrentTrackingModel struct {
	mu        sync.Mutex
	active    int
	maxActive int
}

func (m *concurrentTrackingModel) GenerateResponse(ctx context.Context, messages *[]schema.Message) string {
	m.mu.Lock()
	m.active++
	if m.active > m.maxActive {
		m.maxActive = m.active
	}
	m.mu.Unlock()

	time.Sleep(20 * time.Millisecond)

	m.mu.Lock()
	m.active--
	m.mu.Unlock()

	return "ok"
}

func (m *concurrentTrackingModel) StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error) {
	return m.GenerateResponse(ctx, messages), nil
}

func (m *concurrentTrackingModel) GetModelType() string {
	return "tracking"
}

func TestAIHelperSerializesConcurrentGenerateResponseForSameSession(t *testing.T) {
	model := &concurrentTrackingModel{}
	helper := NewAIHelper(model, "session-1")
	helper.SetSaveFunc(nil)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := helper.GenerateResponse("alice", context.Background(), "hello"); err != nil {
				t.Errorf("GenerateResponse returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	if model.maxActive != 1 {
		t.Fatalf("max concurrent model calls = %d, want 1", model.maxActive)
	}
}

func TestAIHelperConcurrentAddMessageKeepsAllMessages(t *testing.T) {
	helper := NewAIHelper(fakeModel{}, "session-1")
	helper.SetSaveFunc(nil)

	const count = 200
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			helper.AddMessage("hello", "alice", true, false)
		}()
	}
	wg.Wait()

	if got := len(helper.GetMessages()); got != count {
		t.Fatalf("message count = %d, want %d", got, count)
	}
}
