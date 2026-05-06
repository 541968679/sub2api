# Antigravity AI Credits And Balance Cost Analysis

Date: 2026-05-06
Timezone: Asia/Shanghai
Source: production PostgreSQL, `usage_logs`, `accounts`, `users`, `ai_credit_snapshots`

## Background

Production Antigravity usage showed a sharp drop in balance revenue per AI
credit. The initial suspicion was account credential leakage because AI Credits
were consumed faster than expected.

During investigation we added and fixed the Antigravity credit curve so the
admin Usage page can compare:

- AI Credits consumed from `ai_credit_snapshots`
- Antigravity request count from `usage_logs`
- token volume
- local balance/quota cost
- derived ratios such as credits/request and balance/credit

Important fixes made before this analysis:

- Usage buckets were aligned to application timezone.
- Historical soft-deleted Antigravity accounts were included in usage
  aggregation.
- Credit snapshot deltas were attributed across the interval between previous
  and current snapshots to reduce sampling lag.

## Main Conclusion

The current evidence does not point to account leakage as the main cause.

The stronger explanation is:

> Antigravity official AI Credits are consumed heavily by cache-heavy large
> context requests, while Sub2API local balance billing prices
> `cache_read_tokens` much lower than normal input/write/output tokens. When
> cache read share and cache read size rise, AI Credits grow much faster than
> local balance cost, so balance revenue per AI credit falls sharply.

In formula form:

```text
balance_per_credit = local_balance_cost / antigravity_ai_credits
```

After cache-heavy traffic increased:

```text
Antigravity AI Credits/request increased sharply
Sub2API local balance/request increased only mildly
=> balance_per_credit dropped
```

## Data Availability

- Earliest `ai_credit_snapshots`: 2026-04-18 22:25:29 +08
- Earliest Antigravity `usage_logs`: 2026-04-14 09:31:18 +08

Reliable AI Credits/request analysis starts from 2026-04-19 or later. Data from
2026-04-18 is partial because credit sampling only began late that day.

## Daily Trend

Daily Antigravity usage from 2026-04-24 to 2026-05-06:

| Day | Calls | Credits | Credits/Call | Tokens/Call | Balance/Credit |
| --- | ---: | ---: | ---: | ---: | ---: |
| 04-24 | 4,616 | 3,065 | 0.66 | 15,368 | $0.08452 |
| 04-25 | 2,708 | 3,938 | 1.45 | 41,057 | $0.10547 |
| 04-26 | 2,017 | 2,559 | 1.27 | 52,877 | $0.17233 |
| 04-27 | 3,127 | 9,989 | 3.19 | 105,794 | $0.09771 |
| 04-28 | 3,185 | 9,995 | 3.14 | 93,121 | $0.13137 |
| 04-29 | 7,791 | 5,459 | 0.70 | 23,974 | $0.20251 |
| 04-30 | 5,488 | 3,209 | 0.58 | 33,251 | $0.33493 |
| 05-01 | 4,839 | 4,995 | 1.03 | 21,055 | $0.06333 |
| 05-02 | 6,655 | 7,230 | 1.09 | 27,744 | $0.05371 |
| 05-03 | 6,918 | 12,641 | 1.83 | 33,559 | $0.03174 |
| 05-04 | 2,570 | 12,296 | 4.78 | 88,856 | $0.02792 |
| 05-05 | 2,744 | 17,953 | 6.54 | 128,874 | $0.02899 |
| 05-06 | 2,484 | 10,768 | 4.33 | 83,221 | $0.03261 |

The major shift is visible from 2026-05-04 onward:

- Credits/request increased sharply.
- Tokens/request increased sharply.
- Balance/request increased only moderately.
- Balance/credit dropped to about $0.03.

## Period Comparison

Three-period split based on the cache behavior change:

| Period | Calls | Credits | Balance Cost | Balance/Credit | Balance/Call | Credits/Call | Tokens/Call | Cache Read/Call | Cache Read Share |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 04-24 to 04-29 | 23,444 | 35,015 | $4,509.94 | $0.12880 | $0.1924 | 1.494 | 47,047 | 22,020 | 46.8% |
| 04-30 to 05-03 | 23,900 | 28,075 | $2,180.71 | $0.07767 | $0.0912 | 1.175 | 29,337 | 20,964 | 71.5% |
| 05-04 to 05-06 | 7,798 | 41,017 | $1,214.95 | $0.02962 | $0.1558 | 5.260 | 101,143 | 93,646 | 92.6% |

Important interpretation:

- Large context requests existed before 2026-05-04.
- Cache read share had already increased by 2026-05-01 to 2026-05-03.
- The large deterioration after 2026-05-04 came from both very high cache read
  share and much larger cache read per request.

The scale matches closely:

```text
cache_read/request: 93,646 / 20,964 ~= 4.47x
credits/request:    5.260 / 1.175 ~= 4.48x
```

This is strong evidence that AI Credits/request increased in step with
cache_read/request.

## Pricing Evidence

Before 2026-05-04 versus after 2026-05-04:

| Period | Balance/Credit | Balance/Call | Credits/Call | Tokens/Call | Cache Read/Call | Cache Read Share | Cache Read $/MTok | Input/Write $/MTok | Output $/MTok |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| before 05-04 | $0.10377 | $0.1383 | 1.332 | 39,799 | 16,436 | 41.3% | $1.3077 | $4.5902 | $15.9469 |
| since 05-04 | $0.02962 | $0.1558 | 5.260 | 101,143 | 93,646 | 92.6% | $0.9494 | $5.9947 | $33.7557 |

Key point:

- Local balance/request increased only from $0.1383 to $0.1558.
- AI Credits/request increased from 1.332 to 5.260.
- Therefore local balance per AI credit dropped from $0.10377 to $0.02962.

This is not "balance did not grow at all"; it is "balance grew far less than AI
Credits."

## Counterfactual Checks

For 2026-05-04 to 2026-05-06:

| Scenario | Balance Cost | Balance/Credit |
| --- | ---: | ---: |
| Actual local billing | $1,214.95 | $0.02962 |
| If cache_read used pre-05-04 cache_read rate | $1,476.66 | $0.03600 |
| If cache_read was priced as normal input/write | $3,873.65 | $0.09444 |
| If all tokens used pre-05-04 blended token rate | $2,739.81 | $0.06680 |

The important counterfactual is cache read priced as normal input/write:

```text
$0.09444 balance/credit ~= $0.10377 pre-05-04 balance/credit
```

This supports the conclusion that the low local price of `cache_read_tokens`
explains most of the drop in balance/credit.

## User-Level Findings

The 2026-05-04 to 2026-05-06 cache read increase was not uniform across all
users. A small set of users accounted for most of the increase.

Comparison window:

- B: 2026-05-01 to 2026-05-03
- C: 2026-05-04 to 2026-05-06

Top positive cache_read delta contributors:

| User | B Calls | C Calls | B Cache Read/Call | C Cache Read/Call | Cache Read Delta | Positive Delta Share |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| fengshengak@gmail.com | 0 | 876 | - | 140,449 | 123,033,446 | 29.7% |
| admin@zerocode.kaynlab.com | 559 | 755 | 171,276 | 287,533 | 121,344,252 | 29.3% |
| 79712989@qq.com | 4 | 614 | 0 | 103,611 | 63,617,402 | 15.3% |
| yangchengwu2021@gmail.com | 2,311 | 2,523 | 70,458 | 82,442 | 45,172,025 | 10.9% |
| gybilly2023@gmail.com | 12,594 | 860 | 590 | 36,898 | 24,306,868 | 5.9% |
| 83598472@qq.com | 0 | 150 | - | 76,486 | 11,472,893 | 2.8% |

Top four users accounted for about 85.2% of positive cache_read delta.

### Model Mix For Top Users

For 2026-05-04 to 2026-05-06:

| User | Model | Calls | Avg Cache Read | Avg Tokens | Balance/Call | Total Cache Read |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| admin@zerocode.kaynlab.com | claude-opus-4-7 | 755 | 287,533 | 295,702 | $0.3256 | 217,087,326 |
| fengshengak@gmail.com | claude-opus-4-7 | 691 | 157,062 | 163,381 | $0.2334 | 108,530,137 |
| fengshengak@gmail.com | claude-sonnet-4-6 | 176 | 82,405 | 96,569 | $0.1110 | 14,503,309 |
| 79712989@qq.com | claude-opus-4-7 | 535 | 102,916 | 126,402 | $0.2848 | 55,059,809 |
| 79712989@qq.com | claude-opus-4-6 | 77 | 111,138 | 139,843 | $0.3253 | 8,557,593 |
| yangchengwu2021@gmail.com | claude-opus-4-6 | 2,523 | 82,442 | 86,468 | $0.1338 | 208,000,516 |

The increase is concentrated in Claude Opus/Sonnet large-context traffic.

## Today Since 09:00

For 2026-05-06 09:00 to about 16:26 +08:

| Window | Calls | Credits | Balance/Credit | Balance/Call | Credits/Call | Tokens/Call | Cache Read/Call | Cache Read Share |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 00:00 to 09:00 | 335 | 4,616 | $0.01425 | $0.1963 | 13.779 | 168,051 | 165,550 | 98.5% |
| 09:00 to current | 2,149 | 7,285 | $0.03917 | $0.1328 | 3.390 | 69,998 | 64,229 | 91.8% |

Since 09:00, the same mechanism remains active, but less severe than the early
morning period.

Top users since 09:00:

| User | Calls | Cache Read Total | Cache Read/Call | Tokens/Call | Balance/Call |
| --- | ---: | ---: | ---: | ---: | ---: |
| yangchengwu2021@gmail.com | 600 | 45,509,163 | 75,849 | 80,591 | $0.1356 |
| gybilly2023@gmail.com | 860 | 31,732,593 | 36,898 | 42,305 | $0.1063 |
| admin@zerocode.kaynlab.com | 42 | 19,075,999 | 454,190 | 477,335 | $0.6242 |
| fengshengak@gmail.com | 207 | 18,607,220 | 89,890 | 94,624 | $0.1975 |
| 83598472@qq.com | 150 | 11,472,893 | 76,486 | 84,231 | $0.0929 |

## Account Leakage Assessment

The main leakage signal would be:

```text
AI Credits consumed, but no corresponding Sub2API calls/tokens/cost
```

Current data does not show this as the main cause:

- Large recent AI Credit consumption has matching local `usage_logs`.
- High-consumption email/account windows have corresponding calls and large
  tokens.
- The mismatch is between Antigravity AI Credits and local balance pricing, not
  between credits and request existence.

Therefore the primary issue is low local monetization of cache-heavy
Antigravity usage, not proven credential leakage.

## Operational Implications

If the business goal is to recover Antigravity AI Credit cost, current
Antigravity local billing likely underprices cache-heavy traffic.

Possible follow-ups:

- Add Antigravity-specific cache_read pricing multiplier.
- Add a floor such as `effective_cost = max(current_actual_cost, observed_credit_cost)`.
- Track rolling `balance_per_credit` and alert when it falls below threshold.
- Add per-user and per-model dashboards for:
  - cache_read_share
  - cache_read/request
  - credits/request
  - balance/credit
- Add leakage alert:
  - `credits > X` and `calls = 0`, or
  - `credits > X` and `tokens/cost` near zero.

## Caveats

- AI Credit attribution is based on sampled balances. Even after interval
  attribution, it remains an approximation.
- `ai_credit_snapshots` starts on 2026-04-18 22:25 +08, so earlier credit
  comparisons are unavailable.
- Antigravity official AI Credits pricing is inferred from observed balance
  deltas; no direct official per-token credit formula was available in this
  investigation.

