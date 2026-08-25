-- Enable selectable C/K/p50 composite for zuoge only (email, not user_id).
-- Haley 2026-08-25 rec: N=10 K=4 C=2. Other users stay NULL = legacy p50-only.
-- Do not app-default 10/4/2 (or 20/6/3) for anyone else.
UPDATE user_smart_schedule_policies p
SET quality_sched_window_n = 10,
    quality_sched_max_slow_in_window = 4,
    quality_sched_max_consecutive_slow = 2
FROM users u
WHERE p.user_id = u.id
  AND u.email = 'zuoge85@gmail.com'
  AND p.quality_sched_window_n IS NULL
  AND p.quality_sched_max_slow_in_window IS NULL
  AND p.quality_sched_max_consecutive_slow IS NULL;
