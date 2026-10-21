package simulations

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/modules/auth"
)

// RunSimulation starts a ready simulation. The run executes asynchronously on a
// detached context; the response carries the (now running) simulation body.
//
// POST /:id/run → 202 SimulationResponse.
func (a *API) RunSimulation(c *echo.Context) error {
	var req RunRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.runSvc.Start(c.Request().Context(), c.Param("id"), llmTargetFromDTO(req.LLMExecutionTarget))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

// ControlSimulation signals a live run (pause / resume / stop, optional speed).
//
// POST /:id/control → 200.
func (a *API) ControlSimulation(c *echo.Context) error {
	var req ControlRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	if err := a.runSvc.Control(c.Request().Context(), c.Param("id"), req.Action, req.SpeedMs); err != nil {
		return a.mapError(err)
	}
	return c.NoContent(http.StatusOK)
}

// IssueStreamTicket mints a single-use, short-lived connect ticket the client
// presents on the WebSocket upgrade (so no bearer token rides in the URL).
//
// POST /:id/stream-ticket → 200 StreamTicketResponse.
func (a *API) IssueStreamTicket(c *echo.Context) error {
	ticket, expiresAt, err := a.runSvc.IssueTicket(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, StreamTicketResponse{Ticket: ticket, ExpiresAt: expiresAt})
}

// ListSteps returns a page of persisted steps for replay / scrubbing. An
// omitted ?to resolves service-side to the last recorded step (not 0), so the
// full recorded range comes back by default; an explicit ?to (any value) is
// passed through unchanged.
//
// GET /:id/steps?from&to&page&page_size → 200 StepsPageResponse.
func (a *API) ListSteps(c *echo.Context) error {
	from := parseUint64(c.QueryParam("from"), 0)
	var to *uint64
	if raw := c.QueryParam("to"); raw != "" {
		v := parseUint64(raw, 0)
		to = &v
	}
	page := parseUint64(c.QueryParam("page"), 1)
	pageSize := parseUint64(c.QueryParam("page_size"), uint64(defaultPageSize))

	steps, hasMore, err := a.runSvc.GetSteps(c.Request().Context(), c.Param("id"), from, to, page, pageSize)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, StepsPageResponse{
		Items:   stepsToDTOs(steps),
		HasMore: hasMore,
	})
}

// ExportSteps streams the persisted step history as CSV or JSONL (U3): fixed
// columns (step,type,uid,reference_id,x,y,state,decision) + the sorted union of
// the model snapshot's declared property UIDs + a statistics JSON column (env
// rows). Any status is exportable — a running simulation yields its persisted
// prefix. Errors after the first byte cannot change the HTTP status; they are
// logged and the stream is aborted.
//
// GET /:id/steps/export?format=csv|jsonl&from=&to= → 200 attachment stream.
func (a *API) ExportSteps(c *echo.Context) error {
	format := c.QueryParam("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "jsonl" {
		return a.mapError(&ValidationError{Result: ValidationResult{
			Errors: []ValidationIssue{{Path: "format", Reason: "must be csv or jsonl"}},
		}})
	}
	from := parseUint64(c.QueryParam("from"), 0)
	var to *uint64
	if raw := c.QueryParam("to"); raw != "" {
		v := parseUint64(raw, 0)
		to = &v
	}

	exp, err := a.runSvc.ExportSteps(c.Request().Context(), c.Param("id"), format, from, to)
	if err != nil {
		return a.mapError(err)
	}

	c.Response().Header().Set(echo.HeaderContentType, exp.ContentType())
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+exp.Filename()+`"`)
	c.Response().WriteHeader(http.StatusOK)
	if err := exp.WriteTo(c.Request().Context(), c.Response()); err != nil {
		a.logger.Warn("step export aborted mid-stream",
			zap.String("simulation_id", c.Param("id")), zap.Error(err))
	}
	return nil
}

// GetAgent returns one agent's CURRENT state (position/properties/state/memory):
// the live runtime snapshot when a run is active (running/paused), else the
// persisted record. Powers the live agent-inspector refresh in the run view.
//
// GET /:id/agents/:agentId → 200 SimulationAgentDTO.
func (a *API) GetAgent(c *echo.Context) error {
	agentID, err := strconv.ParseUint(c.Param("agentId"), 10, 64)
	if err != nil {
		return a.mapError(ErrAgentNotFound)
	}
	dto, err := a.runSvc.GetAgent(c.Request().Context(), c.Param("id"), agentID)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, dto)
}

// StreamSimulation upgrades the request to a WebSocket and joins the simulation's
// stream room. This route is NOT covered by the header bearer-auth middleware
// (browsers cannot send an Authorization header on a WS handshake); it
// authenticates entirely via the single-use connect ticket:
//
//  1. read the ?ticket query param;
//  2. consume the ticket for this :id (single-use, TTL-bound, sim-bound);
//  3. re-check ownership + streamable status (defense in depth) under the ticket's
//     principal — a run that finished between issue and connect has no room, so the
//     join must not lazily recreate one;
//  4. verify the request Origin against the configured allowlist;
//  5. upgrade and hand the connection to the stream hub (which joins the existing
//     run room or closes the socket if none is live).
//
// On any failure before the upgrade it returns a JSON APIError (no connection is
// leaked); after a successful upgrade the handler owns the socket and returns nil.
//
// GET /:id/ws?ticket=…
func (a *API) StreamSimulation(c *echo.Context) error {
	simID := c.Param("id")

	ticket := c.QueryParam("ticket")
	if ticket == "" {
		return a.mapError(ErrInvalidTicket)
	}

	userID, ok := a.runSvc.ConsumeTicket(ticket, simID)
	if !ok {
		return a.mapError(ErrInvalidTicket)
	}

	// Re-run ownership under the ticket's principal: the ticket already proved
	// authorization at issue time, but re-checking closes the gap between issue
	// and connect (e.g. a deleted/transferred simulation).
	ctx := withTicketPrincipal(c.Request().Context(), userID)
	if err := a.runSvc.AuthorizeStream(ctx, simID); err != nil {
		return a.mapError(err)
	}

	conn, err := a.streamUpgrader().Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		// Upgrade already wrote a response on failure; do not leak a half-open
		// connection. Log and return nil (the handshake is finished either way).
		a.logger.Debug("websocket upgrade failed", zap.String("simulation_id", simID), zap.Error(err))
		return nil
	}

	// Join attaches to the live run's room (opened by the RunManager at run start).
	// AuthorizeStream already gated on a streamable status, but the run can finish
	// in the window between that re-check and the upgrade; in that race the room is
	// gone and Join closes the socket instead of orphaning a fresh one.
	if !a.stream.Join(simID, conn) {
		a.logger.Debug("websocket join found no live run room; connection closed",
			zap.String("simulation_id", simID))
		return nil
	}
	// Ask the run loop for a full-state keyframe so the fresh client syncs
	// immediately — essential for PAUSED runs, where no step frames flow.
	a.runSvc.RequestKeyframe(simID)
	return nil
}

// streamUpgrader builds the gorilla upgrader with an Origin check bound to the
// configured allowlist. WSAllowedOrigin == "*" allows any origin (dev-only); an
// empty origin (non-browser clients) is allowed.
func (a *API) streamUpgrader() *websocket.Upgrader {
	allowed := a.cfg.WSAllowedOrigin
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024 * 10,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			if allowed == "" || allowed == "*" {
				return true
			}
			return origin == allowed
		},
	}
}

// withTicketPrincipal attaches a principal reconstructed from a consumed connect
// ticket to ctx so the WS-upgrade ownership re-check (AuthorizeStream) has an
// identity to authorize. The ticket only carries the issuing user's ID (not the
// admin flag), so the re-check authorizes by ownership: a non-admin owner passes;
// an admin who minted a ticket for a FOREIGN simulation would fail this secondary
// re-check (the ticket itself — minted only after a successful owner-or-admin
// check at issue time, single-use, TTL-bound — is the primary authorization).
func withTicketPrincipal(ctx context.Context, userID string) context.Context {
	return auth.WithPrincipal(ctx, &auth.UserPrincipal{ID: userID})
}

// parseUint64 parses s as a uint64, returning def on empty/invalid/negative input.
func parseUint64(s string, def uint64) uint64 {
	if s == "" {
		return def
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return def
	}
	return n
}
