# OpenAI Image URL Relay 4K Diagnostics

> Production diagnostics for OpenAI-compatible API-key image forwarding after
> forcing URL responses on `/v1/images/*`.

## Scope

- Date: 2026-06-30
- Production endpoint: `https://zerocode.kaynlab.com/v1/images/generations`
- Gateway version under test: Sub2API `v0.1.151`
- Model: `gpt-image-2`
- Size: `3840x2160`
- Quality: `high`
- Request shape: JSON `POST /v1/images/generations`
- Secret policy: do not record downstream API keys, upstream API keys, prompts in
  logs, image bytes, or base64 payloads in this document.

This record separates two different timing domains:

1. **API URL response latency**: downstream client -> Sub2API -> upstream image
   generation -> Sub2API returns a small JSON body containing `data[0].url`.
2. **Image file download latency**: downstream client downloads the PNG directly
   from the returned image URL host.

In URL response mode, Sub2API does not transfer image file bytes to the
downstream client. It returns the upstream-provided URL in a small JSON response.

## Production Fix Baseline

The `v0.1.151` hotfix forces API-key image requests to use
`response_format=url` upstream, including requests where the downstream client
explicitly sends `response_format=b64_json`.

Verified behavior:

- Explicit downstream `response_format=b64_json` no longer causes Sub2API to
  return `b64_json`.
- Production smoke response shape: `has_url=true`, `has_b64_json=false`,
  response body around hundreds of bytes for the small test and around 5.7 KB
  for the older storyboard 4K prompt.
- API response body read after headers is normally millisecond-scale once URL
  mode is active.

## Group Permission Finding

The new native 4K channel was initially blocked before account scheduling:

```text
HTTP 403 permission_error
Image generation is not enabled for this group
```

Production configuration check found:

| Group ID | Name | `allowed_models` | `allow_image_generation` before | Status |
|---:|---|---|---:|---|
| 24 | `gpt image 2` | `["gpt-image-2"]` | `true` | working |
| 36 | `gpt image 2 高质量` | `["gpt-image-2"]` | `false` | blocked |

The production database was updated for `groups.id=36`:

```text
allow_image_generation=true
updated_at=2026-06-30 14:01:41 +08
```

Redis/API-key auth cache invalidation was intentionally left to the operator in
that step. A later key smoke test confirmed image generation reached the
upstream and returned URL output.

## New Native 4K Channel Quality Smoke

Output directory:

```text
E:\cursor project\InvokeAI\tmp\zerocode-native-4k-new-channel\20260630-cg-toy-4k-quality
```

Single-image quality smoke result:

| Metric | Value |
|---|---:|
| HTTP status | `200` |
| API headers latency | `83.954s` |
| API body after headers | `0.002s` |
| API total latency | `83.956s` |
| JSON response size | `515 B` |
| Returned host | `cs.ydn99.com` |
| Image download latency | `37.101s` |
| Total with image download | `121.063s` |
| Image format | `PNG` |
| Image size | `3840x2160` |
| Image bytes | `9,407,346` |
| Response shape | `has_url=true`, `has_b64_json=false` |

Visual result:

- 2x2 contact sheet structure was present.
- The requested 3D toy / pastel / sunlight style was represented.
- Sunflower character with sunglasses and guitar, keyboard track, plush toy on
  butterfly, and low water-view establishing shot were all present.
- The bottom label text was visually dense and small, but the overall image was
  usable for quality validation.

## New Native 4K Channel Concurrency Baseline

Output directory:

```text
E:\cursor project\InvokeAI\tmp\zerocode-native-4k-new-channel\20260630-cg-toy-4k-concurrency-c2-c4-c8
```

Prompt and request shape were the same as the quality smoke. Each successful
request also downloaded the returned PNG and saved it as `original.png`.

| Concurrency | Success | Batch wall | API total min/avg/max | JSON body max | Image download min/avg/max | Avg JSON size | Avg image size | URL/base64 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| c2 | 2/2 | `188.362s` | `88.299 / 133.761 / 179.222s` | `2.466s` | `9.081 / 16.573 / 24.065s` | `515 B` | `10.14 MB` | `2 / 0` |
| c4 | 4/4 | `252.535s` | `88.085 / 115.389 / 194.588s` | `1.436s` | `20.206 / 48.343 / 60.695s` | `515 B` | `10.03 MB` | `4 / 0` |
| c8 | 8/8 | `241.133s` | `78.161 / 104.036 / 180.217s` | `1.489s` | `8.598 / 24.907 / 60.907s` | `515 B` | `9.66 MB` | `8 / 0` |

Findings:

- The new native 4K channel handled `c8` without failures.
- All returned images were `3840x2160` PNG.
- All successful API responses were URL-only; no base64 image payloads were
  returned.
- The API tail was dominated by time before response headers, meaning upstream
  generation or upstream-side queueing.
- Image URL downloads from `cs.ydn99.com` had visible but lower tail latency
  than earlier Japan-hosted URL observations.

## Japan Proxy Detailed Timing Run

The operator switched the downstream test environment to a Japan proxy server.
The detailed timing run used `curl.exe` instead of Node `fetch`, because Node
`fetch` was observed to bypass the local system proxy in this Windows
environment.

Network probe before each run:

```text
ip=35.213.12.141
city=Tokyo
country=JP
org=AS19527 Google LLC
```

`curl` reported `remote_ip=127.0.0.1` for proxied requests, which is the local
proxy endpoint. The external exit was verified by the `ipinfo.io` probe above.

Output directories:

```text
E:\cursor project\InvokeAI\tmp\zerocode-native-4k-new-channel\20260630-cg-toy-4k-jp-proxy-c2-c8-c16-detailed
E:\cursor project\InvokeAI\tmp\zerocode-native-4k-new-channel\20260630-cg-toy-4k-jp-proxy-c8-c16-proxy-only
```

The `c2` result came from the detailed run. The `c8` and `c16` results came
from the proxy-only run to avoid auxiliary direct-download controls affecting
batch wall time.

| Concurrency | API success | Proxy image success | Batch wall | API total avg/max | API pre-body avg/max | API body max | Image total avg/max | Image pre-body avg/max | Image body avg/max | Body throughput avg/min | Avg image bytes | Host | URL/base64 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| c2 | 2/2 | 2/2 | `110.174s` | `107.007 / 107.857s` | `107.005 / 107.856s` | `0.004s` | `2.308 / 2.417s` | `0.977 / 1.093s` | `1.331 / 1.338s` | `6.735 / 6.577 MiB/s` | `9,400,641 B` | `file.kayops.com` | `2 / 0` |
| c8 | 8/8 | 8/8 | `123.582s` | `95.613 / 118.742s` | `95.612 / 118.740s` | `0.004s` | `6.440 / 13.406s` | `4.481 / 8.065s` | `1.959 / 5.340s` | `5.749 / 1.836 MiB/s` | `9,716,372 B` | `cs.ydn99.com` | `8 / 0` |
| c16 | 16/16 | 16/16 | `150.330s` | `108.269 / 145.275s` | `108.267 / 145.274s` | `0.005s` | `7.170 / 15.702s` | `5.148 / 10.904s` | `2.022 / 5.542s` | `5.705 / 1.631 MiB/s` | `9,500,386 B` | `cs.ydn99.com` | `16 / 0` |

Findings:

- All `c2`, `c8`, and `c16` API requests succeeded and returned URL-only
  responses. No `b64_json` payload was returned.
- The API response body transfer was effectively negligible (`<=0.005s` max).
  The API latency was almost entirely pre-body time, so the dominant cost is
  before Sub2API returns the URL: upstream generation, upstream queueing, or
  upstream response wait.
- Proxy image downloads were successful in the final runs. Average full image
  download time was `2.308s` at `c2`, `6.440s` at `c8`, and `7.170s` at `c16`.
- Image download has some tail latency, mostly before image body transfer:
  `c16` image pre-body max was `10.904s`, while body max was `5.542s`.
- Body throughput was generally healthy for 4K PNGs of roughly 9-10 MB:
  average body throughput stayed around `5.7-6.7 MiB/s`.
- Batch wall time was driven by API pre-body tail, not by final PNG transfer.
  For `c16`, the API max was `145.275s` and the whole batch finished in
  `150.330s`.

Operational note:

- A preliminary direct-control diagnostic saw one generated image URL return
  proxy HTTP `502` while the same URL could be downloaded without the proxy.
  The final proxy-only `c8` and `c16` runs did not reproduce that failure, but
  future diagnostics should continue recording per-request image HTTP status,
  host, pre-body time, body time, and throughput.

## Hajimi Native 4K Channel Test

Date: 2026-07-01

Upstream endpoint under test:

```text
https://hajimicc.top/v1/images/generations
```

The upstream key is intentionally not recorded. The same long 4K storyboard
prompt, `gpt-image-2`, `3840x2160`, `quality=high`, and
`response_format=url` were used.

Network probe:

```text
ip=35.212.230.234
city=The Dalles
region=Oregon
country=US
org=AS19527 Google LLC
```

The operator set a 4-minute limit during testing. The script then used
`240000ms` as the per-network-stage timeout for API and image download calls.
For interpretation, strict end-to-end completion within 240 seconds is also
called out below.

Output directories:

```text
E:\cursor project\InvokeAI\tmp\hajimicc-native-4k\20260701-cg-toy-4k-quality-smoke-4min
E:\cursor project\InvokeAI\tmp\hajimicc-native-4k\20260701-cg-toy-4k-concurrency-c2-c8-c16-4min
```

Quality smoke:

| Metric | Value |
|---|---:|
| Success | `1/1` |
| API total | `100.126s` |
| API pre-body | `100.118s` |
| API body | `0.008s` |
| Image download total | `31.629s` |
| Image pre-body | `4.046s` |
| Image body | `27.582s` |
| Image body throughput | `0.344 MiB/s` |
| Image bytes | `9,945,533 B` |
| Image size | `3840x2160` |
| Image format | `PNG` |
| URL host | `www.geek2api.com` |
| URL/base64 | `1 / 0` |

Visual quality finding:

- The image followed the requested 2x2 contact-sheet format.
- `KF1` through `KF4` labels and the Chinese shot descriptions were readable
  on the 4K original.
- The scene-level English text on the arch (`SUNNY MELODY`) was also readable.
- Overall text clarity was better than the earlier native 4K test where labels
  were visibly dense and small.

Concurrency result:

| Concurrency | Success | Strict <=240s end-to-end | Batch wall | API total avg/max | Image total avg/max | Image body avg/max | Body throughput avg/min | Avg image bytes | URL/base64 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| c2 | 2/2 | 2/2 | `173.391s` | `116.746 / 143.961s` | `35.099 / 40.868s` | `32.807 / 38.899s` | `0.302 / 0.240 MiB/s` | `10,000,185 B` | `2 / 0` |
| c8 | 7/8 | 7/8 | `240.084s` | `100.022 / 177.273s` | `42.285 / 67.538s` | `39.847 / 65.192s` | `0.262 / 0.135 MiB/s` | `9,590,125 B` | `7 / 0` |
| c16 | 13/16 | 12/16 | `267.912s` | `104.236 / 176.193s` | `47.927 / 91.605s` | `45.290 / 89.806s` | `0.262 / 0.103 MiB/s` | `9,727,471 B` | `13 / 0` |

Findings:

- The channel can produce native `3840x2160` PNG output with URL-only
  responses and no base64 payload.
- Text clarity is good on the quality smoke image.
- API pre-body latency is in the same broad range as the previous native 4K
  channel: roughly 90-120 seconds for most successful requests, with long-tail
  API waits up to about 176-177 seconds in `c8`/`c16`.
- Image transfer is the weak point for this channel from the current US exit.
  Successful image downloads averaged `35-48s`, and the body transfer itself
  dominated that time.
- Download throughput was low for 9-10 MB PNGs: average body throughput stayed
  around `0.26-0.30 MiB/s`, with c16 minimum at `0.103 MiB/s`.
- Under a strict 240-second end-to-end limit, success was `2/2` at c2, `7/8`
  at c8, and `12/16` at c16.

## Hajimi Current-Exit Native vs Relay Retest

Date: 2026-07-01

The operator intended to switch the local exit to Hong Kong, but `curl.exe`
network probes still showed a Tokyo exit:

```text
ip=35.213.12.141
city=Tokyo
country=JP
org=AS19527 Google LLC
```

This retest therefore records the actual observed exit as Tokyo, not Hong Kong.
The image URL host remained `www.geek2api.com`.

Output directories:

```text
E:\cursor project\InvokeAI\tmp\hajimicc-native-4k\20260701-current-exit-native-relay-smoke
E:\cursor project\InvokeAI\tmp\hajimicc-native-4k\20260701-current-exit-native-relay-concurrency
```

Smoke result:

| Path | Success | API total | Image download | Body throughput | Host | URL/base64 |
|---|---:|---:|---:|---:|---|---:|
| native `hajimicc.top` | 1/1 | `109.688s` | `1.384s` | `12.312 MiB/s` | `www.geek2api.com` | `1 / 0` |
| relay `zerocode.kaynlab.com` | 1/1 | `91.496s` | `1.498s` | `10.424 MiB/s` | `www.geek2api.com` | `1 / 0` |

Native concurrency:

| Concurrency | Success | Strict <=240s end-to-end | Batch wall | API total avg/max | Image total avg/max | Body throughput avg/min | URL/base64 |
|---:|---:|---:|---:|---:|---:|---:|---:|
| c2 | 2/2 | 2/2 | `173.253s` | `130.318 / 171.805s` | `1.359 / 1.364s` | `11.826 / 11.293 MiB/s` | `2 / 0` |
| c8 | 7/8 | 7/8 | `240.188s` | `83.733 / 90.621s` | `1.326 / 1.372s` | `11.920 / 10.450 MiB/s` | `7 / 0` |
| c16 | 15/16 | 15/16 | `240.049s` | `106.374 / 145.000s` | `1.340 / 1.485s` | `11.668 / 10.124 MiB/s` | `15 / 0` |

Relay concurrency:

| Concurrency | Success | Strict <=240s end-to-end | Batch wall | API total avg/max | Image total avg/max | Body throughput avg/min | URL/base64 |
|---:|---:|---:|---:|---:|---:|---:|---:|
| c2 | 2/2 | 2/2 | `90.010s` | `86.477 / 88.392s` | `1.468 / 1.542s` | `10.147 / 9.767 MiB/s` | `2 / 0` |
| c8 | 8/8 | 8/8 | `181.586s` | `99.653 / 180.154s` | `1.547 / 1.698s` | `9.375 / 8.001 MiB/s` | `8 / 0` |
| c16 | 8/16 | 8/16 | `240.125s` | `92.159 / 110.092s` | `1.472 / 1.610s` | `10.059 / 8.813 MiB/s` | `8 / 0` |

Findings:

- Both native and relay paths returned URL-only `3840x2160` PNG output from
  `www.geek2api.com`; no base64 payloads were returned.
- Image URL download performance changed dramatically versus the prior US-exit
  run: image downloads dropped from `35-48s` averages to about `1.3-1.6s`, with
  body throughput around `8-12 MiB/s`.
- The remaining latency is API pre-body time, i.e. generation, queueing, or
  upstream response wait before URL return.
- Native c8 and c16 each had one API request time out at the 240-second limit.
- Relay c2/c8 were stable, but relay c16 returned six HTTP `429` failures and
  two API timeouts. The relay path therefore appears usable up to c8 in this
  run, while c16 hit upstream or relay-side concurrency/rate limiting.

## Hajimi Candidate Native 4K Key Check

Date: 2026-07-02

The operator provided a new native 4K candidate key for the same upstream
endpoint:

```text
base_url=https://hajimicc.top/
key_fingerprint=sk-aGU...um2FAD
```

The full key is stored only in the local ignored test-secret registry:

```text
E:\cursor project\api2sub\tmp\image-channel-secrets\native-4k-channels.local.json
```

That local registry currently tracks the tested native/upscale channels by
`base_url`, local channel id, and key fingerprint. It must not be committed or
copied into documentation.

Output directories:

```text
E:\cursor project\InvokeAI\tmp\hajimicc-native-4k\20260702-new-candidate-quality-concurrency
E:\cursor project\InvokeAI\tmp\hajimicc-native-4k\20260702-new-candidate-quality-concurrency\startprocess-concurrency-v2
```

Quality smoke result:

| Metric | Value |
|---|---:|
| Success | `0/1` |
| HTTP status | `503` |
| API total | `1.193s` |
| Response bytes | `211 B` |
| URL/base64 | `0 / 0` |
| Returned image host | not available |
| Error | `No available channel for model gpt-image-2 under group 4K-3（原生） (distributor)` |

Concurrency result:

| Concurrency | Success | Failed | Avg API time | Error |
|---:|---:|---:|---:|---|
| c2 | 0/2 | 2 | `1.287s` | `No available channel for model gpt-image-2 under group 4K-3（原生） (distributor)` |
| c4 | 0/4 | 4 | `0.901s` | same |
| c8 | 0/8 | 8 | `0.728s` | same |

Findings:

- The new candidate key is accepted by the upstream HTTP layer but cannot be
  scheduled to any available `gpt-image-2` channel in group `4K-3（原生）`.
- The failure happens before image generation, so no 4K sample image, image URL
  host, or text-clarity comparison can be produced from this key yet.
- Because no image URL is returned, a no-proxy direct download test for this
  candidate's own image host is not possible yet.
- This differs from the previously tested Hajimi native key, which returned
  URL-only `3840x2160` PNGs from `www.geek2api.com`.

## Image URL Host Direct-Access Probe

Date: 2026-07-02

Using the last relay-returned `www.geek2api.com/images/...png` URL, `curl.exe`
was tested with `--noproxy "*"` so the local `127.0.0.1:10808` proxy
environment was bypassed.

| Test | HTTP | Remote IP | First byte | Total | Bytes |
|---|---:|---|---:|---:|---:|
| Range `0-0`, no proxy | `206` | `156.254.17.67` | `10.692s` | `10.692s` | `1 B` |
| Full PNG download, no proxy | `200` | `156.254.17.67` | `10.200s` | `10.978s` | `8,478,197 B` |

`www.geek2api.com` continued to resolve to many `156.254.17.x` A records with
TTL `1s`. Direct no-proxy access from the local machine succeeded, but first
byte latency was around ten seconds in this sample. That is enough to explain
some downstream/client-side `502` or timeout wrappers even when the image host
is technically reachable.
