package simulations

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
)

// --- fakes ----------------------------------------------------------------------

// fakeRunStarter records Start/Control calls so the run service's orchestration
// (ownership → validation → manager) can be asserted without a live engine.
type fakeRunStarter struct {
	started    []string
	startErr   error
	controls   []ctrlCall
	controlErr error
	running    map[string]bool
	liveAgents map[uint64]*datasims.SimulationAgent
	keyframes  []string
}

type ctrlCall struct {
	simID string
	msg   controlMsg
}

func (f *fakeRunStarter) Start(_ context.Context, sim *datasims.Simulation) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = append(f.started, sim.ID)
	if f.running == nil {
		f.running = map[string]bool{}
	}
	f.running[sim.ID] = true
	return nil
}

func (f *fakeRunStarter) Control(simID string, msg controlMsg) error {
	if f.controlErr != nil {
		return f.controlErr
	}
	f.controls = append(f.controls, ctrlCall{simID: simID, msg: msg})
	return nil
}

func (f *fakeRunStarter) IsRunning(simID string) bool {
	return f.running != nil && f.running[simID]
}

func (f *fakeRunStarter) RequestKeyframe(simID string) {
	f.keyframes = append(f.keyframes, simID)
}

// liveAgents (when set) answers LiveAgent for a "live" sim; nil map = no live run.
func (f *fakeRunStarter) LiveAgent(simID string, agentID uint64) (*datasims.SimulationAgent, bool) {
	if !f.IsRunning(simID) {
		return nil, false
	}
	if f.liveAgents == nil {
		return nil, false
	}
	return f.liveAgents[agentID], true
}

// fakeTickets is a programmable ticketIssuer.
type fakeTickets struct {
	issued     []string // simIDs an Issue was requested for
	lastUser   string
	token      string
	consumeOK  bool
	consumeUID string
}

func (f *fakeTickets) Issue(userID, simID string, _ time.Time) string {
	f.issued = append(f.issued, simID)
	f.lastUser = userID
	if f.token == "" {
		f.token = "tok-" + simID
	}
	return f.token
}

func (f *fakeTickets) Consume(_ string, _ string, _ time.Time) (string, bool) {
	return f.consumeUID, f.consumeOK
}

// --- harness --------------------------------------------------------------------

func newTestRunService(repo datasims.SimulationsRepository, mgr runManagerPort, tickets ticketIssuer, steps datasims.SimulationStepsRepository) *runService {
	return NewRunService(repo, mgr, tickets, steps, &fakeInvoker{}, RunConfig{StreamTicketTTLSec: 30}, zap.NewNop()).(*runService)
}

// runnableServiceSim builds a ready, owner-stamped simulation that ValidateRunnable
// accepts (rules-only so no LLM target is needed).
func runnableServiceSim(id, owner string) *datasims.Simulation {
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim := validationSim(m)
	sim.ID = id
	sim.CreatedBy = owner
	return sim
}

// --- tests ----------------------------------------------------------------------

func TestRunService_StartOwnershipAndValidation(t *testing.T) {
	// Foreign sim → ErrForbidden, manager untouched.
	t.Run("foreign forbidden", func(t *testing.T) {
		sim := runnableServiceSim("sim-1", "alice")
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})

		_, err := svc.Start(ctxAs("bob", false), "sim-1", nil)
		if err != auth.ErrForbidden {
			t.Fatalf("Start foreign err=%v want ErrForbidden", err)
		}
		if len(mgr.started) != 0 {
			t.Fatalf("manager.Start called for foreign sim: %v", mgr.started)
		}
	})

	// Owner but not ready → ErrNotRunnable, manager untouched.
	t.Run("not runnable", func(t *testing.T) {
		sim := runnableServiceSim("sim-2", "alice")
		sim.Status = datasims.SimulationStatusFinished
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})

		_, err := svc.Start(ctxAs("alice", false), "sim-2", nil)
		if err == nil || !errors.Is(err, ErrNotRunnable) {
			t.Fatalf("Start not-ready err=%v want ErrNotRunnable", err)
		}
		if len(mgr.started) != 0 {
			t.Fatalf("manager.Start called for not-runnable sim: %v", mgr.started)
		}
	})

	// Already running: ErrRunInFlight (409) must be reachable via a second
	// POST /run, not the 422 ErrNotRunnable ValidateRunnable would otherwise
	// raise first; manager untouched.
	t.Run("already running is run in flight", func(t *testing.T) {
		sim := runnableServiceSim("sim-2b", "alice")
		sim.Status = datasims.SimulationStatusRunning
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})

		_, err := svc.Start(ctxAs("alice", false), "sim-2b", nil)
		if err == nil || !errors.Is(err, ErrRunInFlight) {
			t.Fatalf("Start already-running err=%v want ErrRunInFlight", err)
		}
		if len(mgr.started) != 0 {
			t.Fatalf("manager.Start called for already-running sim: %v", mgr.started)
		}
	})

	// Paused (mid-run): also ErrRunInFlight, manager untouched.
	t.Run("paused is run in flight", func(t *testing.T) {
		sim := runnableServiceSim("sim-2c", "alice")
		sim.Status = datasims.SimulationStatusPaused
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})

		_, err := svc.Start(ctxAs("alice", false), "sim-2c", nil)
		if err == nil || !errors.Is(err, ErrRunInFlight) {
			t.Fatalf("Start paused err=%v want ErrRunInFlight", err)
		}
		if len(mgr.started) != 0 {
			t.Fatalf("manager.Start called for paused sim: %v", mgr.started)
		}
	})

	// Owner + ready → manager.Start called, reloaded body returned.
	t.Run("owner ready starts", func(t *testing.T) {
		sim := runnableServiceSim("sim-3", "alice")
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})

		resp, err := svc.Start(ctxAs("alice", false), "sim-3", nil)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if resp == nil {
			t.Fatal("Start returned nil response")
		}
		if len(mgr.started) != 1 || mgr.started[0] != "sim-3" {
			t.Fatalf("manager.Start not called once for sim-3: %v", mgr.started)
		}
	})

	// Admin may start a sim owned by another user.
	t.Run("admin can start foreign", func(t *testing.T) {
		sim := runnableServiceSim("sim-4", "alice")
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})

		if _, err := svc.Start(ctxAs("admin", true), "sim-4", nil); err != nil {
			t.Fatalf("admin Start: %v", err)
		}
		if len(mgr.started) != 1 {
			t.Fatalf("manager.Start not called for admin: %v", mgr.started)
		}
	})

	// Missing sim → ErrSimulationNotFound.
	t.Run("missing not found", func(t *testing.T) {
		repo := newFakeSimRepo()
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		if _, err := svc.Start(ctxAs("alice", false), "nope", nil); err != ErrSimulationNotFound {
			t.Fatalf("Start missing err=%v want ErrSimulationNotFound", err)
		}
	})
}

func TestRunService_ControlOwned(t *testing.T) {
	// Foreign sim → ErrForbidden, manager untouched.
	t.Run("foreign forbidden", func(t *testing.T) {
		sim := runnableServiceSim("sim-c1", "alice")
		sim.Status = datasims.SimulationStatusRunning
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		if err := svc.Control(ctxAs("bob", false), "sim-c1", "pause", nil); err != auth.ErrForbidden {
			t.Fatalf("Control foreign err=%v want ErrForbidden", err)
		}
		if len(mgr.controls) != 0 {
			t.Fatalf("manager.Control called for foreign sim: %v", mgr.controls)
		}
	})

	// Illegal action for status → ErrNotRunning (pause when not running).
	t.Run("illegal action", func(t *testing.T) {
		sim := runnableServiceSim("sim-c2", "alice") // ready (not running)
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		if err := svc.Control(ctxAs("alice", false), "sim-c2", "pause", nil); !errors.Is(err, ErrNotRunning) {
			t.Fatalf("Control pause-when-ready err=%v want ErrNotRunning", err)
		}
		if len(mgr.controls) != 0 {
			t.Fatal("manager.Control should not be called on illegal action")
		}
	})

	// Owner + legal action → manager.Control called with the mapped verb.
	t.Run("owner pause running", func(t *testing.T) {
		sim := runnableServiceSim("sim-c3", "alice")
		sim.Status = datasims.SimulationStatusRunning
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		if err := svc.Control(ctxAs("alice", false), "sim-c3", "pause", nil); err != nil {
			t.Fatalf("Control pause: %v", err)
		}
		if len(mgr.controls) != 1 || mgr.controls[0].msg.action != ctrlPause {
			t.Fatalf("manager.Control verb mismatch: %+v", mgr.controls)
		}
	})

	// speed_ms alongside a verb sends an extra ctrlSpeed message.
	t.Run("speed sends extra control", func(t *testing.T) {
		sim := runnableServiceSim("sim-c4", "alice")
		sim.Status = datasims.SimulationStatusPaused // resume is legal only while paused
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		speed := 250
		if err := svc.Control(ctxAs("alice", false), "sim-c4", "resume", &speed); err != nil {
			t.Fatalf("Control resume+speed: %v", err)
		}
		var sawSpeed bool
		for _, c := range mgr.controls {
			if c.msg.action == ctrlSpeed && c.msg.speedMs == 250 {
				sawSpeed = true
			}
		}
		if !sawSpeed {
			t.Fatalf("expected a ctrlSpeed(250) control message, got %+v", mgr.controls)
		}
	})

	// set_speed forwards ONLY a ctrlSpeed message — no lifecycle verb (in
	// particular, never ctrlStop, which controlActionFor's default branch would
	// otherwise map an unrecognized "set_speed" action to).
	t.Run("set_speed sends only ctrlSpeed no lifecycle verb", func(t *testing.T) {
		sim := runnableServiceSim("sim-c5", "alice")
		sim.Status = datasims.SimulationStatusRunning
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		speed := 500
		if err := svc.Control(ctxAs("alice", false), "sim-c5", "set_speed", &speed); err != nil {
			t.Fatalf("Control set_speed: %v", err)
		}
		if len(mgr.controls) != 1 {
			t.Fatalf("expected exactly one control message for set_speed, got %+v", mgr.controls)
		}
		if mgr.controls[0].msg.action != ctrlSpeed || mgr.controls[0].msg.speedMs != 500 {
			t.Fatalf("set_speed control message=%+v want ctrlSpeed(500)", mgr.controls[0].msg)
		}
	})

	// set_speed is also legal while paused (mirrors ValidateControl legality).
	t.Run("set_speed while paused sends only ctrlSpeed", func(t *testing.T) {
		sim := runnableServiceSim("sim-c6", "alice")
		sim.Status = datasims.SimulationStatusPaused
		repo := newFakeSimRepo(sim)
		mgr := &fakeRunStarter{}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		speed := 100
		if err := svc.Control(ctxAs("alice", false), "sim-c6", "set_speed", &speed); err != nil {
			t.Fatalf("Control set_speed while paused: %v", err)
		}
		if len(mgr.controls) != 1 || mgr.controls[0].msg.action != ctrlSpeed {
			t.Fatalf("set_speed while paused control messages=%+v want exactly one ctrlSpeed", mgr.controls)
		}
	})
}

func TestRunService_IssueTicketOwned(t *testing.T) {
	// Foreign sim → ErrForbidden, ticket not issued.
	t.Run("foreign forbidden", func(t *testing.T) {
		sim := runnableServiceSim("sim-t1", "alice")
		repo := newFakeSimRepo(sim)
		tickets := &fakeTickets{}
		svc := newTestRunService(repo, &fakeRunStarter{}, tickets, &fakeStepSink{})
		if _, _, err := svc.IssueTicket(ctxAs("bob", false), "sim-t1"); err != auth.ErrForbidden {
			t.Fatalf("IssueTicket foreign err=%v want ErrForbidden", err)
		}
		if len(tickets.issued) != 0 {
			t.Fatal("ticket issued for a foreign sim")
		}
	})

	// Owner of a running sim → ticket issued bound to (user, sim) with a future expiry.
	t.Run("owner issues for running", func(t *testing.T) {
		sim := runnableServiceSim("sim-t2", "alice")
		sim.Status = datasims.SimulationStatusRunning
		repo := newFakeSimRepo(sim)
		tickets := &fakeTickets{token: "opaque-token"}
		svc := newTestRunService(repo, &fakeRunStarter{}, tickets, &fakeStepSink{})
		tok, exp, err := svc.IssueTicket(ctxAs("alice", false), "sim-t2")
		if err != nil {
			t.Fatalf("IssueTicket: %v", err)
		}
		if tok != "opaque-token" {
			t.Fatalf("ticket=%q want opaque-token", tok)
		}
		if tickets.lastUser != "alice" {
			t.Fatalf("ticket bound to %q want alice", tickets.lastUser)
		}
		if len(tickets.issued) != 1 || tickets.issued[0] != "sim-t2" {
			t.Fatalf("ticket not issued for sim-t2: %v", tickets.issued)
		}
		if !exp.After(time.Now()) {
			t.Fatalf("expiry %v is not in the future", exp)
		}
	})

	// Paused is also streamable → ticket issued.
	t.Run("owner issues for paused", func(t *testing.T) {
		sim := runnableServiceSim("sim-t3", "alice")
		sim.Status = datasims.SimulationStatusPaused
		repo := newFakeSimRepo(sim)
		tickets := &fakeTickets{token: "tok-paused"}
		svc := newTestRunService(repo, &fakeRunStarter{}, tickets, &fakeStepSink{})
		if _, _, err := svc.IssueTicket(ctxAs("alice", false), "sim-t3"); err != nil {
			t.Fatalf("IssueTicket paused: %v", err)
		}
		if len(tickets.issued) != 1 {
			t.Fatalf("ticket not issued for a paused sim: %v", tickets.issued)
		}
	})

	// Owner of a non-live sim (ready / finished / failed / stopped) → ErrNotStreamable,
	// ticket NOT issued. This is the orphan-room guard: a ticket for a sim with no
	// live run would let the WS upgrade lazily create a room whose goroutine leaks.
	for _, status := range []datasims.SimulationStatus{
		datasims.SimulationStatusReady,
		datasims.SimulationStatusFinished,
		datasims.SimulationStatusFailed,
		datasims.SimulationStatusStopped,
	} {
		t.Run("not streamable: "+string(status), func(t *testing.T) {
			sim := runnableServiceSim("sim-t-"+string(status), "alice")
			sim.Status = status
			repo := newFakeSimRepo(sim)
			tickets := &fakeTickets{}
			svc := newTestRunService(repo, &fakeRunStarter{}, tickets, &fakeStepSink{})
			if _, _, err := svc.IssueTicket(ctxAs("alice", false), sim.ID); !errors.Is(err, ErrNotStreamable) {
				t.Fatalf("IssueTicket %q err=%v want ErrNotStreamable", status, err)
			}
			if len(tickets.issued) != 0 {
				t.Fatalf("ticket issued for a non-streamable %q sim: %v", status, tickets.issued)
			}
		})
	}
}

func TestRunService_ConsumeTicket(t *testing.T) {
	tickets := &fakeTickets{consumeOK: true, consumeUID: "alice"}
	svc := newTestRunService(newFakeSimRepo(), &fakeRunStarter{}, tickets, &fakeStepSink{})
	uid, ok := svc.ConsumeTicket("tok", "sim-x")
	if !ok || uid != "alice" {
		t.Fatalf("ConsumeTicket=(%q,%v) want (alice,true)", uid, ok)
	}

	tickets2 := &fakeTickets{consumeOK: false}
	svc2 := newTestRunService(newFakeSimRepo(), &fakeRunStarter{}, tickets2, &fakeStepSink{})
	if _, ok := svc2.ConsumeTicket("bad", "sim-x"); ok {
		t.Fatal("ConsumeTicket should fail on a rejected ticket")
	}
}

// u64 is a small test helper for an explicit *uint64 "to" argument.
func u64(v uint64) *uint64 { return &v }

func TestRunService_GetStepsOwnedPaged(t *testing.T) {
	// Foreign sim → ErrForbidden, repo untouched.
	t.Run("foreign forbidden", func(t *testing.T) {
		sim := runnableServiceSim("sim-s1", "alice")
		repo := newFakeSimRepo(sim)
		steps := &recordingStepSink{}
		svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, steps)
		if _, _, err := svc.GetSteps(ctxAs("bob", false), "sim-s1", 0, u64(5), 1, 20); err != auth.ErrForbidden {
			t.Fatalf("GetSteps foreign err=%v want ErrForbidden", err)
		}
		if steps.getStepsCalls != 0 {
			t.Fatal("steps repo queried for a foreign sim")
		}
	})

	// Owner + explicit "to" → steps repo queried with the supplied window,
	// UNCHANGED (even a huge value like the front sends, e.g. 1e9).
	t.Run("owner paged explicit to", func(t *testing.T) {
		sim := runnableServiceSim("sim-s2", "alice")
		repo := newFakeSimRepo(sim)
		steps := &recordingStepSink{
			steps:   []datasims.SimulationStep{{Metadata: datasims.StepMetadata{Step: 1, Type: datasims.MetadataTypeAgent, ReferenceID: 1}}},
			hasMore: true,
		}
		svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, steps)
		got, more, err := svc.GetSteps(ctxAs("alice", false), "sim-s2", 2, u64(1_000_000_000), 3, 25)
		if err != nil {
			t.Fatalf("GetSteps: %v", err)
		}
		if !more {
			t.Fatal("hasMore should propagate from the repo")
		}
		if len(got) != 1 {
			t.Fatalf("steps=%d want 1", len(got))
		}
		if steps.lastSimID != "sim-s2" || steps.lastFrom != 2 || steps.lastTo != 1_000_000_000 || steps.lastPage != 3 || steps.lastPageSize != 25 {
			t.Fatalf("GetSteps args mismatch: %+v", steps)
		}
		if steps.rangeCalls != 0 {
			t.Fatalf("GetStepRange should not be consulted when to is explicit, got %d calls", steps.rangeCalls)
		}
	})

	// Owner, no "to" → resolves to the last recorded step via GetStepRange.
	t.Run("owner paged no to defaults to last recorded step", func(t *testing.T) {
		sim := runnableServiceSim("sim-s3", "alice")
		repo := newFakeSimRepo(sim)
		steps := &recordingStepSink{
			steps: []datasims.SimulationStep{{Metadata: datasims.StepMetadata{Step: 1, Type: datasims.MetadataTypeAgent, ReferenceID: 1}}},
		}
		steps.rangeMin, steps.rangeMax = 0, 42
		svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, steps)
		got, _, err := svc.GetSteps(ctxAs("alice", false), "sim-s3", 0, nil, 1, 50)
		if err != nil {
			t.Fatalf("GetSteps: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("steps=%d want 1", len(got))
		}
		if steps.rangeCalls != 1 {
			t.Fatalf("GetStepRange calls=%d want 1", steps.rangeCalls)
		}
		if steps.lastTo != 42 {
			t.Fatalf("resolved to=%d want 42 (the recorded max step)", steps.lastTo)
		}
	})

	// Owner, no "to", no steps recorded yet (GetStepRange → common.ErrNotFound)
	// → empty result, hasMore=false, no error (not surfaced as a failure).
	t.Run("owner paged no to no steps yet returns empty", func(t *testing.T) {
		sim := runnableServiceSim("sim-s4", "alice")
		repo := newFakeSimRepo(sim)
		steps := &recordingStepSink{fakeStepSink: fakeStepSink{rangeErr: common.ErrNotFound}}
		svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, steps)
		got, more, err := svc.GetSteps(ctxAs("alice", false), "sim-s4", 0, nil, 1, 50)
		if err != nil {
			t.Fatalf("GetSteps: %v want nil error", err)
		}
		if more {
			t.Fatal("hasMore should be false when no steps are recorded")
		}
		if len(got) != 0 {
			t.Fatalf("steps=%d want 0", len(got))
		}
		if steps.getStepsCalls != 0 {
			t.Fatal("GetSteps should not be queried when no steps are recorded")
		}
	})
}

func TestRunService_AuthorizeStream(t *testing.T) {
	// A running (streamable) sim: owner + admin pass, foreign is forbidden.
	sim := runnableServiceSim("sim-a1", "alice")
	sim.Status = datasims.SimulationStatusRunning
	repo := newFakeSimRepo(sim)
	svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, &fakeStepSink{})

	if err := svc.AuthorizeStream(ctxAs("alice", false), "sim-a1"); err != nil {
		t.Fatalf("AuthorizeStream owner err=%v want nil", err)
	}
	if err := svc.AuthorizeStream(ctxAs("admin", true), "sim-a1"); err != nil {
		t.Fatalf("AuthorizeStream admin err=%v want nil", err)
	}
	if err := svc.AuthorizeStream(ctxAs("bob", false), "sim-a1"); err != auth.ErrForbidden {
		t.Fatalf("AuthorizeStream foreign err=%v want ErrForbidden", err)
	}

	// Ownership is re-checked BEFORE streamability: a foreign caller on a
	// non-streamable sim still gets ErrForbidden, not ErrNotStreamable.
	t.Run("foreign on finished is forbidden not not-streamable", func(t *testing.T) {
		fin := runnableServiceSim("sim-a2", "alice")
		fin.Status = datasims.SimulationStatusFinished
		svc := newTestRunService(newFakeSimRepo(fin), &fakeRunStarter{}, &fakeTickets{}, &fakeStepSink{})
		if err := svc.AuthorizeStream(ctxAs("bob", false), "sim-a2"); err != auth.ErrForbidden {
			t.Fatalf("AuthorizeStream foreign-finished err=%v want ErrForbidden", err)
		}
	})

	// Owner of a finished/never-run sim → ErrNotStreamable (the run-finished race
	// guard: no room exists to join).
	t.Run("owner on non-streamable is not-streamable", func(t *testing.T) {
		for _, status := range []datasims.SimulationStatus{
			datasims.SimulationStatusReady,
			datasims.SimulationStatusFinished,
			datasims.SimulationStatusStopped,
		} {
			fin := runnableServiceSim("sim-a-"+string(status), "alice")
			fin.Status = status
			svc := newTestRunService(newFakeSimRepo(fin), &fakeRunStarter{}, &fakeTickets{}, &fakeStepSink{})
			if err := svc.AuthorizeStream(ctxAs("alice", false), fin.ID); !errors.Is(err, ErrNotStreamable) {
				t.Fatalf("AuthorizeStream owner %q err=%v want ErrNotStreamable", status, err)
			}
		}
	})
}

// recordingStepSink is a SimulationStepsRepository fake that records the GetSteps
// query window and replays a canned page.
type recordingStepSink struct {
	fakeStepSink
	steps         []datasims.SimulationStep
	hasMore       bool
	getStepsCalls int
	lastSimID     string
	lastFrom      uint64
	lastTo        uint64
	lastPage      uint64
	lastPageSize  uint64
}

func (r *recordingStepSink) GetSteps(_ context.Context, simID string, from, to, page, pageSize uint64) ([]datasims.SimulationStep, bool, error) {
	r.getStepsCalls++
	r.lastSimID = simID
	r.lastFrom = from
	r.lastTo = to
	r.lastPage = page
	r.lastPageSize = pageSize
	return r.steps, r.hasMore, nil
}

func TestRunService_GetAgent(t *testing.T) {
	mkSim := func() *datasims.Simulation {
		sim := runnableServiceSim("sim-ga", "alice")
		sim.Agents[0].Memory = []datasims.AgentMemory{{Step: 1, Type: datasims.AgentMemoryMovement, Summary: "moved"}}
		return sim
	}

	// No live run: served from the persisted record (memory included).
	t.Run("persisted fallback", func(t *testing.T) {
		repo := newFakeSimRepo(mkSim())
		svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, &fakeStepSink{})
		dto, err := svc.GetAgent(ctxAs("alice", false), "sim-ga", 1)
		if err != nil {
			t.Fatalf("GetAgent: %v", err)
		}
		if dto.ID != 1 || len(dto.Memory) != 1 || dto.Memory[0].Summary != "moved" {
			t.Fatalf("persisted agent mismatch: %+v", dto)
		}
	})

	// Live run: the manager's snapshot wins over the (stale) persisted record.
	t.Run("live snapshot wins", func(t *testing.T) {
		repo := newFakeSimRepo(mkSim())
		mgr := &fakeRunStarter{
			running: map[string]bool{"sim-ga": true},
			liveAgents: map[uint64]*datasims.SimulationAgent{
				1: {ID: 1, AgentUID: "wolf", State: "hunting",
					Memory: []datasims.AgentMemory{{Step: 7, Type: datasims.AgentMemoryLLM, Summary: "chose: hunt"}}},
			},
		}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		dto, err := svc.GetAgent(ctxAs("alice", false), "sim-ga", 1)
		if err != nil {
			t.Fatalf("GetAgent: %v", err)
		}
		if dto.State != "hunting" || len(dto.Memory) != 1 || dto.Memory[0].Summary != "chose: hunt" {
			t.Fatalf("live agent mismatch: %+v", dto)
		}
	})

	// Unknown instance id in a live run -> ErrAgentNotFound.
	t.Run("live unknown id", func(t *testing.T) {
		repo := newFakeSimRepo(mkSim())
		mgr := &fakeRunStarter{
			running:    map[string]bool{"sim-ga": true},
			liveAgents: map[uint64]*datasims.SimulationAgent{},
		}
		svc := newTestRunService(repo, mgr, &fakeTickets{}, &fakeStepSink{})
		if _, err := svc.GetAgent(ctxAs("alice", false), "sim-ga", 99); !errors.Is(err, ErrAgentNotFound) {
			t.Fatalf("GetAgent unknown live id err=%v want ErrAgentNotFound", err)
		}
	})

	// Foreign non-admin -> forbidden.
	t.Run("foreign forbidden", func(t *testing.T) {
		repo := newFakeSimRepo(mkSim())
		svc := newTestRunService(repo, &fakeRunStarter{}, &fakeTickets{}, &fakeStepSink{})
		if _, err := svc.GetAgent(ctxAs("bob", false), "sim-ga", 1); err != auth.ErrForbidden {
			t.Fatalf("GetAgent foreign err=%v want ErrForbidden", err)
		}
	})
}
