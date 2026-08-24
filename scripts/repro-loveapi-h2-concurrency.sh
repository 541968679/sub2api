#!/usr/bin/env bash
# repro-loveapi-h2-concurrency.sh
#
# 从生产同一出站路径（主机公网 IP / 与 sub2api 同 SNAT，1730 无 proxy）
# 用 curl HTTP/2 多路复用模拟高并发打 LoveAPI，尽量贴近网关出站，而不是随意 curl。
#
# =============================================================================
# 复现了什么 / 复现不了什么
# =============================================================================
#
# 复现（对照代码）：
# - HTTPS + 默认 HTTP/2：backend/internal/repository/http_upstream.go
#   buildUpstreamTransport 不关 HTTP/2（TLS 指纹那条才 ForceAttemptHTTP2=false）。
#   无代理 = 直连；生产 config 无 connection_pool_isolation → 默认 account_proxy。
#   账号 1730 proxy_id 为空 → 缓存键 account:1730|proxy:direct，账号级单一 client。
# - 出站 URL / 头：backend/internal/service/openai_gateway_service.go
#   API Key + 自定义 base_url → buildOpenAIResponsesURL / buildOpenAIChatCompletionsURL。
#   头：Authorization: Bearer <credentials.api_key>、Content-Type: application/json、
#   Accept: application/json（同步）或 text/event-stream（stream）。
#   OpenAI-Beta 只在 OAuth/Codex 路径默认补；1730 是 apikey，默认不带（可用 --openai-beta）。
# - 生产 1730（loveapi 009）近 7 天 usage_logs 上游几乎全是 /v1/responses
#   （入站 /v1/chat/completions 也会被 auto+responses_supported=true 转到 Responses）。
#   因此本脚本默认 --endpoint responses。--endpoint chat 仍可用。
# - 对端 SETTINGS MAX_CONCURRENT_STREAMS=128：必须把 120+ 路复用在同一条 H2 连接上。
#   默认 --mode multiplex（单进程 curl --parallel --http2）。
#   --mode fork 会各开连接，打不到 stream 上限，只适合对照。
#
# 复现不了：
# - 不经过本网关：无选号 / 粘性会话 / 并发槽 / pair / 计费 / usage_logs / failover。
# - 不是 Go net/http Transport（连接池、ResponseHeaderTimeout 默认 300s 等）。
# - 入站 CC→Responses 的完整转换 body（这里用小 Responses/CC payload）。
# - 账号 header_overrides（credentials.header_overrides）；需要时用 --header。
# - sub2api 生产镜像是 Alpine，无 curl/bash；不要 docker exec 进主容器跑本脚本。
#
# =============================================================================
# 生产怎么跑（不要把 key 打到终端 / 聊天 / 仓库）
# =============================================================================
#
# 1) 本机把脚本拷到生产机（Windows 示例）：
#    scp -i $HOME\.ssh\id_ed25519_sub2api `
#      scripts/repro-loveapi-h2-concurrency.sh `
#      root@172.245.247.80:/opt/sub2api/repro-loveapi-h2-concurrency.sh
#
# 2) SSH 上生产机后，只检查 1730 元数据（禁止 SELECT 完整 api_key）：
#    docker exec sub2api-postgres psql -U sub2api -d sub2api -c \
#      "SELECT id, name, platform, type, proxy_id, concurrency,
#              length(COALESCE(credentials->>'api_key','')) AS api_key_len,
#              credentials ? 'api_key' AS has_api_key,
#              credentials->>'base_url' AS base_url,
#              extra->>'openai_responses_mode' AS openai_responses_mode,
#              extra->>'openai_responses_supported' AS openai_responses_supported
#       FROM accounts WHERE id = 1730 AND deleted_at IS NULL;"
#
#    近 7 天上游端点计数（无密钥）：
#    docker exec sub2api-postgres psql -U sub2api -d sub2api -c \
#      "SELECT COALESCE(upstream_endpoint,'(null)'),
#              COALESCE(inbound_endpoint,'(null)'), count(*)
#       FROM usage_logs
#       WHERE account_id = 1730 AND created_at > now() - interval '7 days'
#       GROUP BY 1, 2 ORDER BY 3 DESC;"
#
# 3) 只在生产机把 key 注入当前 shell（禁止 echo "$LOVEAPI_API_KEY"、禁止 set -x）：
#    export LOVEAPI_API_KEY="$(docker exec sub2api-postgres \
#      psql -U sub2api -d sub2api -tA -c \
#      "SELECT credentials->>'api_key' FROM accounts WHERE id=1730 AND deleted_at IS NULL LIMIT 1;")"
#    LOVEAPI_API_KEY="${LOVEAPI_API_KEY//$'\r'/}"
#    LOVEAPI_API_KEY="${LOVEAPI_API_KEY//$'\n'/}"
#    export LOVEAPI_API_KEY
#    printf 'LOVEAPI_API_KEY_LEN=%s\n' "${#LOVEAPI_API_KEY}"
#    export LOVEAPI_BASE_URL='https://link.loveapi.org'
#
# 4) 先语法/计划（默认不发请求）：
#    bash -n /opt/sub2api/repro-loveapi-h2-concurrency.sh
#    bash /opt/sub2api/repro-loveapi-h2-concurrency.sh --help
#    bash /opt/sub2api/repro-loveapi-h2-concurrency.sh --dry-run
#
# 5) 真打必须显式 --run。默认 130 路会顶到对端 128 stream 上限。
#    未得到明确许可前不要加 --run。
#    同步小包（默认）：
#      bash /opt/sub2api/repro-loveapi-h2-concurrency.sh --run
#    复现 8–18 分钟干等：把 --timeout 加到 600–1200。
#    流式：
#      bash /opt/sub2api/repro-loveapi-h2-concurrency.sh --run --stream true
#
# 6) 更贴近网关进程网络命名空间（可选，需能拉 curl 镜像）：
#    docker run --rm --network container:sub2api \
#      -e LOVEAPI_API_KEY -e LOVEAPI_BASE_URL \
#      -v /opt/sub2api/repro-loveapi-h2-concurrency.sh:/repro.sh:ro \
#      curlimages/curl:8.11.1 sh /repro.sh --dry-run
#
# 默认参数：concurrency=130 timeout=180s requests=130 stream=false
#           endpoint=responses model=gpt-5.6-sol max_tokens=16 mode=multiplex
#
# =============================================================================

set -eu
# 禁止 set -x：会把 Authorization / LOVEAPI_API_KEY 打到终端。
set +x

SCRIPT_NAME="$(basename "$0")"

CONCURRENCY=130
TIMEOUT_SEC=180
REQUESTS=""
DURATION_SEC=0
STREAM="false"
ENDPOINT="responses"
MODEL="gpt-5.6-sol"
MAX_TOKENS=16
MODE="multiplex"
HTTP_VERSION="2"
OPENAI_BETA=0
RUN=0
DRY_RUN=0
PROBE=0
CONNECT_TIMEOUT=30
OUT_DIR=""
USER_AGENT="sub2api-repro-loveapi-h2/1.0"
EXTRA_HEADERS=()
BASE_URL="${LOVEAPI_BASE_URL:-https://link.loveapi.org}"

usage() {
  cat <<'EOF'
用法:
  repro-loveapi-h2-concurrency.sh [选项]

环境变量:
  LOVEAPI_API_KEY    上游 OpenAI 兼容 API key（必填才能 --run / --probe）
  LOVEAPI_BASE_URL   默认 https://link.loveapi.org（账号 credentials.base_url）

选项:
  --concurrency N        同时在途路数，默认 130（对端 H2 SETTINGS 128，需 120+）
  --timeout SEC          每路 curl --max-time，默认 180
                         真实慢单常 8–18 分钟；要复现干等请 --timeout 600 或 1200
  --requests N           总请求数，默认等于 concurrency（一轮）
  --duration SEC         按秒循环发波，0=不循环（多波会新建 H2 连接，不适合打 stream 上限）
  --stream true|false    默认 false（同步 JSON，对应生产 stream:false）
  --endpoint responses|chat
                         默认 responses（生产 1730 实际上游几乎全是 /v1/responses）
  --model NAME           默认 gpt-5.6-sol
  --max-tokens N         小包，默认 16（CC=max_tokens，Responses=max_output_tokens）
  --mode multiplex|fork  默认 multiplex（单连接多路复用，才能顶 128 streams）
                         fork=每路独立 curl，打不到 MAX_CONCURRENT_STREAMS
  --http1.1              对照：强制 HTTP/1.1（不会复用 H2 stream）
  --openai-beta          加上 OpenAI-Beta: responses=experimental（1730 apikey 默认没有）
  --header 'Name: value' 额外请求头，可重复（账号 header_overrides 不会自动带）
  --user-agent STR       默认 sub2api-repro-loveapi-h2/1.0，便于上游日志过滤
  --out DIR              timing CSV 目录（不写响应 body）
  --dry-run              只打印计划，不发请求（默认；可不写）
  --run                  真的发请求（必须显式）
  --probe                只发 1 路，用于确认 key/端点（仍会打到 LoveAPI）
  -h, --help             本帮助

安全:
  响应 body 丢弃（-o /dev/null）。stdout 只有状态码和 timing。
  禁止: echo "$LOVEAPI_API_KEY"、set -x、curl -v、把 --config 文件 cat 出来。

示例（先 dry-run，Brandon 明确说跑再 --run）:
  LOVEAPI_BASE_URL=https://link.loveapi.org \\
    ./repro-loveapi-h2-concurrency.sh --dry-run
  ./repro-loveapi-h2-concurrency.sh --run --concurrency 130 --timeout 180
  ./repro-loveapi-h2-concurrency.sh --run --stream true --timeout 600
EOF
}

die() {
  printf '%s\n' "error: $*" >&2
  exit 2
}

trim() {
  # trim CR/LF/space without printing the value
  local s="$1"
  s="${s//$'\r'/}"
  s="${s//$'\n'/}"
  # leading / trailing spaces
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

is_true() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

base_has_version_suffix() {
  # 对齐 openai_endpoint_url.go openAIBaseURLHasVersionSuffix
  local raw="${1%/}"
  local path=""
  case "$raw" in
    https://*|http://*)
      path="${raw#*://}"
      path="${path#*/}"
      if [ "$path" = "${raw#*://}" ]; then
        path=""
      else
        path="/$path"
      fi
      ;;
    */*) path="${raw#"${raw%%/*}"}" ;;
    *) path="" ;;
  esac
  path="${path%/}"
  [ -n "$path" ] || return 1
  local segment="${path##*/}"
  local rest
  case "$segment" in
    v[0-9]*)
      rest="${segment#v}"
      case "$rest" in
        *[!0-9]*)
          case "$rest" in
            [0-9]*.[0-9]*)
              local a="${rest%%.*}"
              local b="${rest#*.}"
              case "$a$b" in
                *[!0-9]*) return 1 ;;
                *) return 0 ;;
              esac
              ;;
            [0-9]*alpha*|[0-9]*beta*|[0-9]*preview*) return 0 ;;
            *) return 1 ;;
          esac
          ;;
        *) return 0 ;;
      esac
      ;;
    *) return 1 ;;
  esac
}

join_openai_url() {
  # 对齐 buildOpenAIEndpointURL(base, endpoint)
  local base endpoint relative
  base="$(trim "$1")"
  endpoint="$(trim "$2")"
  base="${base%/}"
  case "$endpoint" in
    /*) ;;
    *) endpoint="/$endpoint" ;;
  esac
  relative="${endpoint#/v1}"
  case "$base" in
    *"$endpoint"|*"$relative")
      printf '%s' "$base"
      return 0
      ;;
  esac
  if base_has_version_suffix "$base"; then
    printf '%s%s' "$base" "$relative"
  else
    printf '%s%s' "$base" "$endpoint"
  fi
}

wipe_file() {
  local f="$1"
  [ -n "$f" ] && [ -f "$f" ] || return 0
  if command -v shred >/dev/null 2>&1; then
    shred -u "$f" 2>/dev/null || rm -f "$f"
  else
    dd if=/dev/zero of="$f" bs=4096 count=1 conv=notrunc >/dev/null 2>&1 || true
    rm -f "$f"
  fi
}

TMP_DIR=""
CFG=""
BODY=""
RESULT=""
cleanup() {
  wipe_file "${CFG:-}"
  wipe_file "${BODY:-}"
  if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      -h|--help)
        usage
        exit 0
        ;;
      --concurrency) CONCURRENCY="$2"; shift 2 ;;
      --concurrency=*) CONCURRENCY="${1#*=}"; shift ;;
      --timeout) TIMEOUT_SEC="$2"; shift 2 ;;
      --timeout=*) TIMEOUT_SEC="${1#*=}"; shift ;;
      --requests) REQUESTS="$2"; shift 2 ;;
      --requests=*) REQUESTS="${1#*=}"; shift ;;
      --duration) DURATION_SEC="$2"; shift 2 ;;
      --duration=*) DURATION_SEC="${1#*=}"; shift ;;
      --stream) STREAM="$2"; shift 2 ;;
      --stream=*) STREAM="${1#*=}"; shift ;;
      --endpoint) ENDPOINT="$2"; shift 2 ;;
      --endpoint=*) ENDPOINT="${1#*=}"; shift ;;
      --model) MODEL="$2"; shift 2 ;;
      --model=*) MODEL="${1#*=}"; shift ;;
      --max-tokens|--max-output-tokens) MAX_TOKENS="$2"; shift 2 ;;
      --max-tokens=*|--max-output-tokens=*) MAX_TOKENS="${1#*=}"; shift ;;
      --mode) MODE="$2"; shift 2 ;;
      --mode=*) MODE="${1#*=}"; shift ;;
      --http1.1) HTTP_VERSION="1.1"; shift ;;
      --openai-beta) OPENAI_BETA=1; shift ;;
      --header) EXTRA_HEADERS+=("$2"); shift 2 ;;
      --header=*) EXTRA_HEADERS+=("${1#*=}"); shift ;;
      --user-agent) USER_AGENT="$2"; shift 2 ;;
      --user-agent=*) USER_AGENT="${1#*=}"; shift ;;
      --out) OUT_DIR="$2"; shift 2 ;;
      --out=*) OUT_DIR="${1#*=}"; shift ;;
      --dry-run) DRY_RUN=1; shift ;;
      --run) RUN=1; shift ;;
      --probe) PROBE=1; shift ;;
      -v|--verbose)
        die "拒绝 $1：会把 Authorization 打到终端。只用 timing 输出"
        ;;
      --)
        shift
        break
        ;;
      -*)
        die "未知参数: $1（--help 看用法）"
        ;;
      *)
        die "多余参数: $1"
        ;;
    esac
  done
}

validate_args() {
  case "$ENDPOINT" in
    responses|chat|chat_completions|cc) ;;
    *) die "--endpoint 只能是 responses 或 chat" ;;
  esac
  case "$ENDPOINT" in
    chat_completions|cc) ENDPOINT="chat" ;;
  esac
  case "$MODE" in
    multiplex|fork) ;;
    *) die "--mode 只能是 multiplex 或 fork" ;;
  esac
  case "$CONCURRENCY" in
    ''|*[!0-9]*) die "--concurrency 必须是正整数" ;;
  esac
  [ "$CONCURRENCY" -ge 1 ] || die "--concurrency 必须 >= 1"
  case "$TIMEOUT_SEC" in
    ''|*[!0-9]*) die "--timeout 必须是正整数秒" ;;
  esac
  [ "$TIMEOUT_SEC" -ge 1 ] || die "--timeout 必须 >= 1"
  case "$MAX_TOKENS" in
    ''|*[!0-9]*) die "--max-tokens 必须是正整数" ;;
  esac
  [ "$MAX_TOKENS" -ge 1 ] || die "--max-tokens 必须 >= 1"
  case "$DURATION_SEC" in
    ''|*[!0-9]*) die "--duration 必须是整数秒" ;;
  esac
  if [ -n "$REQUESTS" ]; then
    case "$REQUESTS" in
      ''|*[!0-9]*) die "--requests 必须是正整数" ;;
    esac
    [ "$REQUESTS" -ge 1 ] || die "--requests 必须 >= 1"
  fi
  if is_true "$STREAM"; then
    STREAM="true"
  else
    STREAM="false"
  fi
  if [ "$PROBE" -eq 1 ]; then
    RUN=1
    CONCURRENCY=1
    REQUESTS=1
    DURATION_SEC=0
  fi
  if [ -z "$REQUESTS" ]; then
    REQUESTS="$CONCURRENCY"
  fi
  if [ "$RUN" -eq 1 ] && [ "$DRY_RUN" -eq 1 ]; then
    die "不要同时 --run 和 --dry-run"
  fi
}

resolve_target() {
  local path
  if [ "$ENDPOINT" = "chat" ]; then
    path="/v1/chat/completions"
  else
    path="/v1/responses"
  fi
  TARGET_URL="$(join_openai_url "$BASE_URL" "$path")"
  if [ "$STREAM" = "true" ]; then
    ACCEPT="text/event-stream"
  else
    ACCEPT="application/json"
  fi
}

write_body() {
  BODY="$TMP_DIR/body.json"
  if [ "$ENDPOINT" = "chat" ]; then
    cat >"$BODY" <<EOF
{"model":"${MODEL}","messages":[{"role":"user","content":"hello"}],"max_tokens":${MAX_TOKENS},"stream":${STREAM}}
EOF
  else
    cat >"$BODY" <<EOF
{"model":"${MODEL}","input":"hello","max_output_tokens":${MAX_TOKENS},"stream":${STREAM}}
EOF
  fi
}

curl_http_opt_line() {
  if [ "$HTTP_VERSION" = "1.1" ]; then
    printf '%s\n' "http1.1"
  else
    printf '%s\n' "http2"
  fi
}

write_curl_config() {
  local n="$1"
  local i h
  CFG="$TMP_DIR/curl.cfg"
  umask 077
  : >"$CFG"
  chmod 600 "$CFG"
  for i in $(seq 1 "$n"); do
    if [ "$i" -gt 1 ]; then
      printf 'next\n' >>"$CFG"
    fi
    {
      printf 'url = "%s"\n' "$TARGET_URL"
      printf 'request = "POST"\n'
      printf 'data = "@%s"\n' "$BODY"
      printf 'user-agent = "%s"\n' "$USER_AGENT"
      printf 'header = "Authorization: Bearer %s"\n' "$LOVEAPI_API_KEY"
      printf 'header = "Content-Type: application/json"\n'
      printf 'header = "Accept: %s"\n' "$ACCEPT"
      if [ "$OPENAI_BETA" -eq 1 ]; then
        printf 'header = "OpenAI-Beta: responses=experimental"\n'
      fi
      for h in "${EXTRA_HEADERS[@]+"${EXTRA_HEADERS[@]}"}"; do
        printf 'header = "%s"\n' "$h"
      done
      printf 'output = "/dev/null"\n'
      printf 'silent\n'
      printf 'show-error\n'
      printf 'compressed\n'
      curl_http_opt_line
      printf 'max-time = %s\n' "$TIMEOUT_SEC"
      printf 'connect-timeout = %s\n' "$CONNECT_TIMEOUT"
      # idx http_code http_version ttfb total num_connects errormsg
      printf 'write-out = "%s\\t%%{http_code}\\t%%{http_version}\\t%%{time_starttransfer}\\t%%{time_total}\\t%%{num_connects}\\t%%{errormsg}\\n"\n' "$i"
    } >>"$CFG"
  done
}

summarize() {
  local file="$1"
  [ -s "$file" ] || {
    printf '%s\n' "没有 timing 行（全部失败或未启动）"
    return 0
  }
  awk -F '\t' -v timeout="$TIMEOUT_SEC" '
    function isort(a, n,    i, j, v) {
      for (i = 2; i <= n; i++) {
        v = a[i]
        j = i - 1
        while (j >= 1 && a[j] > v) { a[j+1] = a[j]; j-- }
        a[j+1] = v
      }
    }
    function pct(a, n, p,    idx) {
      if (n < 1) return 0
      idx = int((p / 100.0) * (n - 1) + 1)
      if (idx < 1) idx = 1
      if (idx > n) idx = n
      return a[idx]
    }
    {
      n++
      code = $2
      ver = $3
      ttfb = $4 + 0
      total = $5 + 0
      nconn += ($6 + 0)
      codes[code]++
      vers[ver]++
      ttfbs[n] = ttfb
      totals[n] = total
      ttfb_sum += ttfb
      total_sum += total
      if (n == 1 || ttfb < ttfb_min) ttfb_min = ttfb
      if (n == 1 || ttfb > ttfb_max) ttfb_max = ttfb
      if (n == 1 || total < total_min) total_min = total
      if (n == 1 || total > total_max) total_max = total
      if (ttfb > 60) ttfb_gt60++
      if (ttfb > 180) ttfb_gt180++
      if (total > 60) total_gt60++
      if (total > 180) total_gt180++
      if (code == "000" || $7 ~ /timeout|timed out|Timeout/) timeouts++
      if (total + 0 >= timeout) at_timeout++
    }
    END {
      if (n < 1) { print "rows=0"; exit }
      isort(ttfbs, n)
      isort(totals, n)
      printf "rows=%d  timeouts≈%d  hit_max_time≈%d\n", n, timeouts+0, at_timeout+0
      printf "header_s(time_starttransfer)  min=%.3f avg=%.3f p50=%.3f p95=%.3f max=%.3f  >60s=%d  >180s=%d\n", \
        ttfb_min, ttfb_sum/n, pct(ttfbs,n,50), pct(ttfbs,n,95), ttfb_max, ttfb_gt60+0, ttfb_gt180+0
      printf "total_s                        min=%.3f avg=%.3f p50=%.3f p95=%.3f max=%.3f  >60s=%d  >180s=%d\n", \
        total_min, total_sum/n, pct(totals,n,50), pct(totals,n,95), total_max, total_gt60+0, total_gt180+0
      printf "sum_num_connects=%d  (multiplex 期望接近 1；fork 期望接近请求数)\n", nconn
      printf "http_version:"
      for (v in vers) printf " %s=%d", v, vers[v]
      printf "\nhttp_code:"
      for (c in codes) printf " %s=%d", c, codes[c]
      printf "\n"
      peak = 0
      for (i = 1; i <= n; i++) if (totals[i] > 0) peak++
      printf "estimated_peak_in_flight=%d  (一轮并行启动；仍在途@60s=%d @180s=%d)\n", peak, total_gt60+0, total_gt180+0
    }
  ' "$file"
}

print_plan() {
  local key="${LOVEAPI_API_KEY-}"
  local key_len="${#key}"
  cat <<EOF
plan:
  target          $TARGET_URL
  endpoint        $ENDPOINT
  stream          $STREAM
  accept          $ACCEPT
  model           $MODEL
  max_tokens      $MAX_TOKENS
  concurrency     $CONCURRENCY
  requests        $REQUESTS
  duration_s      $DURATION_SEC
  timeout_s       $TIMEOUT_SEC  (干等 8-18min 请加大)
  mode            $MODE
  http            $HTTP_VERSION
  openai_beta     $OPENAI_BETA
  user_agent      $USER_AGENT
  extra_headers   ${#EXTRA_HEADERS[@]}
  LOVEAPI_API_KEY_LEN  $key_len
  note            默认不打印 body；config 含 Bearer，0600 且退出时 shred
EOF
  if [ "$MODE" = "fork" ]; then
    printf '%s\n' "  warn            fork 各开 TCP/TLS，通常打不到 MAX_CONCURRENT_STREAMS=128"
  fi
  if [ "$DURATION_SEC" -gt 0 ]; then
    printf '%s\n' "  warn            --duration 多波可能换新连接；打 stream 上限请一轮 --requests>=130"
  fi
  if [ "$CONCURRENCY" -ge 120 ] && [ "$HTTP_VERSION" = "2" ] && [ "$MODE" = "multiplex" ]; then
    printf '%s\n' "  expect          120+ 路同连接，用于顶对端 128 streams"
  fi
}

run_multiplex_wave() {
  local n="$1"
  local out="$2"
  write_curl_config "$n"
  local start_wall
  start_wall="$(date '+%Y-%m-%dT%H:%M:%S%z')"
  printf 'wave_start %s n=%s parallel-max=%s\n' "$start_wall" "$n" "$CONCURRENCY"
  # --parallel 必须出现在命令行；config 里的 next 才会并发。
  # 不把 key 放进 argv（在 --config 文件里）。
  set +e
  curl --parallel --parallel-immediate --parallel-max "$CONCURRENCY" --config "$CFG" >>"$out"
  local rc=$?
  set -e
  wipe_file "$CFG"
  CFG=""
  printf 'wave_end %s curl_exit=%s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" "$rc"
  return 0
}

run_fork_wave() {
  local n="$1"
  local out="$2"
  local i
  local start_wall
  start_wall="$(date '+%Y-%m-%dT%H:%M:%S%z')"
  printf 'wave_start %s n=%s fork_concurrency=%s\n' "$start_wall" "$n" "$CONCURRENCY"
  for i in $(seq 1 "$n"); do
    while [ "$(jobs -pr | wc -l)" -ge "$CONCURRENCY" ]; do
      sleep 0.05
    done
    (
      local hdrs=(
        -H "Authorization: Bearer ${LOVEAPI_API_KEY}"
        -H "Content-Type: application/json"
        -H "Accept: ${ACCEPT}"
        -A "$USER_AGENT"
      )
      if [ "$OPENAI_BETA" -eq 1 ]; then
        hdrs+=(-H "OpenAI-Beta: responses=experimental")
      fi
      local h
      for h in "${EXTRA_HEADERS[@]+"${EXTRA_HEADERS[@]}"}"; do
        hdrs+=(-H "$h")
      done
      local http_flag=(--http2)
      [ "$HTTP_VERSION" = "1.1" ] && http_flag=(--http1.1)
      set +e
      curl -sS "${http_flag[@]}" --compressed \
        --max-time "$TIMEOUT_SEC" --connect-timeout "$CONNECT_TIMEOUT" \
        -o /dev/null \
        -w "${i}\t%{http_code}\t%{http_version}\t%{time_starttransfer}\t%{time_total}\t%{num_connects}\t%{errormsg}\n" \
        "${hdrs[@]}" \
        --data-binary @"$BODY" \
        -X POST "$TARGET_URL"
      set -e
    ) >>"$out" &
    inflight=$((inflight + 1))
  done
  wait
  printf 'wave_end %s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')"
}

check_curl() {
  need_cmd curl
  if ! curl --version 2>/dev/null | head -n 1 | grep -qi curl; then
    die "curl 不可用"
  fi
  if [ "$HTTP_VERSION" = "2" ]; then
    if ! curl --version 2>/dev/null | grep -qi nghttp2; then
      die "本机 curl 未链 nghttp2，无法 --http2。生产主机 curl 8.5+ 通常可以"
    fi
  fi
  if [ "$MODE" = "multiplex" ]; then
    if ! curl --help 2>/dev/null | grep -q -- '--parallel'; then
      die "curl 太旧，没有 --parallel（需要 7.66+）。请升级或 --mode fork（打不到 stream 上限）"
    fi
  fi
}

main() {
  parse_args "$@"
  validate_args
  resolve_target

  if [ "$RUN" -eq 0 ]; then
    print_plan
    printf '%s\n' "dry-run: 未发请求。确认要打 LoveAPI 时再加 --run（或 --probe 只打 1 路）。"
    exit 0
  fi

  LOVEAPI_API_KEY="$(trim "${LOVEAPI_API_KEY:-}")"
  [ -n "$LOVEAPI_API_KEY" ] || die "未设置 LOVEAPI_API_KEY（只许从生产机 env 注入，禁止写进命令历史可见处）"
  printf 'LOVEAPI_API_KEY_LEN=%s\n' "${#LOVEAPI_API_KEY}"

  check_curl
  need_cmd date
  need_cmd awk
  need_cmd seq

  TMP_DIR="$(mktemp -d /tmp/loveapi-h2-repro.XXXXXX)"
  chmod 700 "$TMP_DIR"
  write_body

  if [ -z "$OUT_DIR" ]; then
    OUT_DIR="$TMP_DIR/out"
  fi
  mkdir -p "$OUT_DIR"
  RESULT="$OUT_DIR/timing.tsv"
  : >"$RESULT"
  chmod 600 "$RESULT" 2>/dev/null || true

  print_plan
  printf 'timing_tsv %s\n' "$RESULT"
  printf 'wall_start %s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')"

  local deadline=0
  if [ "$DURATION_SEC" -gt 0 ]; then
    deadline=$(( $(date +%s) + DURATION_SEC ))
  fi

  local wave=0
  while :; do
    wave=$((wave + 1))
    printf 'wave=%s\n' "$wave"
    if [ "$MODE" = "fork" ]; then
      run_fork_wave "$REQUESTS" "$RESULT"
    else
      run_multiplex_wave "$REQUESTS" "$RESULT"
    fi
    if [ "$DURATION_SEC" -le 0 ]; then
      break
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      break
    fi
  done

  printf 'wall_end %s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')"
  printf '%s\n' "---- summary ----"
  summarize "$RESULT"
  printf '%s\n' "timing 已写 $RESULT （无 body）。目录 $OUT_DIR 不会自动删除。"
}

main "$@"
