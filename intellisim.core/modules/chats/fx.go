package chats

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"chats",
	fx.Provide(
		NewModuleConfig,
		NewEventsHub,
		NewChatMapper,
		NewChatService,
		NewChatAPI,
	),
)
