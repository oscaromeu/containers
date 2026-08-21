// allquiet-sync reconciles AllQuiet triage verdicts into e2e.incidents
// (CORE-1841, Fase 2). One-shot by design: a CronJob schedules it, the
// cycle is idempotent, and a non-zero exit (error or broken invariant)
// pages through the existing job-failure alerts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/oscaromeu/containers/apps/allquiet-sync/allquiet"
	"github.com/oscaromeu/containers/apps/allquiet-sync/store"
	appsync "github.com/oscaromeu/containers/apps/allquiet-sync/sync"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(context.Background(), log); err != nil {
		log.Error("sync failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	var (
		teamIDs   = flag.String("team-ids", "", "comma-separated AllQuiet team ids to sync (required)")
		env       = flag.String("env", "", "environment written to incidents and used to read runs (required)")
		chAddr    = flag.String("clickhouse-addr", "localhost:9000", "ClickHouse native address")
		chDB      = flag.String("clickhouse-database", "e2e", "database holding runs and incidents")
		chUser    = flag.String("clickhouse-user", "default", "ClickHouse user (password via CLICKHOUSE_PASSWORD)")
		aqURL     = flag.String("allquiet-url", "https://allquiet.eu", "AllQuiet base URL (EU region)")
		lookback  = flag.Duration("lookback", 7*24*time.Hour, "list incidents updated within this window")
		snap      = flag.Duration("snap-window", 6*time.Hour, "search radius around the incident creation when snapping to the real episode")
		probeAttr = flag.String("probe-attribute", "Job", "incident attribute holding the probe name when no service was affected")
		retention = flag.Int("runs-retention-days", 90, "runs retention; bounds the orphan invariant")
		dryRun    = flag.Bool("dry-run", false, "log every action without writing")
	)
	flag.Parse()

	apiKey := os.Getenv("ALLQUIET_API_KEY")
	switch {
	case apiKey == "":
		return errors.New("ALLQUIET_API_KEY is not set")
	case *teamIDs == "":
		return errors.New("--team-ids is required")
	case *env == "":
		return errors.New("--env is required")
	}

	client, err := allquiet.New(apiKey, allquiet.WithBaseURL(*aqURL))
	if err != nil {
		return err
	}

	st, err := store.Open(store.Config{
		Addr:              *chAddr,
		Database:          *chDB,
		Username:          *chUser,
		Password:          os.Getenv("CLICKHOUSE_PASSWORD"),
		Env:               *env,
		RunsRetentionDays: *retention,
	})
	if err != nil {
		return err
	}
	defer st.Close()

	incidents, err := client.SearchIncidents(ctx, allquiet.SearchParams{
		TeamIDs:         strings.Split(*teamIDs, ","),
		LastUpdatedFrom: time.Now().Add(-*lookback),
	})
	if err != nil {
		return err
	}

	// The listing truncates long timelines; the verdict may live in the cut.
	for i, inc := range incidents {
		if len(inc.Events) >= inc.EventsTotalCount {
			continue
		}
		full, err := client.GetIncident(ctx, inc.ID)
		if err != nil {
			return fmt.Errorf("refetch incident %s: %w", inc.ID, err)
		}
		incidents[i] = *full
	}

	res, err := appsync.Run(ctx, incidents, st, appsync.Options{
		SnapWindow:     *snap,
		ProbeAttribute: *probeAttr,
		DryRun:         *dryRun,
		Log:            log,
	})
	if err != nil {
		return err
	}

	log.Info("cycle done", "incidents", len(incidents), "inserted", res.Inserted,
		"retracted", res.Retracted, "unchanged", res.Unchanged, "dry_run", *dryRun)
	if res.Overlaps > 0 || res.Orphans > 0 {
		return fmt.Errorf("invariants violated: %d overlapping, %d orphan windows", res.Overlaps, res.Orphans)
	}
	return nil
}
