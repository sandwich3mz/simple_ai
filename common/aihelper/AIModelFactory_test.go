package aihelper

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

type fakeModel struct{}

func (fakeModel) GenerateResponse(ctx context.Context, messages *[]schema.Message) string {
	return "ok"
}

func (fakeModel) StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error) {
	if cb != nil {
		cb("ok")
	}
	return "ok", nil
}

func (fakeModel) GetModelType() string {
	return "fake"
}

func TestAIModelFactorySupportsRegisteredProviders(t *testing.T) {
	factory := newAIModelFactory()
	const provider AIModelProvider = "fake"

	factory.RegisterProvider(provider, func(ctx context.Context, cfg *AIModelFactoryConfig) (AIModel, error) {
		if cfg.Provider != provider {
			t.Fatalf("provider = %q, want %q", cfg.Provider, provider)
		}
		return fakeModel{}, nil
	})

	model, err := factory.Create(context.Background(), &AIModelFactoryConfig{Provider: provider})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got := model.GetModelType(); got != "fake" {
		t.Fatalf("GetModelType() = %q, want fake", got)
	}
}
