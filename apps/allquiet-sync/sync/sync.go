// Package sync turns AllQuiet triage verdicts into confirmed incident
// windows and reconciles them into e2e.incidents. It is pure decision
// logic: no HTTP, no SQL — main wires it to the client and the store.
//
// The triage contract (CORE-1841, decided 2026-08-14):
//
//	Affects service        -> downtime      (the official SLO goes down)
//	Archive without affects -> false_positive (the official SLO does not move)
//	nothing                -> no window     (stays purple in the review queue)
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/oscaromeu/containers/apps/allquiet-sync/allquiet"
)

// Sentinel marks a window without an end (incident still open). A far value
// instead of NULL keeps the dashboard join a flat comparison.
var Sentinel = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

// Intent and status strings as the AllQuiet timeline reports them, verified
// against the sandbox on 2026-08-21 (other observed intents: Created, Resolved).
const (
	IntentAffects  = "Affects"
	IntentArchive  = "Archived"
	StatusResolved = "Resolved"
)

// alertLag covers the gap between the last fail of a short episode and the
// alert that creates the incident (scrape + eval + group_wait).
const alertLag = 10 * time.Minute

const (
	KindDowntime      = "downtime"
	KindFalsePositive = "false_positive"
)

// Window is one confirmed-incident window, desired or in force.
type Window struct {
	Probe       string
	StartedAt   time.Time
	EndedAt     time.Time
	Kind        string
	ExternalID  string
	Reason      string
	ConfirmedBy string
}

// Episode is a real fail streak in e2e.runs.
type Episode struct {
	Start      time.Time
	End        time.Time
	FailedRuns int
}

// Store is what the reconcile needs from ClickHouse.
type Store interface {
	// Episodes returns the fail streaks of a probe inside [from, to).
	Episodes(ctx context.Context, probe string, from, to time.Time) ([]Episode, error)
	// CurrentWindows returns the statuspage windows currently in force.
	CurrentWindows(ctx context.Context) ([]Window, error)
	// Insert appends one version of a window; deleted retracts it.
	Insert(ctx context.Context, w Window, deleted bool) error
	// Invariants counts overlapping and orphan windows; both must be 0.
	Invariants(ctx context.Context) (overlaps, orphans int, err error)
}

// Options tune one reconcile cycle.
type Options struct {
	// SnapWindow is the search radius around the incident creation when
	// snapping to the real episode: the incident timestamps lie on both
	// sides (alert delay, group_interval), so they are hints, not limits.
	SnapWindow time.Duration
	// ProbeAttribute names the incident attribute holding the probe when
	// no service was affected (the false-positive path).
	ProbeAttribute string
	DryRun         bool
	Log            *slog.Logger
}

// Result summarizes one cycle.
type Result struct {
	Inserted  int
	Retracted int
	Unchanged int
	Overlaps  int
	Orphans   int
}

type key struct {
	probe     string
	startedMs int64
}

// Run reconciles one batch of incidents into the store. Re-running with the
// same inputs writes nothing: the desired state is derived from AllQuiet and
// compared against what is already in force.
func Run(ctx context.Context, incidents []allquiet.Incident, st Store, opts Options) (*Result, error) {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	desired := make(map[key]Window)
	seen := make(map[string]bool)
	for _, inc := range incidents {
		seen[inc.ID] = true
		windows, err := desiredWindows(ctx, inc, st, opts, log)
		if err != nil {
			return nil, err
		}
		for _, w := range windows {
			desired[key{w.Probe, w.StartedAt.UnixMilli()}] = w
		}
	}

	current, err := st.CurrentWindows(ctx)
	if err != nil {
		return nil, err
	}
	inForce := make(map[key]Window, len(current))
	for _, w := range current {
		inForce[key{w.Probe, w.StartedAt.UnixMilli()}] = w
	}

	res := &Result{}
	for k, d := range desired {
		if c, ok := inForce[k]; ok && sameWindow(c, d) {
			res.Unchanged++
			continue
		}
		log.Info("window", "action", "insert", "probe", d.Probe, "kind", d.Kind,
			"started_at", d.StartedAt, "ended_at", d.EndedAt, "incident", d.ExternalID,
			"confirmed_by", d.ConfirmedBy, "dry_run", opts.DryRun)
		if !opts.DryRun {
			if err := st.Insert(ctx, d, false); err != nil {
				return nil, fmt.Errorf("insert window for %s: %w", d.Probe, err)
			}
		}
		res.Inserted++
	}

	// Retraction (caso 5): a window in force whose incident no longer wants
	// it — the affects was taken away. Only incidents fetched this cycle can
	// retract; anything older than the lookback is left alone.
	for k, c := range inForce {
		if !seen[c.ExternalID] {
			continue
		}
		if _, ok := desired[k]; ok {
			continue
		}
		log.Info("window", "action", "retract", "probe", c.Probe, "kind", c.Kind,
			"started_at", c.StartedAt, "incident", c.ExternalID, "dry_run", opts.DryRun)
		if !opts.DryRun {
			if err := st.Insert(ctx, c, true); err != nil {
				return nil, fmt.Errorf("retract window for %s: %w", c.Probe, err)
			}
		}
		res.Retracted++
	}

	res.Overlaps, res.Orphans, err = st.Invariants(ctx)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type verdict struct {
	kind        string
	probes      []string
	confirmedBy string
	resolvedAt  *time.Time
}

// verdictOf reads the human judgment off an incident. Affects computes
// AllQuiet's own uptime, so it is reserved for real downtime; Archive
// without affects is the deliberate false-positive click. The automatic
// Resolved is not a judgment and maps to nothing.
func verdictOf(inc allquiet.Incident, probeAttribute string) verdict {
	var v verdict
	switch {
	case len(inc.Services) > 0:
		v.kind = KindDowntime
		for _, s := range inc.Services {
			v.probes = append(v.probes, s.DisplayName)
		}
		v.confirmedBy = lastUserByIntent(inc.Events, IntentAffects)
	case inc.IsArchived:
		v.kind = KindFalsePositive
		if p := inc.AttributeValue(probeAttribute); p != "" {
			v.probes = []string{p}
		}
		v.confirmedBy = lastUserByIntent(inc.Events, IntentArchive)
	default:
		return v
	}
	v.resolvedAt = resolvedAt(inc.Events)
	return v
}

// resolvedAt returns when the incident last became Resolved, or nil if it is
// open — including reopened after a resolve (caso 3).
func resolvedAt(events []allquiet.Event) *time.Time {
	sorted := make([]allquiet.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Modification.Timestamp.Before(sorted[j].Modification.Timestamp)
	})
	var res *time.Time
	for _, e := range sorted {
		if e.Status == nil {
			continue
		}
		if *e.Status == StatusResolved {
			t := e.Modification.Timestamp
			res = &t
		} else {
			res = nil
		}
	}
	return res
}

func lastUserByIntent(events []allquiet.Event, intent string) string {
	var who string
	var when time.Time
	for _, e := range events {
		if e.Modification.Intent != intent || e.Modification.User == nil {
			continue
		}
		if !e.Modification.Timestamp.Before(when) {
			when = e.Modification.Timestamp
			who = e.Modification.User.DisplayName
		}
	}
	return who
}

func desiredWindows(ctx context.Context, inc allquiet.Incident, st Store, opts Options, log *slog.Logger) ([]Window, error) {
	v := verdictOf(inc, opts.ProbeAttribute)
	if v.kind == "" {
		return nil, nil
	}
	if inc.CreatedAt == nil {
		log.Warn("incident without createdAt, skipping", "incident", inc.ID)
		return nil, nil
	}
	if len(v.probes) == 0 {
		log.Warn("judged incident without a probe, skipping",
			"incident", inc.ID, "kind", v.kind, "attribute", opts.ProbeAttribute)
		return nil, nil
	}
	createdAt := *inc.CreatedAt

	hintEnd := createdAt
	if v.resolvedAt != nil {
		hintEnd = *v.resolvedAt
	}

	var out []Window
	for _, probe := range v.probes {
		if probe == "" {
			log.Warn("service without display name, skipping", "incident", inc.ID)
			continue
		}
		episodes, err := st.Episodes(ctx, probe, createdAt.Add(-opts.SnapWindow), hintEnd.Add(opts.SnapWindow))
		if err != nil {
			return nil, fmt.Errorf("episodes for %s: %w", probe, err)
		}
		started, ended, snapped := windowBounds(v, createdAt, episodes, opts.SnapWindow)
		if !snapped {
			// casos 7 y 8: the human confirmation is never dropped, but a
			// window built from incident timestamps may show up as orphan
			// in the invariants until someone reviews it. On purpose.
			log.Warn("no unambiguous episode, using incident timestamps",
				"incident", inc.ID, "probe", probe, "episodes", len(episodes))
		}
		out = append(out, Window{
			Probe:       probe,
			StartedAt:   started,
			EndedAt:     ended,
			Kind:        v.kind,
			ExternalID:  inc.ID,
			Reason:      inc.Title,
			ConfirmedBy: v.confirmedBy,
		})
	}
	return out, nil
}

// windowBounds snaps the window to the real episode: only e2e.runs holds the
// truth about when the failing actually started and stopped. The alert fires
// while the episode runs, so the right episode is the one containing
// createdAt (with alertLag of slack); the latest such start wins.
func windowBounds(v verdict, createdAt time.Time, episodes []Episode, snap time.Duration) (started, ended time.Time, snapped bool) {
	var candidate *Episode
	for i, ep := range episodes {
		if !ep.Start.After(createdAt) && !ep.End.Add(alertLag).Before(createdAt) {
			candidate = &episodes[i]
		}
	}

	open := v.kind == KindDowntime && v.resolvedAt == nil
	if candidate != nil {
		if open {
			return candidate.Start, Sentinel, true
		}
		return candidate.Start, ceilSecond(candidate.End), true
	}

	started = createdAt
	switch {
	case open:
		ended = Sentinel
	case v.resolvedAt != nil:
		ended = ceilSecond(*v.resolvedAt)
	default:
		// a false positive is always a closed window, even without a resolve
		ended = ceilSecond(createdAt)
	}
	return started, ended, false
}

// ceilSecond rounds up to the next whole second (lección T2: the closing
// bound is the last fail rounded up, and the bounds are inclusive).
func ceilSecond(t time.Time) time.Time {
	truncated := t.Truncate(time.Second)
	if truncated.Equal(t) {
		return t
	}
	return truncated.Add(time.Second)
}

// Reason stays out of the diff on purpose: it is human-owned free text and a
// human note must survive the sync (the title only seeds new windows).
func sameWindow(a, b Window) bool {
	return a.EndedAt.Equal(b.EndedAt) &&
		a.Kind == b.Kind &&
		a.ExternalID == b.ExternalID &&
		a.ConfirmedBy == b.ConfirmedBy
}
