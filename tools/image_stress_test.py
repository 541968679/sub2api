#!/usr/bin/env python3
"""
Sub2API 图片生成压力测试脚本
================================

用途
----
针对客户反馈的 "通过 API 调用图片生成模型错误率高" 问题，
对 sub2api 网关的 Gemini image 生成接口做可复现的压测与错误分类。

支持两条入口代码路径：
  1. gemini-native   → POST /v1beta/models/{model}:generateContent
                       （也支持 :streamGenerateContent，--stream）
  2. anthropic-messages → POST /v1/messages（走 GeminiMessagesCompatService 翻译层）

输出
----
<out-dir>/
  run.json        —— 本次运行的参数快照
  requests.jsonl  —— 每个请求一行 JSON（含 X-Request-ID，用于关联服务端日志）
  summary.md      —— 人读的汇总报告（成功率、延迟分位数、错误分类、时间窗）

用法
----
    export SUB2API_KEY=sk-xxx          # 绝不要写进命令行
    python tools/image_stress_test.py \\
        --base-url https://zerocode.kaynlab.com \\
        --mode gemini-native \\
        --model gemini-3-pro-image \\
        --total 50 --concurrency 5 \\
        --image-size 2K

建议执行顺序（见 plan 文件 "执行计划" 一节）：
    冒烟 (1 req) → 基线 (concurrency=1) → 加压 (3/5/10) → 切模式 → 切模型 → 流式

依赖
----
  Python 3.10+
  pip install 'httpx[http2]'

关联服务端日志
----
  summary.md 会列出失败请求的 X-Request-ID，拿它到服务器查：
    python deploy/remote_exec.py 'docker logs sub2api --since 1h | grep <rid>'
"""

from __future__ import annotations

import argparse
import asyncio
import base64
import dataclasses
import hashlib
import json
import os
import random
import re
import signal
import statistics
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import httpx
except ImportError:
    print(
        "ERROR: httpx 未安装。请运行:  pip install 'httpx[http2]'",
        file=sys.stderr,
    )
    sys.exit(2)


# ──────────────────────────────────────────────────────────────
# 常量
# ──────────────────────────────────────────────────────────────

DEFAULT_BASE_URL = "https://zerocode.kaynlab.com"

# 稳定提示词集 —— 没有明显 safety 触发点，用于测量"健康状态下的错误率"
DEFAULT_PROMPTS: list[tuple[str, str]] = [
    ("cat-portrait", "A photorealistic portrait of a tabby cat sitting on a wooden bench"),
    ("red-apple", "A red apple on a white marble table, soft natural light"),
    ("cityscape", "Night view of a modern Asian city with neon signs and rain-slicked streets"),
    ("anime-girl", "Anime style illustration of a girl with blue hair holding a sunflower"),
    ("mountain", "A snow-capped mountain under a starry sky, long exposure photograph"),
    ("text-sign", 'A wooden sign that reads "Welcome" hanging on a cafe door'),
    ("geometric", "Minimalist geometric composition of red triangles and blue circles"),
    ("food-ramen", "A steaming bowl of tonkotsu ramen with chashu and soft-boiled egg"),
    ("robot-toy", "A small cute robot toy made of brushed aluminum"),
    ("watercolor-flower", "Watercolor painting of cherry blossoms against a pale blue sky"),
]

# 压力词集：历史上容易出问题的边界 case
STRESS_EXTRA_PROMPTS: list[tuple[str, str]] = [
    (
        "long-prompt",
        "An extremely detailed fantasy illustration depicting a dragon perched on a crumbling "
        "stone tower in the middle of a vast mountain range at sunset. The dragon has iridescent "
        "scales that catch the dying light, ember-red eyes, and smoke curling from its nostrils. "
        "Below the tower, a winding river glitters through a dense pine forest. In the far "
        "distance, a castle city is visible with thin columns of chimney smoke rising into the "
        "orange and purple sky. Birds of various species fly in formation near the clouds. "
        "The style should be reminiscent of classical oil paintings with hyperrealistic detail "
        "and dramatic cinematic lighting, high dynamic range, 8k quality, no watermarks.",
    ),
    ("mixed-cn-en", "一只橘色小猫 (an orange kitten) sitting in a 樱花 (sakura) garden, 水彩风格"),
    ("text-numbers", 'A storefront window with the number "2026" painted in gold letters'),
    ("multi-subject", "Three different breeds of dogs (golden retriever, husky, corgi) playing together on a green lawn"),
    ("hands", "A close-up photograph of two human hands holding a small bird, soft focus"),
]

# 错误分类表 —— 和 plan 文件一致
ERR_CLIENT_4XX = "client_4xx"
ERR_AUTH = "auth_401_403"
ERR_RATE_LIMIT = "rate_limit_429"
ERR_GATEWAY_5XX = "gateway_5xx"
ERR_OVERLOADED_529 = "overloaded_529"
ERR_EMPTY_STREAM = "empty_stream"
ERR_SAFETY_BLOCK = "safety_block"
ERR_GOOGLE_CONFIG = "google_config_error"
ERR_SIGNATURE = "signature_error"
ERR_TIMEOUT = "timeout"
ERR_NETWORK = "network_error"
ERR_PARSE = "parse_error"

SAFETY_FINISH_REASONS = {"SAFETY", "PROHIBITED_CONTENT", "BLOCKLIST", "SPII", "RECITATION"}

VALID_IMAGE_SIZES = {"1K", "2K", "4K"}


# ──────────────────────────────────────────────────────────────
# 数据结构
# ──────────────────────────────────────────────────────────────


@dataclass
class RunConfig:
    base_url: str
    mode: str  # gemini-native | anthropic-messages
    model: str
    total: int
    concurrency: int
    image_size: str  # 1K | 2K | 4K | mixed
    prompt_set: str
    timeout: float
    warmup: int
    stream: bool
    save_images: bool
    out_dir: Path
    api_key_present: bool  # 不落盘 key 本身

    def to_dict(self) -> dict:
        d = dataclasses.asdict(self)
        d["out_dir"] = str(self.out_dir)
        return d


@dataclass
class RequestResult:
    seq: int
    started_at: str
    duration_ms: int
    status_code: int | None
    x_request_id: str | None
    model: str
    image_size: str
    prompt_id: str
    outcome: str  # success | http_error | empty_image | safety_block | timeout | network_error | parse_error
    error_category: str | None
    error_message: str | None
    response_bytes: int
    image_sha256: str | None
    first_byte_ms: int | None
    finish_reason: str | None
    block_reason: str | None

    def as_jsonline(self) -> str:
        return json.dumps(dataclasses.asdict(self), ensure_ascii=False, separators=(",", ":"))


# ──────────────────────────────────────────────────────────────
# 请求构造
# ──────────────────────────────────────────────────────────────


def build_gemini_native_request(
    base_url: str,
    model: str,
    api_key: str,
    prompt: str,
    image_size: str,
    stream: bool,
) -> tuple[str, str, dict[str, str], bytes]:
    """返回 (method, url, headers, body) —— Gemini 原生协议"""
    action = "streamGenerateContent" if stream else "generateContent"
    # sub2api 的 Gemini 路由位于 /v1beta/models/{model}:{action}
    # Google 官方支持两种鉴权：?key=XXX 或 x-goog-api-key header。sub2api 两种都接受。
    url = f"{base_url.rstrip('/')}/v1beta/models/{model}:{action}"
    body_obj = {
        "contents": [{"role": "user", "parts": [{"text": prompt}]}],
        "generationConfig": {
            "responseModalities": ["TEXT", "IMAGE"],
            "imageConfig": {"imageSize": image_size},
        },
    }
    headers = {
        "Content-Type": "application/json",
        "x-goog-api-key": api_key,
        "User-Agent": "sub2api-stress/1.0",
    }
    if stream:
        headers["Accept"] = "text/event-stream"
    return "POST", url, headers, json.dumps(body_obj, ensure_ascii=False).encode("utf-8")


def build_anthropic_messages_request(
    base_url: str,
    model: str,
    api_key: str,
    prompt: str,
    image_size: str,
    stream: bool,
) -> tuple[str, str, dict[str, str], bytes]:
    """返回 (method, url, headers, body) —— Anthropic /v1/messages 兼容协议"""
    url = f"{base_url.rstrip('/')}/v1/messages"
    # Gemini image 生成参数不走 Anthropic 的 schema，只能靠模型名 + 默认尺寸，
    # 服务端 GeminiMessagesCompatService 会在转译时用默认 2K（见 extractImageSize 兜底）。
    # 这里把 image_size 放进 metadata 作为记录，但它不会被后端读取。
    body_obj: dict[str, Any] = {
        "model": model,
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": prompt}],
        "stream": stream,
    }
    headers = {
        "Content-Type": "application/json",
        "x-api-key": api_key,
        "anthropic-version": "2023-06-01",
        "User-Agent": "sub2api-stress/1.0",
    }
    if stream:
        headers["Accept"] = "text/event-stream"
    return "POST", url, headers, json.dumps(body_obj, ensure_ascii=False).encode("utf-8")


# ──────────────────────────────────────────────────────────────
# 响应解析 & 错误分类
# ──────────────────────────────────────────────────────────────

_SIGNATURE_RE = re.compile(r"(thought_signature|signature\s+(mismatch|invalid|error))", re.IGNORECASE)


def _classify_http_error(status: int, body_text: str) -> tuple[str, str]:
    """从 HTTP 错误响应中提取错误分类和简要消息"""
    # 先从 body 里拿 message
    msg = body_text.strip()
    try:
        data = json.loads(body_text)
        if isinstance(data, dict):
            # Gemini 格式：{"error": {"message": "...", "code": 500, "status": "..."}}
            err = data.get("error")
            if isinstance(err, dict) and err.get("message"):
                msg = str(err["message"])
            elif isinstance(err, str):
                msg = err
    except (ValueError, TypeError):
        pass

    msg = msg[:500]  # 截断，避免日志巨大
    lower = msg.lower()

    # 特殊文本信号优先
    if "invalid project resource name" in lower:
        return ERR_GOOGLE_CONFIG, msg
    if _SIGNATURE_RE.search(lower):
        return ERR_SIGNATURE, msg
    if "empty" in lower and ("stream" in lower or "response" in lower):
        return ERR_EMPTY_STREAM, msg

    # 再按 status code
    if status in (401, 403):
        return ERR_AUTH, msg
    if status == 429:
        return ERR_RATE_LIMIT, msg
    if status == 529:
        return ERR_OVERLOADED_529, msg
    if 500 <= status <= 599:
        return ERR_GATEWAY_5XX, msg
    if 400 <= status <= 499:
        return ERR_CLIENT_4XX, msg
    return ERR_PARSE, msg or f"unexpected status {status}"


def _parse_gemini_native_success(body_text: str) -> tuple[str, str | None, str | None, str | None, int, str | None]:
    """
    解析 Gemini 原生成功响应。
    返回 (outcome, error_category, error_message, image_sha256, response_bytes_used, finish_reason/block_reason)。
    response_bytes_used 忽略（caller 用原始 len）。
    """
    try:
        data = json.loads(body_text)
    except (ValueError, TypeError) as e:
        return "parse_error", ERR_PARSE, f"json decode: {e}", None, 0, None

    # promptFeedback.blockReason 优先检查 —— 请求被整体拒绝
    prompt_fb = data.get("promptFeedback") if isinstance(data, dict) else None
    if isinstance(prompt_fb, dict) and prompt_fb.get("blockReason"):
        return "safety_block", ERR_SAFETY_BLOCK, f"promptFeedback.blockReason={prompt_fb['blockReason']}", None, 0, str(prompt_fb["blockReason"])

    candidates = data.get("candidates") if isinstance(data, dict) else None
    if not candidates or not isinstance(candidates, list):
        return "empty_image", ERR_EMPTY_STREAM, "no candidates in response", None, 0, None

    cand = candidates[0]
    if not isinstance(cand, dict):
        return "parse_error", ERR_PARSE, "candidate is not an object", None, 0, None

    finish_reason = cand.get("finishReason")
    if isinstance(finish_reason, str) and finish_reason in SAFETY_FINISH_REASONS:
        return "safety_block", ERR_SAFETY_BLOCK, f"finishReason={finish_reason}", None, 0, finish_reason

    content = cand.get("content") or {}
    parts = content.get("parts") or [] if isinstance(content, dict) else []
    image_b64: str | None = None
    for p in parts:
        if not isinstance(p, dict):
            continue
        # Gemini HTTP JSON 用 camelCase：inlineData.data
        inline = p.get("inlineData") or p.get("inline_data")
        if isinstance(inline, dict) and inline.get("data"):
            image_b64 = str(inline["data"])
            break

    if not image_b64:
        # 200 OK 但没图 —— 就是客户说的"请求成功了可是没东西"
        return "empty_image", ERR_EMPTY_STREAM, f"no inline_data in parts (finishReason={finish_reason})", None, 0, finish_reason

    try:
        raw = base64.b64decode(image_b64, validate=False)
        sha = hashlib.sha256(raw).hexdigest()
    except Exception as e:  # noqa: BLE001
        return "parse_error", ERR_PARSE, f"base64 decode: {e}", None, 0, finish_reason

    return "success", None, None, sha, len(raw), finish_reason


def _parse_anthropic_messages_success(
    body_text: str,
) -> tuple[str, str | None, str | None, str | None, int, str | None]:
    """解析 Anthropic /v1/messages 成功响应"""
    try:
        data = json.loads(body_text)
    except (ValueError, TypeError) as e:
        return "parse_error", ERR_PARSE, f"json decode: {e}", None, 0, None

    # Anthropic 格式：{"content":[{"type":"image","source":{"type":"base64","data":"..."}}, ...]}
    # sub2api 的 compat 层翻译 Gemini 图片为 Anthropic 图片 content block
    content = data.get("content") if isinstance(data, dict) else None
    stop_reason = data.get("stop_reason") if isinstance(data, dict) else None
    if not content or not isinstance(content, list):
        return "empty_image", ERR_EMPTY_STREAM, f"no content blocks (stop_reason={stop_reason})", None, 0, stop_reason

    image_b64: str | None = None
    for block in content:
        if not isinstance(block, dict):
            continue
        if block.get("type") == "image":
            source = block.get("source")
            if isinstance(source, dict) and source.get("data"):
                image_b64 = str(source["data"])
                break
        # 兼容：也可能被翻译为 text 里包含 base64，或 image_url
        if block.get("type") == "image_url":
            url_obj = block.get("image_url")
            if isinstance(url_obj, dict) and isinstance(url_obj.get("url"), str) and url_obj["url"].startswith("data:"):
                # data URL
                try:
                    image_b64 = url_obj["url"].split(",", 1)[1]
                    break
                except (IndexError, ValueError):
                    pass

    if not image_b64:
        return "empty_image", ERR_EMPTY_STREAM, f"no image block in content (stop_reason={stop_reason})", None, 0, stop_reason

    try:
        raw = base64.b64decode(image_b64, validate=False)
        sha = hashlib.sha256(raw).hexdigest()
    except Exception as e:  # noqa: BLE001
        return "parse_error", ERR_PARSE, f"base64 decode: {e}", None, 0, stop_reason

    return "success", None, None, sha, len(raw), stop_reason


# ──────────────────────────────────────────────────────────────
# 单次请求
# ──────────────────────────────────────────────────────────────


async def run_one(
    client: httpx.AsyncClient,
    seq: int,
    cfg: RunConfig,
    api_key: str,
    prompt_id: str,
    prompt_text: str,
    image_size: str,
    jsonl_fh,  # 已打开的文件句柄
    jsonl_lock: asyncio.Lock,
) -> RequestResult:
    started_at = datetime.now(timezone.utc).isoformat()
    t0 = time.perf_counter()
    method: str
    url: str
    headers: dict[str, str]
    body: bytes

    if cfg.mode == "gemini-native":
        method, url, headers, body = build_gemini_native_request(
            cfg.base_url, cfg.model, api_key, prompt_text, image_size, cfg.stream
        )
    else:
        method, url, headers, body = build_anthropic_messages_request(
            cfg.base_url, cfg.model, api_key, prompt_text, image_size, cfg.stream
        )

    status: int | None = None
    x_request_id: str | None = None
    response_bytes = 0
    image_sha: str | None = None
    outcome: str
    err_cat: str | None = None
    err_msg: str | None = None
    first_byte_ms: int | None = None
    finish_reason: str | None = None
    block_reason: str | None = None

    try:
        if cfg.stream:
            # 流式：测 first-byte 延迟，然后收敛成完整 body
            async with client.stream(method, url, headers=headers, content=body) as resp:
                status = resp.status_code
                x_request_id = resp.headers.get("x-request-id") or resp.headers.get("X-Request-ID")
                chunks: list[bytes] = []
                async for chunk in resp.aiter_bytes():
                    if first_byte_ms is None:
                        first_byte_ms = int((time.perf_counter() - t0) * 1000)
                    chunks.append(chunk)
                body_bytes = b"".join(chunks)
        else:
            resp = await client.request(method, url, headers=headers, content=body)
            status = resp.status_code
            x_request_id = resp.headers.get("x-request-id") or resp.headers.get("X-Request-ID")
            body_bytes = resp.content

        response_bytes = len(body_bytes)
        body_text = body_bytes.decode("utf-8", errors="replace")

        if status >= 400:
            outcome = "http_error"
            err_cat, err_msg = _classify_http_error(status, body_text)
        else:
            if cfg.mode == "gemini-native":
                # 对于 streamGenerateContent，响应其实是 JSON 数组（Gemini 的流式是 JSON 流）
                # 这里我们把多个 JSON 对象合成一个 —— 实际 Gemini HTTP 流式返回的是连续的 JSON 对象数组
                if cfg.stream and body_text.lstrip().startswith("["):
                    # 已经是数组，照常解析后取最后一个元素作为完整候选
                    try:
                        arr = json.loads(body_text)
                        if isinstance(arr, list) and arr:
                            # 把所有 candidates[0].content.parts 合并成一份再走单响应解析
                            merged: dict[str, Any] = {"candidates": [{"content": {"parts": []}}]}
                            last_finish = None
                            for item in arr:
                                if not isinstance(item, dict):
                                    continue
                                for cand in item.get("candidates") or []:
                                    if not isinstance(cand, dict):
                                        continue
                                    if cand.get("finishReason"):
                                        last_finish = cand["finishReason"]
                                    cont = cand.get("content") or {}
                                    for part in cont.get("parts") or []:
                                        merged["candidates"][0]["content"]["parts"].append(part)
                            if last_finish:
                                merged["candidates"][0]["finishReason"] = last_finish
                            body_text = json.dumps(merged)
                    except (ValueError, TypeError):
                        pass
                outcome, err_cat, err_msg, image_sha, raw_bytes, finish_reason = _parse_gemini_native_success(body_text)
            else:
                outcome, err_cat, err_msg, image_sha, raw_bytes, finish_reason = _parse_anthropic_messages_success(body_text)

            if image_sha and cfg.save_images and raw_bytes:
                try:
                    img_dir = cfg.out_dir / "images"
                    img_dir.mkdir(parents=True, exist_ok=True)
                    (img_dir / f"{seq:05d}-{image_sha[:12]}.bin").write_bytes(
                        base64.b64decode(_extract_b64_from_text(body_text, cfg.mode) or "")
                    )
                except Exception:  # noqa: BLE001
                    # save-images 失败不影响主流程
                    pass

    except httpx.TimeoutException as e:
        outcome = "timeout"
        err_cat = ERR_TIMEOUT
        err_msg = f"{type(e).__name__}: {e}"[:500]
    except httpx.HTTPError as e:
        outcome = "network_error"
        err_cat = ERR_NETWORK
        err_msg = f"{type(e).__name__}: {e}"[:500]
    except Exception as e:  # noqa: BLE001 — 任何未预期错误都必须转成结果记录
        outcome = "network_error"
        err_cat = ERR_NETWORK
        err_msg = f"{type(e).__name__}: {e}"[:500]

    duration_ms = int((time.perf_counter() - t0) * 1000)
    result = RequestResult(
        seq=seq,
        started_at=started_at,
        duration_ms=duration_ms,
        status_code=status,
        x_request_id=x_request_id,
        model=cfg.model,
        image_size=image_size,
        prompt_id=prompt_id,
        outcome=outcome,
        error_category=err_cat,
        error_message=err_msg,
        response_bytes=response_bytes,
        image_sha256=image_sha,
        first_byte_ms=first_byte_ms,
        finish_reason=finish_reason,
        block_reason=block_reason,
    )

    # 即时落盘 —— 即使中途 Ctrl+C，已完成的数据也保留
    async with jsonl_lock:
        jsonl_fh.write(result.as_jsonline() + "\n")
        jsonl_fh.flush()

    return result


def _extract_b64_from_text(body_text: str, mode: str) -> str | None:
    """为 save-images 重新从响应里掏出 base64。懒实现 —— 失败返回 None。"""
    try:
        data = json.loads(body_text)
    except (ValueError, TypeError):
        return None
    if mode == "gemini-native":
        for cand in (data.get("candidates") or []):
            for part in (cand.get("content") or {}).get("parts") or []:
                inline = part.get("inlineData") or part.get("inline_data")
                if inline and inline.get("data"):
                    return inline["data"]
    else:
        for block in data.get("content") or []:
            if block.get("type") == "image":
                source = block.get("source") or {}
                if source.get("data"):
                    return source["data"]
    return None


# ──────────────────────────────────────────────────────────────
# 压测主循环
# ──────────────────────────────────────────────────────────────


def load_prompts(prompt_set: str) -> list[tuple[str, str]]:
    if prompt_set == "default":
        return list(DEFAULT_PROMPTS)
    if prompt_set == "stress":
        return list(DEFAULT_PROMPTS) + list(STRESS_EXTRA_PROMPTS)
    # 否则当成文件路径
    p = Path(prompt_set)
    if not p.is_file():
        raise SystemExit(f"prompt-set 必须是 default/stress 或一个存在的文件: {prompt_set}")
    prompts: list[tuple[str, str]] = []
    for i, line in enumerate(p.read_text(encoding="utf-8").splitlines()):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        prompts.append((f"file-{i}", line))
    if not prompts:
        raise SystemExit(f"prompt-set 文件没有有效提示词: {prompt_set}")
    return prompts


def resolve_image_size(policy: str, seq: int) -> str:
    if policy == "mixed":
        return ["1K", "2K", "4K"][seq % 3]
    return policy


async def stress_run(cfg: RunConfig, api_key: str) -> list[RequestResult]:
    cfg.out_dir.mkdir(parents=True, exist_ok=True)
    (cfg.out_dir / "run.json").write_text(
        json.dumps(cfg.to_dict(), ensure_ascii=False, indent=2),
        encoding="utf-8",
    )

    prompts = load_prompts(cfg.prompt_set)
    print(
        f"[stress] base={cfg.base_url} mode={cfg.mode} model={cfg.model} "
        f"total={cfg.total} concurrency={cfg.concurrency} size={cfg.image_size} "
        f"stream={cfg.stream} out={cfg.out_dir}"
    )

    sem = asyncio.Semaphore(cfg.concurrency)
    jsonl_lock = asyncio.Lock()
    results: list[RequestResult] = []
    stop_flag = {"stop": False}

    def _sigint(_sig, _frame):
        if stop_flag["stop"]:
            # 第二次 Ctrl+C → 立刻退出
            raise KeyboardInterrupt
        stop_flag["stop"] = True
        print("\n[stress] 收到 Ctrl+C，等待在飞请求完成后退出（再次 Ctrl+C 强退）...", file=sys.stderr)

    signal.signal(signal.SIGINT, _sigint)

    timeout = httpx.Timeout(cfg.timeout, connect=15.0)
    limits = httpx.Limits(max_connections=max(cfg.concurrency * 2, 20), max_keepalive_connections=cfg.concurrency)
    # 某些环境（包括 Windows）上 h2 握手偶尔会不稳定，图片生成又是大响应体，
    # 关闭 http2 更稳。需要开启时手动改这里即可。
    async with httpx.AsyncClient(http2=False, timeout=timeout, limits=limits, follow_redirects=False) as client:
        with (cfg.out_dir / "requests.jsonl").open("w", encoding="utf-8") as jsonl_fh:

            async def worker(seq: int) -> None:
                async with sem:
                    if stop_flag["stop"]:
                        return
                    pid, ptext = prompts[seq % len(prompts)]
                    # 冷启动阶段的请求用默认 size，避免 4K 把 warmup 拉长
                    size = "2K" if seq < cfg.warmup else resolve_image_size(cfg.image_size, seq)
                    r = await run_one(
                        client, seq, cfg, api_key, pid, ptext, size, jsonl_fh, jsonl_lock
                    )
                    results.append(r)
                    # 实时进度打印（每完成 1 个都打，总量不会很大）
                    marker = "OK " if r.outcome == "success" else f"ERR({r.error_category or r.outcome})"
                    print(
                        f"[{seq + 1:>4}/{cfg.total}] {marker:<24} {r.duration_ms:>6}ms "
                        f"status={r.status_code} prompt={pid} rid={r.x_request_id or '-'}",
                        flush=True,
                    )

            tasks = [asyncio.create_task(worker(i)) for i in range(cfg.total)]
            try:
                await asyncio.gather(*tasks)
            except KeyboardInterrupt:
                for t in tasks:
                    t.cancel()

    return results


# ──────────────────────────────────────────────────────────────
# 汇总报告
# ──────────────────────────────────────────────────────────────


def _percentile(values: list[int], p: float) -> int:
    if not values:
        return 0
    values = sorted(values)
    k = (len(values) - 1) * p
    lo = int(k)
    hi = min(lo + 1, len(values) - 1)
    return int(values[lo] + (values[hi] - values[lo]) * (k - lo))


def summarize(cfg: RunConfig, results: list[RequestResult]) -> str:
    # 跳过 warmup
    eff = [r for r in results if r.seq >= cfg.warmup]
    total = len(eff)
    if total == 0:
        return "# 压测汇总\n\n没有有效请求（全是 warmup）。\n"

    ok = [r for r in eff if r.outcome == "success"]
    fail = [r for r in eff if r.outcome != "success"]
    durations = [r.duration_ms for r in eff]
    ok_durations = [r.duration_ms for r in ok]
    succ_rate = len(ok) / total * 100

    lines: list[str] = []
    lines.append("# Sub2API 图片生成压测汇总")
    lines.append("")
    lines.append(f"- 生成时间：{datetime.now(timezone.utc).isoformat()}")
    lines.append(f"- Base URL：`{cfg.base_url}`")
    lines.append(f"- Mode：`{cfg.mode}`  Model：`{cfg.model}`  ImageSize：`{cfg.image_size}`  Stream：`{cfg.stream}`")
    lines.append(f"- 请求参数：total={cfg.total} concurrency={cfg.concurrency} warmup={cfg.warmup}（已排除）")
    lines.append("")
    lines.append("## 总体")
    lines.append("")
    lines.append(f"- 有效请求：**{total}**")
    lines.append(f"- 成功：**{len(ok)}** ({succ_rate:.2f}%)")
    lines.append(f"- 失败：**{len(fail)}**")
    lines.append("")

    lines.append("## 延迟（ms）")
    lines.append("")
    if durations:
        lines.append(f"- 全部：min={min(durations)} mean={int(statistics.fmean(durations))} max={max(durations)}")
        lines.append(
            f"  p50={_percentile(durations, 0.5)} p90={_percentile(durations, 0.9)} "
            f"p95={_percentile(durations, 0.95)} p99={_percentile(durations, 0.99)}"
        )
    if ok_durations:
        lines.append(f"- 仅成功：min={min(ok_durations)} mean={int(statistics.fmean(ok_durations))} max={max(ok_durations)}")
        lines.append(
            f"  p50={_percentile(ok_durations, 0.5)} p90={_percentile(ok_durations, 0.9)} "
            f"p95={_percentile(ok_durations, 0.95)} p99={_percentile(ok_durations, 0.99)}"
        )
    lines.append("")

    # 错误分类
    from collections import Counter
    cat_counter: Counter[str] = Counter()
    outcome_counter: Counter[str] = Counter()
    for r in fail:
        outcome_counter[r.outcome] += 1
        cat_counter[r.error_category or "unknown"] += 1

    lines.append("## 失败分类")
    lines.append("")
    if not fail:
        lines.append("（无失败）")
    else:
        lines.append("| 分类 | 次数 | 占失败 | 占全部 |")
        lines.append("|---|---:|---:|---:|")
        for cat, cnt in cat_counter.most_common():
            lines.append(
                f"| `{cat}` | {cnt} | {cnt / len(fail) * 100:.1f}% | {cnt / total * 100:.1f}% |"
            )
        lines.append("")
        lines.append("### outcome 分布")
        for oc, cnt in outcome_counter.most_common():
            lines.append(f"- `{oc}`：{cnt}")
    lines.append("")

    # 按 prompt 统计成功率 —— 找出"哪些提示词更容易炸"
    lines.append("## Prompt 成功率")
    lines.append("")
    prompt_stats: dict[str, tuple[int, int]] = {}  # pid -> (ok, total)
    for r in eff:
        o, t = prompt_stats.get(r.prompt_id, (0, 0))
        prompt_stats[r.prompt_id] = (o + (1 if r.outcome == "success" else 0), t + 1)
    lines.append("| prompt_id | 成功 | 总计 | 成功率 |")
    lines.append("|---|---:|---:|---:|")
    for pid, (o, t) in sorted(prompt_stats.items(), key=lambda kv: kv[1][0] / max(kv[1][1], 1)):
        lines.append(f"| `{pid}` | {o} | {t} | {o / t * 100:.0f}% |")
    lines.append("")

    # 时间窗 —— 把开始时间戳按 30s 分桶
    lines.append("## 时间窗（30s 桶）")
    lines.append("")
    buckets: dict[int, list[RequestResult]] = {}
    if eff:
        t0 = datetime.fromisoformat(eff[0].started_at).timestamp()
        for r in eff:
            ts = datetime.fromisoformat(r.started_at).timestamp()
            b = int((ts - t0) // 30)
            buckets.setdefault(b, []).append(r)
        lines.append("| 窗口 (s) | 请求数 | 成功率 | 均值延迟 (ms) |")
        lines.append("|---:|---:|---:|---:|")
        for b in sorted(buckets):
            items = buckets[b]
            bo = sum(1 for x in items if x.outcome == "success")
            avg = int(statistics.fmean(x.duration_ms for x in items)) if items else 0
            lines.append(f"| {b * 30}-{(b + 1) * 30} | {len(items)} | {bo / len(items) * 100:.0f}% | {avg} |")
    lines.append("")

    # Top 失败 X-Request-ID —— 给服务端日志关联用
    lines.append("## Top 失败请求（X-Request-ID）")
    lines.append("")
    lines.append("拿这些 `request_id` 到服务器查上下文：")
    lines.append("")
    lines.append("```bash")
    lines.append("python deploy/remote_exec.py 'docker logs sub2api --since 1h | grep <request_id>'")
    lines.append("```")
    lines.append("")
    lines.append("| # | status | category | duration_ms | prompt | request_id | message |")
    lines.append("|---:|---:|---|---:|---|---|---|")
    for r in fail[:20]:
        msg = (r.error_message or "").replace("|", "\\|").replace("\n", " ")[:120]
        lines.append(
            f"| {r.seq} | {r.status_code} | `{r.error_category}` | {r.duration_ms} "
            f"| `{r.prompt_id}` | `{r.x_request_id or '-'}` | {msg} |"
        )
    lines.append("")

    # image_sha 重复检测
    from collections import Counter as C2
    sha_counter: C2[str] = C2()
    for r in ok:
        if r.image_sha256:
            sha_counter[r.image_sha256] += 1
    dups = [(s, c) for s, c in sha_counter.items() if c > 1]
    if dups:
        lines.append("## ⚠ 图片重复")
        lines.append("")
        lines.append("以下 sha256 出现多次 —— 可能是上游返回了缓存/占位图：")
        lines.append("")
        for sha, cnt in sorted(dups, key=lambda x: -x[1])[:10]:
            lines.append(f"- `{sha[:16]}…` × {cnt}")
        lines.append("")

    return "\n".join(lines)


# ──────────────────────────────────────────────────────────────
# 入口
# ──────────────────────────────────────────────────────────────


def parse_args(argv: list[str] | None = None) -> RunConfig:
    p = argparse.ArgumentParser(
        prog="image_stress_test",
        description="Sub2API 图片生成 API 压力测试",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    p.add_argument("--base-url", default=DEFAULT_BASE_URL)
    p.add_argument("--mode", choices=["gemini-native", "anthropic-messages"], default="gemini-native")
    p.add_argument("--model", default="gemini-3-pro-image")
    p.add_argument("--total", type=int, default=50)
    p.add_argument("--concurrency", type=int, default=5)
    p.add_argument("--image-size", default="2K", help="1K / 2K / 4K / mixed")
    p.add_argument("--prompt-set", default="default", help="default / stress / 或提示词文件路径")
    p.add_argument("--timeout", type=float, default=120.0, help="单请求超时秒数")
    p.add_argument("--warmup", type=int, default=3, help="前 N 个请求不计入统计")
    p.add_argument("--stream", action="store_true", help="gemini-native 模式下用 streamGenerateContent")
    p.add_argument("--save-images", action="store_true", help="把生成的图片落盘到 <out>/images/")
    p.add_argument("--out", default="", help="输出目录，默认 output/stress-<timestamp>")
    p.add_argument("--api-key", default="", help="API key，默认从 $SUB2API_KEY 读取")
    args = p.parse_args(argv)

    if args.image_size not in VALID_IMAGE_SIZES and args.image_size != "mixed":
        p.error(f"--image-size 必须是 1K/2K/4K/mixed，当前: {args.image_size}")
    if args.total <= 0:
        p.error("--total 必须 > 0")
    if args.concurrency <= 0:
        p.error("--concurrency 必须 > 0")
    if args.warmup < 0:
        p.error("--warmup 必须 >= 0")
    if args.warmup >= args.total:
        # 小总量场景（冒烟）自动 clamp，而不是硬失败
        new_warmup = max(0, args.total - 1)
        print(
            f"[stress] 提示：--warmup={args.warmup} >= --total={args.total}，自动调整为 {new_warmup}",
            file=sys.stderr,
        )
        args.warmup = new_warmup
    if args.mode == "anthropic-messages" and args.stream:
        print("[stress] 注意：--stream 在 anthropic-messages 模式下仅设置 stream=true，不走 Gemini 原生流式路径", file=sys.stderr)

    api_key = args.api_key or os.environ.get("SUB2API_KEY", "")

    if args.out:
        out_dir = Path(args.out)
    else:
        ts = datetime.now().strftime("%Y%m%d-%H%M%S")
        out_dir = Path("output") / f"stress-{ts}"

    return RunConfig(
        base_url=args.base_url,
        mode=args.mode,
        model=args.model,
        total=args.total,
        concurrency=args.concurrency,
        image_size=args.image_size,
        prompt_set=args.prompt_set,
        timeout=args.timeout,
        warmup=args.warmup,
        stream=args.stream,
        save_images=args.save_images,
        out_dir=out_dir,
        api_key_present=bool(api_key),
    ), api_key


def _reconfigure_stdio_utf8() -> None:
    """Windows 默认控制台是 cp936，直接 print 中文会乱码；重配置 stdout/stderr 为 UTF-8。"""
    for stream_name in ("stdout", "stderr"):
        stream = getattr(sys, stream_name, None)
        reconfigure = getattr(stream, "reconfigure", None)
        if callable(reconfigure):
            try:
                reconfigure(encoding="utf-8", errors="replace")
            except Exception:  # noqa: BLE001
                pass


def main(argv: list[str] | None = None) -> int:
    _reconfigure_stdio_utf8()
    cfg, api_key = parse_args(argv)
    if not api_key:
        print(
            "ERROR: 未提供 API key。请 export SUB2API_KEY=... 或使用 --api-key",
            file=sys.stderr,
        )
        return 2

    random.seed(42)  # 提示词轮换可重放

    try:
        results = asyncio.run(stress_run(cfg, api_key))
    except KeyboardInterrupt:
        print("\n[stress] 强制退出", file=sys.stderr)
        return 130

    summary = summarize(cfg, results)
    (cfg.out_dir / "summary.md").write_text(summary, encoding="utf-8")
    print("")
    print(f"[stress] 完成。结果目录：{cfg.out_dir}")
    print(f"[stress]   - run.json")
    print(f"[stress]   - requests.jsonl")
    print(f"[stress]   - summary.md")
    # 控制台也打一份简短尾行，方便 scrollback
    eff = [r for r in results if r.seq >= cfg.warmup]
    ok = sum(1 for r in eff if r.outcome == "success")
    print(f"[stress] 成功 {ok}/{len(eff)} ({ok / max(len(eff), 1) * 100:.1f}%)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
