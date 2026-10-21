package simulations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/pkg/httpapi"
)

// fakeRunSvc is a programmable RunService for exercising the run API handlers in
// isolation (no live engine / repo). Each field records the last call and lets a
// test inject a canned result or error.
type fakeRunSvc struct {
	startResp     *SimulationResponse
	startErr      error
	startID       string
	startOverride *common.LLMTarget

	controlErr     error
	controlID      string
	controlAction  string
	controlSpeedMs *int

	ticket    string
	ticketExp time.Time
	ticketErr error
	ticketID  string

	consumeUID string
	consumeOK  bool

	stepsItems    []datasims.SimulationStep
	stepsMore     bool
	stepsErr      error
	stepsArgs     [5]uint64 // from,to,page,pageSize unused index padding
	stepsToWasNil bool      // true iff GetSteps was called with a nil "to" (omitted query param)

	authStreamErr error
	authStreamID  string

	agentDTO        *SimulationAgentDTO
	agentErr        error
	agentID         string
	agentInstanceID uint64
	keyframeID      string
}

func (f *fakeRunSvc) Start(_ context.Context, id string, execOverride *common.LLMTarget) (*SimulationResponse, error) {
	f.startID = id
	f.startOverride = execOverride
	if f.startErr != nil {
		return nil, f.startErr
	}
	if f.startResp != nil {
		return f.startResp, nil
	}
	return &SimulationResponse{ID: id, Status: string(datasims.SimulationStatusRunning)}, nil
}

func (f *fakeRunSvc) Control(_ context.Context, id, action string, speedMs *int) error {
	f.controlID = id
	f.controlAction = action
	f.controlSpeedMs = speedMs
	return f.controlErr
}

func (f *fakeRunSvc) IssueTicket(_ context.Context, id string) (string, time.Time, error) {
	f.ticketID = id
	if f.ticketErr != nil {
		return "", time.Time{}, f.ticketErr
	}
	return f.ticket, f.ticketExp, nil
}

func (f *fakeRunSvc) ConsumeTicket(_ string, _ string) (string, bool) {
	return f.consumeUID, f.consumeOK
}

func (f *fakeRunSvc) GetSteps(_ context.Context, _ string, from uint64, to *uint64, page, pageSize uint64) ([]datasims.SimulationStep, bool, error) {
	f.stepsToWasNil = to == nil
	toVal := uint64(0)
	if to != nil {
		toVal = *to
	}
	f.stepsArgs = [5]uint64{from, toVal, page, pageSize, 0}
	return f.stepsItems, f.stepsMore, f.stepsErr
}

func (f *fakeRunSvc) ExportSteps(_ context.Context, _, format string, _ uint64, _ *uint64) (*stepExporter, error) {
	return &stepExporter{format: format}, nil
}

func (f *fakeRunSvc) AuthorizeStream(_ context.Context, id string) error {
	f.authStreamID = id
	return f.authStreamErr
}

func (f *fakeRunSvc) RequestKeyframe(id string) {
	f.keyframeID = id
}

func (f *fakeRunSvc) GetAgent(_ context.Context, id string, agentID uint64) (*SimulationAgentDTO, error) {
	f.agentID = id
	f.agentInstanceID = agentID
	if f.agentErr != nil {
		return nil, f.agentErr
	}
	return f.agentDTO, nil
}

// newRunAPI builds an API wired with a fake RunService + a real (unused) stream hub.
func newRunAPI(svc *fakeRunSvc) *API {
	cfg := RunConfig{WSAllowedOrigin: "http://localhost:3000", StreamTicketTTLSec: 30}
	hub := NewStreamHub(cfg, zap.NewNop())
	return NewAPI(zap.NewNop(), nil, NewEventsHub(), svc, hub, cfg, nil)
}

// newReq builds an echo.Context for a request with the given method/path, a
// principal in the request context, and the :id path param set.
func newReq(t *testing.T, method, target, id, body string, principal *auth.UserPrincipal) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	e.Validator = httpapi.NewGoPlaygroundValidator()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	if principal != nil {
		r = r.WithContext(auth.WithPrincipal(r.Context(), principal))
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(r, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: id}})
	return c, rec
}

func principalAlice() *auth.UserPrincipal { return &auth.UserPrincipal{ID: "alice"} }

func TestRunAPI_RegistersFiveRoutes(t *testing.T) {
	e := echo.New()
	api := newRunAPI(&fakeRunSvc{})
	api.SetupRoutes(e.Group("/simulations"))

	want := map[string]bool{
		"POST /simulations/:id/run":           false,
		"POST /simulations/:id/control":       false,
		"POST /simulations/:id/stream-ticket": false,
		"GET /simulations/:id/steps":          false,
		"GET /simulations/:id/ws":             false,
	}
	for _, rt := range e.Router().Routes() {
		key := rt.Method + " " + rt.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("route not registered: %s", key)
		}
	}
}

func TestRunSimulation_202(t *testing.T) {
	svc := &fakeRunSvc{startResp: &SimulationResponse{ID: "sim-1", Status: "running"}}
	api := newRunAPI(svc)
	c, rec := newReq(t, http.MethodPost, "/simulations/sim-1/run", "sim-1", "{}", principalAlice())

	if err := api.RunSimulation(c); err != nil {
		t.Fatalf("RunSimulation err=%v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d want 202", rec.Code)
	}
	if svc.startID != "sim-1" {
		t.Fatalf("service Start id=%q want sim-1", svc.startID)
	}
}

func TestRunSimulation_MapsNotRunnable(t *testing.T) {
	svc := &fakeRunSvc{startErr: ErrNotRunnable}
	api := newRunAPI(svc)
	c, _ := newReq(t, http.MethodPost, "/simulations/sim-1/run", "sim-1", "{}", principalAlice())

	err := api.RunSimulation(c)
	apiErr, ok := err.(*httpapi.APIError)
	if !ok {
		t.Fatalf("err type=%T want *APIError", err)
	}
	if apiErr.Status != http.StatusUnprocessableEntity || apiErr.Code != "not_runnable" {
		t.Fatalf("apiErr=%+v want 422 not_runnable", apiErr)
	}
}

func TestControlSimulation_200(t *testing.T) {
	svc := &fakeRunSvc{}
	api := newRunAPI(svc)
	c, rec := newReq(t, http.MethodPost, "/simulations/sim-1/control", "sim-1",
		`{"action":"pause"}`, principalAlice())

	if err := api.ControlSimulation(c); err != nil {
		t.Fatalf("ControlSimulation err=%v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if svc.controlAction != "pause" {
		t.Fatalf("control action=%q want pause", svc.controlAction)
	}
}

func TestControlSimulation_RejectsBadAction(t *testing.T) {
	api := newRunAPI(&fakeRunSvc{})
	c, _ := newReq(t, http.MethodPost, "/simulations/sim-1/control", "sim-1",
		`{"action":"explode"}`, principalAlice())

	err := api.ControlSimulation(c)
	if err == nil {
		t.Fatal("expected a validation error for an illegal action")
	}
}

func TestIssueStreamTicket_200(t *testing.T) {
	exp := time.Now().Add(30 * time.Second)
	svc := &fakeRunSvc{ticket: "tok-abc", ticketExp: exp}
	api := newRunAPI(svc)
	c, rec := newReq(t, http.MethodPost, "/simulations/sim-1/stream-ticket", "sim-1", "", principalAlice())

	if err := api.IssueStreamTicket(c); err != nil {
		t.Fatalf("IssueStreamTicket err=%v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tok-abc") {
		t.Fatalf("body missing ticket: %s", rec.Body.String())
	}
}

func TestListSteps_ParsesParamsAndPaginates(t *testing.T) {
	svc := &fakeRunSvc{
		stepsItems: []datasims.SimulationStep{
			{Metadata: datasims.StepMetadata{Step: 2, Type: datasims.MetadataTypeAgent, ReferenceID: 1}},
		},
		stepsMore: true,
	}
	api := newRunAPI(svc)
	c, rec := newReq(t, http.MethodGet,
		"/simulations/sim-1/steps?from=2&to=9&page=3&page_size=25", "sim-1", "", principalAlice())

	if err := api.ListSteps(c); err != nil {
		t.Fatalf("ListSteps err=%v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if svc.stepsArgs[0] != 2 || svc.stepsArgs[1] != 9 || svc.stepsArgs[2] != 3 || svc.stepsArgs[3] != 25 {
		t.Fatalf("GetSteps args=%v want [2 9 3 25 ...]", svc.stepsArgs)
	}
	if svc.stepsToWasNil {
		t.Fatal("an explicit ?to must be forwarded, not nil")
	}
}

// TestListSteps_OmittedTo_PassesNilToService pins the handler-level half of the
// ISS-13 fix: an omitted ?to must reach the service as nil (letting it resolve
// to the last recorded step), not the zero value (which previously produced the
// [0,0] window bug — only step 0 ever returned).
func TestListSteps_OmittedTo_PassesNilToService(t *testing.T) {
	svc := &fakeRunSvc{}
	api := newRunAPI(svc)
	c, rec := newReq(t, http.MethodGet, "/simulations/sim-1/steps", "sim-1", "", principalAlice())

	if err := api.ListSteps(c); err != nil {
		t.Fatalf("ListSteps err=%v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !svc.stepsToWasNil {
		t.Fatal("an omitted ?to must be forwarded as nil so the service resolves it")
	}
}

func TestStreamSimulation_RejectsBadTicketWithoutUpgrade(t *testing.T) {
	svc := &fakeRunSvc{consumeOK: false}
	api := newRunAPI(svc)
	// No principal: the WS route is ticket-authenticated, not header-authenticated.
	c, _ := newReq(t, http.MethodGet, "/simulations/sim-1/ws?ticket=bad", "sim-1", "", nil)

	err := api.StreamSimulation(c)
	apiErr, ok := err.(*httpapi.APIError)
	if !ok {
		t.Fatalf("err type=%T want *APIError", err)
	}
	if apiErr.Status != http.StatusForbidden || apiErr.Code != "invalid_ticket" {
		t.Fatalf("apiErr=%+v want 403 invalid_ticket", apiErr)
	}
	if svc.authStreamID != "" {
		t.Fatal("AuthorizeStream must not run when the ticket is invalid")
	}
}

func TestStreamSimulation_MissingTicketRejected(t *testing.T) {
	svc := &fakeRunSvc{consumeOK: true, consumeUID: "alice"}
	api := newRunAPI(svc)
	c, _ := newReq(t, http.MethodGet, "/simulations/sim-1/ws", "sim-1", "", nil)

	err := api.StreamSimulation(c)
	apiErr, ok := err.(*httpapi.APIError)
	if !ok || apiErr.Status != http.StatusForbidden {
		t.Fatalf("err=%v want 403 invalid_ticket for a missing ticket", err)
	}
}
