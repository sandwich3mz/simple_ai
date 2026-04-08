package aihelper

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

type StreamCallBack func(content string)
type StreamCallback = StreamCallBack

type AIModel interface {
	GenerateResponse(ctx context.Context, messages *[]schema.Message) string
	StreamResponse(ctx context.Context, messages *[]schema.Message, cb StreamCallBack) (string, error)
	GetModelType() string
}
