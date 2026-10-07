package provider

import (
	"context"

	"github.com/emorydu/air-harness/ch09/internal/api"
)

type Provider interface {
	Send(ctx context.Context, messages []api.Message, tools []api.ToolDef) (api.Response, error)
	Model() string
	SetModel(name string)
}
