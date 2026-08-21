# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains guidelines for backend development. Fill in each file with your project's specific conventions.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | To fill |
| [Database Guidelines](./database-guidelines.md) | ORM patterns, queries, migrations | To fill |
| [Error Handling](./error-handling.md) | Error types, handling strategies | To fill |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | To fill |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging, log levels | To fill |
| [Display Token Pricing](./display-token-pricing.md) | L1/L2 usage display transform + B1 cache amplify (M) | Active |
| [OpenAI API-Key Upstream Routing](./openai-apikey-upstream-routing.md) | `(inbound, extra)` → Responses vs CC; passthrough; dual probe | Active |
| [Anthropic Messages SSE](./anthropic-messages-sse.md) | Responses→Anthropic SSE: `message_start` before `content_block_*`; empty compact 502 / `event: error` | Active |
| [Account User Schedule](./account-user-schedule.md) | Independent allow/deny/pair-cap/quality-gate plus user×platform smart-schedule composition | Active |
| [Account Quality Snapshots](./account-quality-snapshots.md) | last-N \(Q_a\) (site-wide N, default 20) persisted every 5m + history API | Active |
| [Account Quality Hard Close](./account-quality-hard-close.md) | Opt-in last-N pause via TempUnschedulableUntil (same stats as the grid) | Active |
| [Pool Mode Hard-Error Eviction](./pool-mode-hard-eviction.md) | Opt-in SetError for pool-mode billing/tenant-dead errors; default off | Active |
| [Schedule PnL](./schedule-pnl.md) | `true_cost` ingest + admin this-user×this-account schedule profit APIs | Active |

---

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: All documentation should be written in **English**.
