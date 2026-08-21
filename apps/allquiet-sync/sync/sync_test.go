package sync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oscaromeu/containers/apps/allquiet-sync/allquiet"
)

var (
	t0        = time.Date(2026, 8, 14, 14, 2, 59, 0, time.UTC) // incident createdAt
	epStart   = time.Date(2026, 8, 14, 14, 1, 5, 101e6, time.UTC)
	epEnd     = time.Date(2026, 8, 14, 14, 6, 4, 648e6, time.UTC)
	epEndCeil = time.Date(2026, 8, 14, 14, 6, 5, 0, time.UTC)
)

func str(s string) *string { return &s }

func event(ts time.Time, intent string, status *string, user string) allquiet.Event {
	e := allquiet.Event{Status: status}
	e.Modification.Timestamp = ts
	e.Modification.Intent = intent
	if user != "" {
		e.Modification.User = &allquiet.User{DisplayName: user}
	}
	return e
}

func incident(id string, services []string, archived bool, events ...allquiet.Event) allquiet.Incident {
	created := t0
	inc := allquiet.Incident{
		ID:               id,
		Title:            "E2E probe canary is failing.",
		CreatedAt:        &created,
		IsArchived:       archived,
		Events:           events,
		EventsTotalCount: len(events),
		Attributes:       []allquiet.Attribute{{Name: "Job", Value: "canary"}},
	}
	for _, s := range services {
		inc.Services = append(inc.Services, allquiet.Entity{ID: "svc-" + s, DisplayName: s})
	}
	return inc
}

func TestVerdictOf(t *testing.T) {
	affects := event(t0.Add(time.Minute), IntentAffects, nil, "Oscar")
	archive := event(t0.Add(2*time.Minute), IntentArchive, nil, "Oscar")

	tests := []struct {
		name        string
		inc         allquiet.Incident
		kind        string
		probes      []string
		confirmedBy string
	}{
		{"caso 1: sin juicio humano no hay ventana", incident("i1", nil, false), "", nil, ""},
		{"affects es caída real", incident("i2", []string{"canary"}, false, affects), KindDowntime, []string{"canary"}, "Oscar"},
		{"archive sin affects es falso positivo, la sonda sale del atributo", incident("i3", nil, true, archive), KindFalsePositive, []string{"canary"}, "Oscar"},
		{"caso 6: archivado con affects sigue siendo caída", incident("i4", []string{"canary"}, true, affects), KindDowntime, []string{"canary"}, "Oscar"},
		{"caso 4: dos servicios, dos sondas", incident("i5", []string{"canary", "echo"}, false, affects), KindDowntime, []string{"canary", "echo"}, "Oscar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := verdictOf(tt.inc, "Job")
			if v.kind != tt.kind {
				t.Fatalf("kind = %q, want %q", v.kind, tt.kind)
			}
			if fmt.Sprint(v.probes) != fmt.Sprint(tt.probes) {
				t.Fatalf("probes = %v, want %v", v.probes, tt.probes)
			}
			if v.confirmedBy != tt.confirmedBy {
				t.Fatalf("confirmedBy = %q, want %q", v.confirmedBy, tt.confirmedBy)
			}
		})
	}
}

func TestResolvedAt(t *testing.T) {
	resolve := event(t0.Add(4*time.Minute), "Resolve", str(StatusResolved), "")
	reopen := event(t0.Add(5*time.Minute), "Unresolve", str("Open"), "Oscar")
	resolveAgain := event(t0.Add(6*time.Minute), "Resolve", str(StatusResolved), "Oscar")

	if got := resolvedAt(nil); got != nil {
		t.Fatalf("sin eventos = %v, want nil", got)
	}
	if got := resolvedAt([]allquiet.Event{resolve}); got == nil || !got.Equal(t0.Add(4*time.Minute)) {
		t.Fatalf("resuelto = %v, want %v", got, t0.Add(4*time.Minute))
	}
	// caso 3: reabierto tras un resolve vuelve a ser ventana abierta
	if got := resolvedAt([]allquiet.Event{resolve, reopen}); got != nil {
		t.Fatalf("reabierto = %v, want nil", got)
	}
	if got := resolvedAt([]allquiet.Event{resolve, reopen, resolveAgain}); got == nil || !got.Equal(t0.Add(6*time.Minute)) {
		t.Fatalf("re-resuelto = %v, want %v", got, t0.Add(6*time.Minute))
	}
}

func TestWindowBounds(t *testing.T) {
	snap := 6 * time.Hour
	resolved := t0.Add(5 * time.Minute)
	episode := Episode{Start: epStart, End: epEnd, FailedRuns: 6}

	t.Run("episodio único cerrado: snap con redondeo hacia arriba", func(t *testing.T) {
		v := verdict{kind: KindDowntime, resolvedAt: &resolved}
		started, ended, snapped := windowBounds(v, t0, []Episode{episode}, snap)
		if !snapped || !started.Equal(epStart) || !ended.Equal(epEndCeil) {
			t.Fatalf("got %v..%v snapped=%v", started, ended, snapped)
		}
	})
	t.Run("caso 2: incidente abierto es ventana abierta (sentinel)", func(t *testing.T) {
		v := verdict{kind: KindDowntime}
		started, ended, snapped := windowBounds(v, t0, []Episode{episode}, snap)
		if !snapped || !started.Equal(epStart) || !ended.Equal(Sentinel) {
			t.Fatalf("got %v..%v snapped=%v", started, ended, snapped)
		}
	})
	t.Run("caso 7: sin episodio caen las pistas del incidente, sin snap", func(t *testing.T) {
		v := verdict{kind: KindDowntime, resolvedAt: &resolved}
		started, ended, snapped := windowBounds(v, t0, nil, snap)
		if snapped || !started.Equal(t0) || !ended.Equal(ceilSecond(resolved)) {
			t.Fatalf("got %v..%v snapped=%v", started, ended, snapped)
		}
	})
	t.Run("gana el episodio que contiene createdAt, no los vecinos", func(t *testing.T) {
		neighbor := Episode{Start: t0.Add(-time.Hour), End: t0.Add(-50 * time.Minute)}
		v := verdict{kind: KindDowntime, resolvedAt: &resolved}
		started, _, snapped := windowBounds(v, t0, []Episode{neighbor, episode}, snap)
		if !snapped || !started.Equal(epStart) {
			t.Fatalf("got %v snapped=%v, want the containing episode", started, snapped)
		}
	})
	t.Run("episodio corto reciente casa por el margen de la alerta", func(t *testing.T) {
		short := Episode{Start: t0.Add(-9 * time.Minute), End: t0.Add(-5 * time.Minute)}
		v := verdict{kind: KindDowntime, resolvedAt: &resolved}
		started, _, snapped := windowBounds(v, t0, []Episode{short}, snap)
		if !snapped || !started.Equal(short.Start) {
			t.Fatalf("got %v snapped=%v, want lag match", started, snapped)
		}
	})
	t.Run("episodio lejano no casa: pistas crudas", func(t *testing.T) {
		far := Episode{Start: t0.Add(-8 * time.Hour), End: t0.Add(-7 * time.Hour)}
		v := verdict{kind: KindDowntime, resolvedAt: &resolved}
		_, _, snapped := windowBounds(v, t0, []Episode{far}, snap)
		if snapped {
			t.Fatal("an episode long over must not snap")
		}
	})
	t.Run("falso positivo sin resolve: ventana cerrada siempre", func(t *testing.T) {
		v := verdict{kind: KindFalsePositive}
		_, ended, _ := windowBounds(v, t0, nil, snap)
		if ended.Equal(Sentinel) {
			t.Fatal("a false positive must never be an open window")
		}
	})
}

func TestCeilSecond(t *testing.T) {
	exact := time.Date(2026, 8, 14, 14, 6, 5, 0, time.UTC)
	if got := ceilSecond(exact); !got.Equal(exact) {
		t.Fatalf("exact second changed: %v", got)
	}
	if got := ceilSecond(epEnd); !got.Equal(epEndCeil) {
		t.Fatalf("ceil(%v) = %v, want %v", epEnd, got, epEndCeil)
	}
}

// fakeStore models the append-only table: the version in force of a key is
// the last insert, and a deleted last version retracts it.
type fakeStore struct {
	episodes map[string][]Episode
	inForce  map[key]Window
	inserted int
	deleted  int
}

func newFakeStore() *fakeStore {
	return &fakeStore{episodes: map[string][]Episode{}, inForce: map[key]Window{}}
}

func (f *fakeStore) Episodes(_ context.Context, probe string, from, to time.Time) ([]Episode, error) {
	var out []Episode
	for _, ep := range f.episodes[probe] {
		if !ep.End.Before(from) && ep.Start.Before(to) {
			out = append(out, ep)
		}
	}
	return out, nil
}

func (f *fakeStore) CurrentWindows(context.Context) ([]Window, error) {
	var out []Window
	for _, w := range f.inForce {
		out = append(out, w)
	}
	return out, nil
}

func (f *fakeStore) Insert(_ context.Context, w Window, deleted bool) error {
	k := key{w.Probe, w.StartedAt.UnixMilli()}
	if deleted {
		f.deleted++
		delete(f.inForce, k)
		return nil
	}
	f.inserted++
	f.inForce[k] = w
	return nil
}

func (f *fakeStore) Invariants(context.Context) (int, int, error) { return 0, 0, nil }

func TestRunReconciles(t *testing.T) {
	ctx := context.Background()
	opts := Options{SnapWindow: 6 * time.Hour, ProbeAttribute: "Job"}
	affects := event(t0.Add(time.Minute), IntentAffects, nil, "Oscar")
	resolve := event(t0.Add(5*time.Minute), "Resolve", str(StatusResolved), "")

	st := newFakeStore()
	st.episodes["canary"] = []Episode{{Start: epStart, End: epEnd, FailedRuns: 6}}

	open := incident("d5592bcf", []string{"canary"}, false, affects)

	// primer ciclo: ventana abierta
	res, err := Run(ctx, []allquiet.Incident{open}, st, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 1 || res.Retracted != 0 {
		t.Fatalf("first cycle: %+v", res)
	}
	w := st.inForce[key{"canary", epStart.UnixMilli()}]
	if !w.EndedAt.Equal(Sentinel) || w.Kind != KindDowntime || w.ExternalID != "d5592bcf" {
		t.Fatalf("open window = %+v", w)
	}

	// mismo input otra vez: cero escrituras (idempotencia sin estado propio)
	res, err = Run(ctx, []allquiet.Incident{open}, st, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 0 || res.Retracted != 0 || res.Unchanged != 1 {
		t.Fatalf("idempotent cycle: %+v", res)
	}

	// el resolve cierra la ventana por upsert de la misma clave
	closed := incident("d5592bcf", []string{"canary"}, false, affects, resolve)
	res, err = Run(ctx, []allquiet.Incident{closed}, st, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 1 {
		t.Fatalf("close cycle: %+v", res)
	}
	w = st.inForce[key{"canary", epStart.UnixMilli()}]
	if !w.EndedAt.Equal(epEndCeil) {
		t.Fatalf("closed window ends %v, want %v", w.EndedAt, epEndCeil)
	}

	// caso 5: quitan el affects y la ventana se retracta
	unaffected := incident("d5592bcf", nil, false, affects, resolve)
	res, err = Run(ctx, []allquiet.Incident{unaffected}, st, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Retracted != 1 || len(st.inForce) != 0 {
		t.Fatalf("retract cycle: %+v inForce=%v", res, st.inForce)
	}
}

func TestRunLeavesUnfetchedIncidentsAlone(t *testing.T) {
	st := newFakeStore()
	old := Window{Probe: "echo", StartedAt: t0.Add(-30 * 24 * time.Hour), EndedAt: t0,
		Kind: KindDowntime, ExternalID: "ancient"}
	st.inForce[key{"echo", old.StartedAt.UnixMilli()}] = old

	res, err := Run(context.Background(), nil, st, Options{SnapWindow: time.Hour, ProbeAttribute: "Job"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Retracted != 0 || len(st.inForce) != 1 {
		t.Fatalf("a window outside the fetched batch was touched: %+v", res)
	}
}

func TestRunMultiService(t *testing.T) {
	st := newFakeStore()
	affects := event(t0.Add(time.Minute), IntentAffects, nil, "Oscar")
	resolve := event(t0.Add(5*time.Minute), "Resolve", str(StatusResolved), "")
	inc := incident("multi", []string{"canary", "echo"}, false, affects, resolve)

	res, err := Run(context.Background(), []allquiet.Incident{inc}, st,
		Options{SnapWindow: 6 * time.Hour, ProbeAttribute: "Job"})
	if err != nil {
		t.Fatal(err)
	}
	// caso 4: N ventanas, mismo external_id; sin episodio caen a pistas crudas
	if res.Inserted != 2 {
		t.Fatalf("multi-service: %+v", res)
	}
	for _, w := range st.inForce {
		if w.ExternalID != "multi" {
			t.Fatalf("window %+v lost its external id", w)
		}
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	st := newFakeStore()
	affects := event(t0.Add(time.Minute), IntentAffects, nil, "Oscar")
	inc := incident("dry", []string{"canary"}, false, affects)

	res, err := Run(context.Background(), []allquiet.Incident{inc}, st,
		Options{SnapWindow: 6 * time.Hour, ProbeAttribute: "Job", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 1 || st.inserted != 0 || len(st.inForce) != 0 {
		t.Fatalf("dry-run wrote: %+v inserted=%d", res, st.inserted)
	}
}
