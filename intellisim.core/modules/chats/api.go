package chats

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/modules/auth"
	"intelli.sim.core/pkg/httpapi"
)

const sseHeartbeatInterval = 25 * time.Second

type ChatAPI struct {
	logger      *zap.Logger
	chatService ChatService
	hub         *EventsHub
}

func NewChatAPI(logger *zap.Logger, service ChatService, hub *EventsHub) *ChatAPI {
	return &ChatAPI{logger: logger.Named("chat-api"), chatService: service, hub: hub}
}

func (a *ChatAPI) SetupRoutes(group *echo.Group) {
	group.POST("", a.CreateChat)
	group.GET("", a.ListChats)
	group.GET("/:id", a.GetChat)
	group.PUT("/:id", a.UpdateChat)
	group.DELETE("/:id", a.DeleteChat)
	group.GET("/:id/messages", a.ListMessages)
	group.POST("/:id/messages", a.SendMessage)
	group.GET("/:id/events", a.StreamEvents)
}

// mapError translates service sentinels into the shared APIError envelope.
func (a *ChatAPI) mapError(err error) error {
	switch {
	case errors.Is(err, ErrChatNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "not_found", "chat not found")
	case errors.Is(err, ErrModelUnavailable):
		return httpapi.NewAPIError(http.StatusConflict, "model_unavailable",
			"the chat's model is not available right now — select another model to continue")
	case errors.Is(err, ErrEffortNotAllowed):
		return httpapi.NewAPIError(http.StatusConflict, "effort_not_allowed",
			"the requested effort is not allowed for this model")
	case errors.Is(err, ErrGenerationInProgress):
		return httpapi.NewAPIError(http.StatusConflict, "generation_in_progress",
			"a generation is already in progress for this chat")
	case errors.Is(err, ErrInvalidCursor):
		return httpapi.NewAPIError(http.StatusBadRequest, "invalid_cursor",
			"cursor message does not belong to this chat")
	case errors.Is(err, ErrNoChanges):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "validation_failed",
			"update request has no changes")
	case errors.Is(err, auth.ErrNoPrincipal):
		return httpapi.NewAPIError(http.StatusUnauthorized, "unauthorized", "no principal")
	case errors.Is(err, auth.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, "forbidden",
			"you do not have access to this chat")
	default:
		a.logger.Error("chat request failed", zap.Error(err))
		return httpapi.NewAPIError(http.StatusInternalServerError, "internal", "internal server error")
	}
}

func bindAndValidate(c *echo.Context, target any) error {
	if err := c.Bind(target); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	return c.Validate(target) // returns *APIError (validation_failed) on failure
}

func (a *ChatAPI) CreateChat(c *echo.Context) error {
	var req CreateChatRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	chat, err := a.chatService.CreateChat(c.Request().Context(), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusCreated, chat)
}

func (a *ChatAPI) ListChats(c *echo.Context) error {
	var q ListChatsQuery
	if err := bindAndValidate(c, &q); err != nil {
		return err
	}
	resp, err := a.chatService.ListChats(c.Request().Context(), q)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *ChatAPI) GetChat(c *echo.Context) error {
	chat, err := a.chatService.GetChat(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, chat)
}

func (a *ChatAPI) UpdateChat(c *echo.Context) error {
	var req UpdateChatRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	chat, err := a.chatService.UpdateChat(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, chat)
}

func (a *ChatAPI) DeleteChat(c *echo.Context) error {
	if err := a.chatService.DeleteChat(c.Request().Context(), c.Param("id")); err != nil {
		return a.mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *ChatAPI) ListMessages(c *echo.Context) error {
	var q ListMessagesQuery
	if err := bindAndValidate(c, &q); err != nil {
		return err
	}
	page, err := a.chatService.ListMessages(c.Request().Context(), c.Param("id"), q)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, page)
}

func (a *ChatAPI) SendMessage(c *echo.Context) error {
	var req SendMessageRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.chatService.SendMessage(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

// StreamEvents is the per-chat SSE stream (auth + ownership enforced).
// Protocol: named events (chat.status / message.created / chat.updated /
// chat.deleted / chat.error) with JSON data; ": ping" heartbeat every 25s.
// No replay — on reconnect clients refetch the chat + latest messages page.
func (a *ChatAPI) StreamEvents(c *echo.Context) error {
	chatID := c.Param("id")
	// Ownership gate (also yields 404 for missing/foreign chats).
	if _, err := a.chatService.GetChat(c.Request().Context(), chatID); err != nil {
		return a.mapError(err)
	}

	sub, cancel := a.hub.Subscribe(chatID)
	defer cancel()

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")
	resp.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(resp)
	if err := rc.Flush(); err != nil {
		return nil // client cannot stream; bail quietly
	}

	writeEvent := func(ev Event) error {
		data, err := json.Marshal(ev.Payload)
		if err != nil {
			a.logger.Error("failed to marshal SSE payload", zap.Error(err))
			return nil
		}
		if _, err := fmt.Fprintf(resp, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
			return err
		}
		return rc.Flush()
	}

	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request().Context().Done():
			return nil
		case ev, ok := <-sub:
			if !ok {
				return nil // hub disconnected us (overflow or chat deleted)
			}
			if err := writeEvent(ev); err != nil {
				return nil
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(resp, ": ping\n\n"); err != nil {
				return nil
			}
			if err := rc.Flush(); err != nil {
				return nil
			}
		}
	}
}
