#!/usr/bin/env python3
"""Read-only Antigravity usage/security audit via psql.

The script emits and optionally runs a set of PostgreSQL queries that reconcile
local Sub2 usage logs with Antigravity AI Credits snapshots. It never writes to
the database: the generated SQL runs inside a READ ONLY transaction.
"""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import textwrap
from datetime import datetime, timedelta, timezone


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Audit Antigravity usage mismatch signals in a Sub2API database.",
    )
    parser.add_argument(
        "--database-url",
        default=os.environ.get("DATABASE_URL", ""),
        help="PostgreSQL connection string. Defaults to DATABASE_URL.",
    )
    parser.add_argument(
        "--start",
        help="Audit window start, e.g. 2026-05-06T00:00:00+08:00. Defaults to 24h before --end.",
    )
    parser.add_argument(
        "--end",
        help="Audit window end, e.g. 2026-05-07T00:00:00+08:00. Defaults to now in UTC.",
    )
    parser.add_argument(
        "--top",
        type=int,
        default=50,
        help="Maximum rows per high-cardinality report section. Default: 50.",
    )
    parser.add_argument(
        "--sql-only",
        action="store_true",
        help="Print the generated SQL instead of running psql.",
    )
    return parser.parse_args()


def parse_timestamp(value: str | None, fallback: datetime) -> datetime:
    if not value:
        return fallback
    normalized = value.strip().replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError as exc:
        raise SystemExit(f"invalid timestamp {value!r}: {exc}") from exc
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed


def iso(dt: datetime) -> str:
    return dt.astimezone(timezone.utc).isoformat(timespec="seconds")


def sql_literal(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def generate_sql(start: str, end: str, top: int) -> str:
    start_lit = sql_literal(start)
    end_lit = sql_literal(end)
    top_lit = str(max(1, top))
    return textwrap.dedent(
        f"""
        \\pset pager off
        \\pset null '[null]'
        \\timing on

        BEGIN READ ONLY;

        \\echo ''
        \\echo '== Audit Window =='
        SELECT {start_lit}::timestamptz AS audit_start, {end_lit}::timestamptz AS audit_end;

        \\echo ''
        \\echo '== Antigravity Account Coverage =='
        SELECT
          COUNT(*) AS antigravity_accounts,
          COUNT(*) FILTER (WHERE status = 'active') AS active_accounts,
          COUNT(DISTINCT lower(trim(credentials->>'email'))) FILTER (WHERE nullif(trim(credentials->>'email'), '') IS NOT NULL) AS distinct_emails
        FROM accounts
        WHERE platform = 'antigravity'
          AND deleted_at IS NULL;

        \\echo ''
        \\echo '== Local Antigravity Usage By Account / API Key / Client =='
        SELECT
          a.id AS account_id,
          a.name AS account_name,
          lower(trim(a.credentials->>'email')) AS antigravity_email,
          k.id AS api_key_id,
          k.name AS api_key_name,
          u.email AS user_email,
          COALESCE(NULLIF(left(ul.user_agent, 160), ''), '[empty]') AS user_agent,
          COALESCE(NULLIF(ul.ip_address, ''), '[empty]') AS ip_address,
          COUNT(*) AS requests,
          SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens) AS tokens,
          ROUND(SUM(ul.total_cost)::numeric, 8) AS local_quota_cost,
          ROUND(SUM(ul.actual_cost)::numeric, 8) AS charged_cost,
          MIN(ul.created_at) AS first_seen,
          MAX(ul.created_at) AS last_seen
        FROM usage_logs ul
        JOIN accounts a ON a.id = ul.account_id
        JOIN api_keys k ON k.id = ul.api_key_id
        JOIN users u ON u.id = ul.user_id
        WHERE a.platform = 'antigravity'
          AND ul.created_at >= {start_lit}::timestamptz
          AND ul.created_at < {end_lit}::timestamptz
        GROUP BY a.id, a.name, lower(trim(a.credentials->>'email')), k.id, k.name, u.email, user_agent, ip_address
        ORDER BY requests DESC
        LIMIT {top_lit};

        \\echo ''
        \\echo '== AI Credits Consumption From Snapshots By Email =='
        WITH ordered AS (
          SELECT
            lower(trim(email)) AS email,
            credit_type,
            amount,
            captured_at,
            LAG(amount) OVER (PARTITION BY lower(trim(email)), credit_type ORDER BY captured_at) AS prev_amount
          FROM ai_credit_snapshots
          WHERE captured_at >= ({start_lit}::timestamptz - interval '30 minutes')
            AND captured_at < {end_lit}::timestamptz
        )
        SELECT
          email,
          credit_type,
          ROUND(SUM(GREATEST(prev_amount - amount, 0))::numeric, 6) AS credits_consumed,
          COUNT(*) AS samples,
          MIN(captured_at) AS first_sample,
          MAX(captured_at) AS last_sample
        FROM ordered
        WHERE captured_at >= {start_lit}::timestamptz
          AND captured_at < {end_lit}::timestamptz
        GROUP BY email, credit_type
        ORDER BY credits_consumed DESC, samples DESC
        LIMIT {top_lit};

        \\echo ''
        \\echo '== Credits vs Local Usage By Antigravity Email =='
        WITH local_usage AS (
          SELECT
            lower(trim(a.credentials->>'email')) AS email,
            COUNT(*) AS local_requests,
            ROUND(SUM(ul.total_cost)::numeric, 8) AS local_quota_cost,
            ROUND(SUM(ul.actual_cost)::numeric, 8) AS charged_cost,
            COUNT(DISTINCT ul.api_key_id) AS api_keys,
            COUNT(DISTINCT ul.ip_address) FILTER (WHERE nullif(ul.ip_address, '') IS NOT NULL) AS ip_count,
            COUNT(DISTINCT ul.user_agent) FILTER (WHERE nullif(ul.user_agent, '') IS NOT NULL) AS ua_count
          FROM usage_logs ul
          JOIN accounts a ON a.id = ul.account_id
          WHERE a.platform = 'antigravity'
            AND ul.created_at >= {start_lit}::timestamptz
            AND ul.created_at < {end_lit}::timestamptz
          GROUP BY lower(trim(a.credentials->>'email'))
        ),
        ordered AS (
          SELECT
            lower(trim(email)) AS email,
            credit_type,
            amount,
            captured_at,
            LAG(amount) OVER (PARTITION BY lower(trim(email)), credit_type ORDER BY captured_at) AS prev_amount
          FROM ai_credit_snapshots
          WHERE captured_at >= ({start_lit}::timestamptz - interval '30 minutes')
            AND captured_at < {end_lit}::timestamptz
        ),
        credit_usage AS (
          SELECT
            email,
            ROUND(SUM(GREATEST(prev_amount - amount, 0))::numeric, 6) AS credits_consumed,
            COUNT(*) AS snapshot_rows
          FROM ordered
          WHERE captured_at >= {start_lit}::timestamptz
            AND captured_at < {end_lit}::timestamptz
          GROUP BY email
        )
        SELECT
          COALESCE(c.email, l.email) AS email,
          COALESCE(c.credits_consumed, 0) AS credits_consumed,
          COALESCE(l.local_requests, 0) AS local_requests,
          COALESCE(l.local_quota_cost, 0) AS local_quota_cost,
          COALESCE(l.charged_cost, 0) AS charged_cost,
          COALESCE(l.api_keys, 0) AS api_keys,
          COALESCE(l.ip_count, 0) AS ip_count,
          COALESCE(l.ua_count, 0) AS ua_count,
          CASE
            WHEN COALESCE(c.credits_consumed, 0) > 0 THEN ROUND((COALESCE(l.local_requests, 0)::numeric / c.credits_consumed), 4)
            ELSE NULL
          END AS local_requests_per_credit
        FROM credit_usage c
        FULL OUTER JOIN local_usage l ON l.email = c.email
        WHERE COALESCE(c.credits_consumed, 0) > 0
           OR COALESCE(l.local_requests, 0) > 0
        ORDER BY credits_consumed DESC, local_requests DESC
        LIMIT {top_lit};

        \\echo ''
        \\echo '== API Keys With Many IPs / User-Agents =='
        SELECT
          k.id AS api_key_id,
          k.name AS api_key_name,
          u.email AS user_email,
          COUNT(*) AS requests,
          COUNT(DISTINCT ul.ip_address) FILTER (WHERE nullif(ul.ip_address, '') IS NOT NULL) AS ip_count,
          COUNT(DISTINCT ul.user_agent) FILTER (WHERE nullif(ul.user_agent, '') IS NOT NULL) AS ua_count,
          array_agg(DISTINCT ul.ip_address) FILTER (WHERE nullif(ul.ip_address, '') IS NOT NULL) AS ips,
          array_agg(DISTINCT left(ul.user_agent, 120)) FILTER (WHERE nullif(ul.user_agent, '') IS NOT NULL) AS user_agents
        FROM usage_logs ul
        JOIN accounts a ON a.id = ul.account_id
        JOIN api_keys k ON k.id = ul.api_key_id
        JOIN users u ON u.id = ul.user_id
        WHERE a.platform = 'antigravity'
          AND ul.created_at >= {start_lit}::timestamptz
          AND ul.created_at < {end_lit}::timestamptz
        GROUP BY k.id, k.name, u.email
        HAVING COUNT(DISTINCT ul.ip_address) FILTER (WHERE nullif(ul.ip_address, '') IS NOT NULL) > 1
            OR COUNT(DISTINCT ul.user_agent) FILTER (WHERE nullif(ul.user_agent, '') IS NOT NULL) > 1
        ORDER BY ip_count DESC, ua_count DESC, requests DESC
        LIMIT {top_lit};

        \\echo ''
        \\echo '== Duplicate usage_logs request_id/api_key_id Rows =='
        SELECT
          ul.api_key_id,
          ul.request_id,
          COUNT(*) AS duplicate_rows,
          MIN(ul.created_at) AS first_seen,
          MAX(ul.created_at) AS last_seen
        FROM usage_logs ul
        JOIN accounts a ON a.id = ul.account_id
        WHERE a.platform = 'antigravity'
          AND ul.created_at >= {start_lit}::timestamptz
          AND ul.created_at < {end_lit}::timestamptz
          AND nullif(ul.request_id, '') IS NOT NULL
        GROUP BY ul.api_key_id, ul.request_id
        HAVING COUNT(*) > 1
        ORDER BY duplicate_rows DESC, last_seen DESC
        LIMIT {top_lit};

        \\echo ''
        \\echo '== Usage Billing Dedup Rows By API Key =='
        SELECT
          d.api_key_id,
          k.name AS api_key_name,
          u.email AS user_email,
          COUNT(*) AS billed_requests,
          COUNT(DISTINCT d.request_fingerprint) AS distinct_fingerprints,
          MIN(d.created_at) AS first_seen,
          MAX(d.created_at) AS last_seen
        FROM usage_billing_dedup d
        LEFT JOIN api_keys k ON k.id = d.api_key_id
        LEFT JOIN users u ON u.id = k.user_id
        WHERE d.created_at >= {start_lit}::timestamptz
          AND d.created_at < {end_lit}::timestamptz
        GROUP BY d.api_key_id, k.name, u.email
        ORDER BY billed_requests DESC
        LIMIT {top_lit};

        \\echo ''
        \\echo '== Antigravity Usage Without User-Agent / IP =='
        SELECT
          COUNT(*) FILTER (WHERE nullif(user_agent, '') IS NULL) AS missing_user_agent,
          COUNT(*) FILTER (WHERE nullif(ip_address, '') IS NULL) AS missing_ip_address,
          COUNT(*) AS total_antigravity_usage_rows
        FROM usage_logs ul
        JOIN accounts a ON a.id = ul.account_id
        WHERE a.platform = 'antigravity'
          AND ul.created_at >= {start_lit}::timestamptz
          AND ul.created_at < {end_lit}::timestamptz;

        ROLLBACK;
        """
    ).strip() + "\n"


def run_psql(database_url: str, sql: str) -> int:
    psql = shutil.which("psql")
    if not psql:
        print("psql was not found on PATH. Re-run with --sql-only and execute the SQL on the server.", file=sys.stderr)
        return 127
    if not database_url:
        print("database URL missing. Pass --database-url or set DATABASE_URL.", file=sys.stderr)
        return 2
    proc = subprocess.run(
        [psql, "--no-psqlrc", "--set", "ON_ERROR_STOP=1", "--dbname", database_url],
        input=sql,
        text=True,
        check=False,
    )
    return proc.returncode


def main() -> int:
    args = parse_args()
    end_dt = parse_timestamp(args.end, datetime.now(timezone.utc))
    start_dt = parse_timestamp(args.start, end_dt - timedelta(hours=24))
    if end_dt <= start_dt:
        raise SystemExit("--end must be after --start")

    sql = generate_sql(iso(start_dt), iso(end_dt), args.top)
    if args.sql_only:
        print(sql, end="")
        return 0
    return run_psql(args.database_url, sql)


if __name__ == "__main__":
    raise SystemExit(main())
