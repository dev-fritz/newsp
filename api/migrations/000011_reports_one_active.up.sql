-- A user has at most one pending or running report. The service checks first
-- for a friendly error; this index closes the race between concurrent requests
-- and the scheduler.

-- Make existing data compatible: keep the newest active report per user and
-- fail the older ones (same code the orphan reconciler uses), canceling their
-- jobs so a worker cannot bring them back to running.
WITH demoted AS (
    UPDATE reports SET status = 'failed', error = 'AI_FAILURE', completed_at = now()
    WHERE status IN ('pending', 'running')
    AND id NOT IN (
        SELECT DISTINCT ON (user_id) id
        FROM reports
        WHERE status IN ('pending', 'running')
        ORDER BY user_id, created_at DESC, id DESC
    )
    RETURNING id
)
UPDATE jobs SET status = 'canceled', finished_at = now(), updated_at = now()
WHERE status IN ('pending', 'running')
AND dedupe_key IN (SELECT 'report:' || id::text FROM demoted);

CREATE UNIQUE INDEX reports_one_active_per_user ON reports (user_id)
    WHERE status IN ('pending', 'running');
