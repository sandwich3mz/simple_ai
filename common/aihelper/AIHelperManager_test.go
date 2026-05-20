package aihelper

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

type modelWithType struct {
	modelType string
}

func (m modelWithType) GenerateResponse(ctx context.Context, messages *[]schema.Message) string {
	return "ok"
}

func (m modelWithType) StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error) {
	return "ok", nil
}

func (m modelWithType) GetModelType() string {
	return m.modelType
}

func TestGetOrCreateAIHelperReplacesModelWhenProviderChanges(t *testing.T) {
	manager := newAIHelperManager()
	factory := GetGlobalAIModelFactory()

	const providerA AIModelProvider = "switch-provider-a"
	const providerB AIModelProvider = "switch-provider-b"
	factory.RegisterProvider(providerA, func(ctx context.Context, cfg *AIModelFactoryConfig) (AIModel, error) {
		return modelWithType{modelType: string(providerA)}, nil
	})
	factory.RegisterProvider(providerB, func(ctx context.Context, cfg *AIModelFactoryConfig) (AIModel, error) {
		return modelWithType{modelType: string(providerB)}, nil
	})

	helper, err := manager.GetOrCreateAIHelper("alice", "session-1", &AIModelFactoryConfig{Provider: providerA})
	if err != nil {
		t.Fatalf("create helper with provider A: %v", err)
	}
	if got := helper.GetModelType(); got != string(providerA) {
		t.Fatalf("initial model type = %q, want %q", got, providerA)
	}

	helper, err = manager.GetOrCreateAIHelper("alice", "session-1", &AIModelFactoryConfig{Provider: providerB})
	if err != nil {
		t.Fatalf("switch helper to provider B: %v", err)
	}
	if got := helper.GetModelType(); got != string(providerB) {
		t.Fatalf("model type after switch = %q, want %q", got, providerB)
	}
}
