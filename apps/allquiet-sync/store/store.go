// Package store is the ClickHouse adapter: episodes come from e2e.runs, the
// windows in force from e2e.incidents, and every write is an append — the
// table is the audit trail of who decided what and when, so nothing is ever
// updated or deleted.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	appsync "github.com/oscaromeu/containers/apps/allquiet-sync/sync"
)

// Config connects the store to one ClickHouse and one environment.
type Config struct {
	Addr     string
	Database string
	Username string
	Password string
	// Env scopes every query; it is also what gets written, and the table
	// constraint rejects it empty so a bad config fails loudly.
	Env string
	// RunsRetentionDays bounds the orphan invariant: windows outlive runs
	// (2y vs 90d/400d), so older windows are orphans by design, not by bug.
	RunsRetentionDays int
}

// Store implements sync.Store against ClickHouse.
type Store struct {
	conn driver.Conn
	cfg  Config
}

// Open connects and pings. Inserts always wait for the answer: constraints
// only protect when somebody is listening (verified in prod — a violation
// under wait_for_async_insert=0 is lost in silence).
func Open(cfg Config) (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"wait_for_async_insert": 1,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping %s: %w", cfg.Addr, err)
	}
	return &Store{conn: conn, cfg: cfg}, nil
}

// Close releases the connection pool.
func (s *Store) Close() error {
	return s.conn.Close()
}

// The lab already renamed runs.status -> runs.outcome (C3); prod still has
// the old name, so pointing this at prod requires the rename first.
const episodesQuery = `
WITH r AS (
    SELECT started_at, outcome,
           sum(if(outcome = 'pass', 1, 0)) OVER (
               ORDER BY started_at
               ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
           ) AS episode_id
    FROM runs FINAL
    WHERE env = ? AND probe = ?
      AND started_at >= ? AND started_at < ?
)
SELECT min(started_at) AS episode_start,
       max(started_at) AS episode_end,
       toInt64(count()) AS failed_runs
FROM r
WHERE outcome != 'pass'
GROUP BY episode_id
ORDER BY episode_start`

// Episodes returns the fail streaks of a probe inside [from, to).
func (s *Store) Episodes(ctx context.Context, probe string, from, to time.Time) ([]appsync.Episode, error) {
	rows, err := s.conn.Query(ctx, episodesQuery, s.cfg.Env, probe, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []appsync.Episode
	for rows.Next() {
		var ep appsync.Episode
		var failed int64
		if err := rows.Scan(&ep.Start, &ep.End, &failed); err != nil {
			return nil, err
		}
		ep.FailedRuns = int(failed)
		episodes = append(episodes, ep)
	}
	return episodes, rows.Err()
}

// The table is append-only: the version in force of each window is the one
// with the highest updated_at, and a deleted latest version retracts it.
const currentWindowsQuery = `
SELECT probe, started_at,
       argMax(ended_at, updated_at)     AS ended_at,
       argMax(kind, updated_at)         AS kind,
       argMax(external_id, updated_at)  AS external_id,
       argMax(reason, updated_at)       AS reason,
       argMax(confirmed_by, updated_at) AS confirmed_by
FROM incidents
WHERE env = ?
GROUP BY probe, env, started_at
HAVING argMax(deleted, updated_at) = 0
   AND argMax(source, updated_at) = 'statuspage'`

// CurrentWindows returns the statuspage windows in force; manual windows are
// untouchable and never reported.
func (s *Store) CurrentWindows(ctx context.Context) ([]appsync.Window, error) {
	rows, err := s.conn.Query(ctx, currentWindowsQuery, s.cfg.Env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []appsync.Window
	for rows.Next() {
		var w appsync.Window
		if err := rows.Scan(&w.Probe, &w.StartedAt, &w.EndedAt, &w.Kind,
			&w.ExternalID, &w.Reason, &w.ConfirmedBy); err != nil {
			return nil, err
		}
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

const insertQuery = `
INSERT INTO incidents
    (probe, env, started_at, ended_at, kind, source, external_id, reason, confirmed_by, deleted, updated_at)`

// Insert appends one version of a window. updated_at comes from here, not
// from the column default: it must be strictly increasing per key or argMax
// cannot tell versions apart. Always via batch: Exec interpolates time.Time
// at second precision and silently drops the DateTime64(3) milliseconds,
// which corrupts the window key (found the hard way, 2026-08-21).
func (s *Store) Insert(ctx context.Context, w appsync.Window, deleted bool) error {
	var del uint8
	if deleted {
		del = 1
	}
	batch, err := s.conn.PrepareBatch(ctx, insertQuery)
	if err != nil {
		return err
	}
	if err := batch.Append(w.Probe, s.cfg.Env, w.StartedAt, w.EndedAt, w.Kind,
		"statuspage", w.ExternalID, w.Reason, w.ConfirmedBy, del, time.Now().UTC()); err != nil {
		return err
	}
	return batch.Send()
}

// 7a — overlapping windows per (env, probe). leadInFrame never returns NULL
// at the end of the partition (it returns the type default, 1970), so the
// guard is rows_ahead, not IS NOT NULL.
const overlapsQuery = `
SELECT toInt64(count())
FROM (
    SELECT started_at AS a_start,
           ended_at   AS a_end,
           count() OVER w AS rows_ahead,
           leadInFrame(started_at) OVER w AS next_start
    FROM (
        SELECT probe, env, started_at,
               argMax(ended_at, updated_at) AS ended_at
        FROM incidents
        WHERE env = ?
        GROUP BY probe, env, started_at
        HAVING argMax(deleted, updated_at) = 0
    )
    WINDOW w AS (PARTITION BY env, probe ORDER BY started_at
                 ROWS BETWEEN CURRENT ROW AND UNBOUNDED FOLLOWING)
)
WHERE rows_ahead > 1 AND next_start <= a_end`

// 7b — orphan windows covering no failed run. No FINAL on incidents: the
// table is plain MergeTree and FINAL on it is ILLEGAL_FINAL.
const orphansQuery = `
SELECT toInt64(count())
FROM (
    SELECT i.probe, i.started_at
    FROM (
        SELECT env, probe, started_at,
               argMax(ended_at, updated_at) AS ended_at
        FROM incidents
        WHERE env = ?
        GROUP BY env, probe, started_at
        HAVING argMax(deleted, updated_at) = 0
    ) AS i
    LEFT JOIN (
        SELECT probe, started_at, outcome
        FROM runs
        WHERE env = ? AND started_at >= now() - toIntervalDay(?)
    ) AS r ON r.probe = i.probe
    WHERE i.started_at >= now() - toIntervalDay(?)
    GROUP BY i.probe, i.started_at, i.ended_at
    HAVING maxIf(1, r.outcome != 'pass'
                    AND r.started_at >= i.started_at
                    AND r.started_at <= i.ended_at) = 0
)`

// Invariants counts overlapping and orphan windows; both must be 0, and the
// caller turns anything else into a failed run so it pages.
func (s *Store) Invariants(ctx context.Context) (overlaps, orphans int, err error) {
	var n int64
	if err := s.conn.QueryRow(ctx, overlapsQuery, s.cfg.Env).Scan(&n); err != nil {
		return 0, 0, fmt.Errorf("overlap invariant: %w", err)
	}
	overlaps = int(n)

	if err := s.conn.QueryRow(ctx, orphansQuery, s.cfg.Env, s.cfg.Env,
		s.cfg.RunsRetentionDays, s.cfg.RunsRetentionDays).Scan(&n); err != nil {
		return 0, 0, fmt.Errorf("orphan invariant: %w", err)
	}
	orphans = int(n)
	return overlaps, orphans, nil
}
