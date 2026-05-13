package session

import "testing"

func TestResolveChatRuntimeConfigUsesBaseModelAndFeatureFlags(t *testing.T) {
	runtime, err := ResolveChatRuntimeConfig("deepseek", ChatFeatureOptions{
		EnableRAG: true,
		EnableMCP: true,
	})
	if err != nil {
		t.Fatalf("ResolveChatRuntimeConfig returned error: %v", err)
	}
	if runtime.BaseModelType != "deepseek" {
		t.Fatalf("BaseModelType = %q, want deepseek", runtime.BaseModelType)
	}
	if !runtime.EnableRAG || !runtime.EnableMCP {
		t.Fatalf("features = rag:%v mcp:%v, want both enabled", runtime.EnableRAG, runtime.EnableMCP)
	}
}

func TestResolveChatRuntimeConfigConvertsLegacyRAGAndMCPModelTypes(t *testing.T) {
	tests := []struct {
		name      string
		modelType string
		wantRAG   bool
		wantMCP   bool
	}{
		{name: "rag", modelType: "rag", wantRAG: true},
		{name: "legacy rag number", modelType: "2", wantRAG: true},
		{name: "mcp", modelType: "mcp", wantMCP: true},
		{name: "legacy mcp number", modelType: "3", wantMCP: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := ResolveChatRuntimeConfig(tt.modelType, ChatFeatureOptions{})
			if err != nil {
				t.Fatalf("ResolveChatRuntimeConfig returned error: %v", err)
			}
			if runtime.BaseModelType != "qwen" {
				t.Fatalf("BaseModelType = %q, want qwen", runtime.BaseModelType)
			}
			if runtime.EnableRAG != tt.wantRAG || runtime.EnableMCP != tt.wantMCP {
				t.Fatalf("features = rag:%v mcp:%v, want rag:%v mcp:%v",
					runtime.EnableRAG, runtime.EnableMCP, tt.wantRAG, tt.wantMCP)
			}
		})
	}
}
