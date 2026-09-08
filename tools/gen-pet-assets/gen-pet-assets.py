#!/usr/bin/env python3
"""Generate the desktop-pet (桌宠) sprite assets via an OpenAI-style image API.

The pet window shows a chibi Hatsune Miku with one sprite per state
(idle / running / cheer / drag / sleep / wave). Each state is generated
separately with a fixed character-description block for consistency, then
post-processed: the background is removed (transparent output when the model
supports it, edge flood-fill over white otherwise), the sprite is cropped,
normalised onto a 660x900 transparent canvas (bottom-aligned, displayed at a
140px width in the 180x240 pet window) and quantised to keep the go:embed
small.

The API key is read from the PET_IMG_API_KEY environment variable and is
never written to disk or logs.

Usage:
  PET_IMG_API_KEY=sk-... python tools/gen-pet-assets/gen-pet-assets.py \
      --endpoint https://www.cafecode.work/v1/images/generations \
      --model gpt-image-2 --size 1024x1536 \
      --out frontend/src/assets/pet [--only idle,wave] [--skip-generate]
"""

from __future__ import annotations

import argparse
import base64
import ipaddress
import json
import os
import re
import socket
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

# The relay can be slow and rate-limits aggressively (429s even on the first
# burst), so give each request a generous timeout and wait long between
# retries.
REQUEST_TIMEOUT = 360
ATTEMPTS = 10

# Output canvas: high-res source that the pet window scales down to a
# 140px-wide sprite, bottom-aligned so the feet stay planted.
CANVAS = (660, 900)

# Fixed character block shared by every prompt so the six sprites stay
# consistent in outfit, palette and style across separate generations.
CHARACTER = (
    "Kawaii chibi sticker illustration of Hatsune Miku, two-heads-tall chibi "
    "proportions with an oversized head and tiny body, long teal twin-tails "
    "tied with plain solid-black ribbon hair ties, no stripes and no pink "
    "accents on the ribbons, big teal eyes, grey sleeveless dress shirt with "
    "a teal necktie, grey pleated mini skirt, detached black sleeves, black "
    "thigh-high boots with teal trim, thick clean outlines, flat cel shading, "
    "vibrant colors, full body centered with margin around the character, "
    "isolated on a plain pure white background, no shadow, no ground, no "
    "text, no watermark, no border, no scenery"
)

STATES = {
    "idle": "standing upright and relaxed, arms hanging naturally at her sides, gentle closed-mouth smile, looking at the viewer, symmetrical front view",
    "running": "running happily straight toward the viewer, mid-stride, one arm bent and swinging forward, the other swung back, mouth open in an excited smile, ribbons fluttering, slight forward lean",
    "cheer": "cheering in celebration, both arms raised high in a V shape, huge open-mouth happy smile, joyful sparkling expression, front view",
    "drag": "lifted up into the air, both arms raised straight above her head in surprise, legs dangling, wide eyes and a small open mouth, startled but cute expression, front view",
    "sleep": "sitting on the ground hugging her knees, eyes closed, peaceful sleeping face, head resting on her knees, twin-tails draped beside her, front view",
    "wave": "standing and cheerfully waving hello with one hand raised high, big happy smile, eyes closed in delight, the other arm relaxed, front view",
}

# --- Hardened fetching -------------------------------------------------------
# Every URL (the configured endpoint, download URLs returned in responses, and
# every redirect hop) is checked to be https without credentials, resolving to
# public addresses only, so the script cannot be turned into a probe for
# intranet or metadata services.


def check_url(url: str) -> None:
    parts = urllib.parse.urlparse(url)
    if parts.scheme != "https" or not parts.hostname:
        raise RuntimeError(f"only https URLs are allowed, got scheme {parts.scheme!r}")
    if parts.username or parts.password:
        raise RuntimeError("credentials in URLs are not allowed")
    for info in socket.getaddrinfo(parts.hostname, parts.port or 443, type=socket.SOCK_STREAM):
        ip = ipaddress.ip_address(info[4][0])
        if not ip.is_global:
            raise RuntimeError(f"host {parts.hostname} resolves to non-public address {ip}")


def safe_state_name(state: str) -> str:
    """Return state as a filename component, refusing anything but the plain
    lowercase names used by STATES, so every path derived from CLI input can
    never contain separators or traverse out of its directory."""
    if not re.fullmatch(r"[a-z]+", state):
        raise RuntimeError(f"invalid state name: {state!r}")
    return state


def path_under(base: Path, name: str) -> Path:
    """Join base/name and require the result to sit directly inside the
    resolved base — defence in depth even if a name ever went wrong."""
    base_resolved = base.resolve()
    out = base_resolved / name
    if out.parent != base_resolved:
        raise RuntimeError(f"path escapes its directory: {out}")
    return out


def safe_api_key() -> str:
    """Read PET_IMG_API_KEY and require a plain token of letters, digits,
    dots, dashes and underscores. The key is placed into an Authorization
    header, so any other character (whitespace, CR/LF, control chars) is
    either a guaranteed auth failure or a header-injection risk — refuse it
    before it reaches any request."""
    key = os.environ.get("PET_IMG_API_KEY", "")
    if not re.fullmatch(r"[A-Za-z0-9_.\-]{8,}", key):
        sys.exit("PET_IMG_API_KEY is not set or malformed")
    return key


class _SafeRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: D102
        check_url(newurl)
        return super().redirect_request(req, fp, code, msg, headers, newurl)


_OPENER = urllib.request.build_opener(_SafeRedirectHandler)


def _read_error(err: urllib.error.HTTPError) -> str:
    try:
        return err.read().decode("utf-8", "replace")[:300]
    except Exception:  # noqa: BLE001 - best-effort diagnostics only
        return ""


def _open(req: urllib.request.Request, timeout: int):
    try:
        return _OPENER.open(req, timeout=timeout)
    except urllib.error.HTTPError as err:
        raise RuntimeError(f"HTTP {err.code}: {_read_error(err)}") from err


def http_get(url: str, timeout: int) -> bytes:
    check_url(url)
    with _open(urllib.request.Request(url), timeout) as resp:
        return resp.read()


def http_post_json(url: str, payload: dict, api_key: str, timeout: int) -> dict:
    check_url(url)
    req = urllib.request.Request(
        url,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
    )
    with _open(req, timeout) as resp:
        return json.load(resp)


def candidate_bodies(model: str, prompt: str, size: str | None) -> list[dict]:
    """Request bodies to try in order, most-preferred first.

    gpt-image models always return base64 and support a transparent
    background, but relays may reject the extra parameters, so the plain body
    is the fallback. Other relay models take the response_format form.
    """
    base = {"model": model, "prompt": prompt, "n": 1}
    sized = {"size": size} if size else {}
    if model.startswith("gpt-image"):
        return [
            dict(base, background="transparent", output_format="png", **sized),
            dict(base, **sized),
        ]
    return [dict(base, response_format="b64_json")]


def generate_one(
    api_key: str, endpoint: str, model: str, prompt: str, size: str | None, timeout: int
) -> bytes:
    errors = []
    for body in candidate_bodies(model, prompt, size):
        try:
            payload = http_post_json(endpoint, body, api_key, timeout)
        except RuntimeError as err:
            errors.append(str(err))
            continue
        items = payload.get("data") or []
        if not items:
            raise RuntimeError(f"no data in response: {json.dumps(payload)[:300]}")
        item = items[0]
        if item.get("b64_json"):
            return base64.b64decode(item["b64_json"])
        if item.get("url"):
            return http_get(item["url"], timeout)
        raise RuntimeError(f"no b64_json/url in item: {json.dumps(item)[:300]}")
    raise RuntimeError("; ".join(errors))


def run_generation(
    api_key: str,
    endpoint: str,
    model: str,
    states: list[str],
    raw_dir: Path,
    size: str | None,
    timeout: int,
) -> None:
    raw_dir.mkdir(parents=True, exist_ok=True)
    for state in states:
        out_file = path_under(raw_dir, f"{safe_state_name(state)}.png")
        if out_file.exists():
            print(f"[{state}] raw asset already exists, skip ({out_file})")
            continue
        prompt = f"{CHARACTER}, {STATES[state]}"
        for attempt in range(1, ATTEMPTS + 1):
            try:
                started = time.monotonic()
                print(f"[{state}] generating (attempt {attempt}, model {model}) ...")
                png = generate_one(api_key, endpoint, model, prompt, size, timeout)
                out_file.write_bytes(png)
                print(f"[{state}] saved {len(png) // 1024} KiB in {time.monotonic() - started:.0f}s")
                break
            except Exception as err:  # noqa: BLE001 - report and retry
                print(f"[{state}] attempt {attempt} failed: {err}", file=sys.stderr)
                if attempt == ATTEMPTS:
                    raise
                # 429 quota exhaustion ("上游账号额度等待恢复") can take
                # minutes to recover: wait it out patiently.
                time.sleep(min(180, 90 * attempt))


def neutralise_pink_accents(arr):
    """Desaturate saturated pink/magenta pixels to neutral dark grey.

    Generations sometimes tint the black hair ribbons with pink stripes while
    the rest of the set keeps plain black ones. Only highly saturated
    magenta-pink hues are touched (the hair ribbons are the only such area);
    skin blush is far less saturated and the mouth is a red hue, so both
    survive. Luminance is kept (slightly darkened) so the stripes turn into
    a grey sheen on the black ribbon.
    """
    import numpy as np

    rgb = arr[:, :, :3].astype(np.float32) / 255.0
    r, g, b = rgb[..., 0], rgb[..., 1], rgb[..., 2]
    mx = rgb.max(axis=2)
    mn = rgb.min(axis=2)
    delta = mx - mn
    sat = np.where(mx > 0, delta / np.maximum(mx, 1e-6), 0.0)

    safe_delta = np.where(delta > 1e-6, delta, 1.0)
    h_r = (60.0 * ((g - b) / safe_delta)) % 360.0
    h_g = 60.0 * ((b - r) / safe_delta) + 120.0
    h_b = 60.0 * ((r - g) / safe_delta) + 240.0
    hue = np.where(delta > 1e-6, np.where(mx == r, h_r, np.where(mx == g, h_g, h_b)), 0.0)

    sel = (hue >= 275.0) & (hue <= 360.0) & (sat > 0.30) & (mx > 0.12)
    luma = 255.0 * (0.299 * r + 0.587 * g + 0.114 * b)
    grey = np.clip(luma * 0.72, 0, 225)
    out = arr.copy()
    for c in range(3):
        channel = out[:, :, c]
        channel[sel] = grey[sel].astype(np.int16)
    return out


def process_one(raw_file: Path, out_file: Path) -> None:
    import numpy as np
    from PIL import Image

    img = Image.open(raw_file).convert("RGBA")
    arr = neutralise_pink_accents(np.asarray(img).astype(np.int16))

    # Transparent-background art only needs the alpha channel; white
    # backgrounds are removed by a mask grown from the image border (a global
    # near-white mask would also punch holes in the pale shirt).
    alpha = arr[:, :, 3]
    if alpha.min() < 250:
        pass  # already has real transparency, keep it
    else:
        rgb = arr[:, :, :3]
        near_white = (255 - rgb.min(axis=2)) <= 30
        bg = np.zeros_like(near_white)
        bg[0, :] = near_white[0, :]
        bg[-1, :] = near_white[-1, :]
        bg[:, 0] = near_white[:, 0]
        bg[:, -1] = near_white[:, -1]
        while True:
            prev = bg
            bg = near_white & (
                bg | np.roll(bg, 1, 0) | np.roll(bg, -1, 0) | np.roll(bg, 1, 1) | np.roll(bg, -1, 1)
            )
            if np.array_equal(bg, prev):
                break
        alpha = np.where(bg, 0, 255).astype(np.uint8)
        # Feather the cut edge: foreground pixels touching the background
        # keep an alpha proportional to how far they are from white, which
        # removes the residual white halo without visible steps.
        fg = ~bg
        boundary = fg & (
            np.roll(bg, 1, 0) | np.roll(bg, -1, 0) | np.roll(bg, 1, 1) | np.roll(bg, -1, 1)
        )
        whiteness = rgb.min(axis=2)
        alpha = np.where(boundary, np.clip((255 - whiteness) * 4, 0, 255), alpha).astype(np.uint8)

    rgba = np.dstack([arr[:, :, :3].astype(np.uint8), alpha])
    sprite = Image.fromarray(rgba, "RGBA")

    bbox = sprite.getbbox()
    if bbox is None:
        raise RuntimeError(f"{raw_file}: background removal left an empty sprite")
    sprite = sprite.crop(bbox)

    # Fit into the canvas preserving aspect ratio, then anchor to the bottom
    # centre so the feet stay planted across states.
    sprite.thumbnail((CANVAS[0], CANVAS[1]), Image.LANCZOS)
    canvas = Image.new("RGBA", CANVAS, (0, 0, 0, 0))
    canvas.paste(
        sprite,
        ((CANVAS[0] - sprite.width) // 2, CANVAS[1] - sprite.height),
        sprite,
    )
    quantised = canvas.quantize(colors=256, method=Image.Quantize.FASTOCTREE)
    out_file.parent.mkdir(parents=True, exist_ok=True)
    quantised.save(out_file, optimize=True)
    print(f"[{out_file.stem}] {out_file.stat().st_size // 1024} KiB -> {out_file}")

    # Debug composite over a mid-grey background to expose halos and holes;
    # kept next to the raw files so the repo asset dir stays clean.
    debug = Image.alpha_composite(Image.new("RGBA", CANVAS, (120, 120, 120, 255)), canvas)
    debug.convert("RGB").save(raw_file.with_suffix(".debug.jpg"), quality=90)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", default="frontend/src/assets/pet", help="output asset dir")
    parser.add_argument(
        "--endpoint",
        default="https://ayase.cn/simple-api/v1/images/generations",
        help="OpenAI-style images/generations endpoint",
    )
    parser.add_argument("--model", default="grok-imagine-image-quality")
    parser.add_argument("--size", default=None, help="image size, e.g. 1024x1536")
    parser.add_argument("--timeout", type=int, default=REQUEST_TIMEOUT, help="per-request timeout seconds")
    parser.add_argument("--only", default="", help="comma-separated subset of states")
    parser.add_argument("--raw-dir", default=None, help="where raw generations are kept (default: system temp)")
    parser.add_argument("--skip-generate", action="store_true", help="reuse existing raw files")
    parser.add_argument("--skip-process", action="store_true", help="only generate raw files")
    args = parser.parse_args()

    states = [s.strip() for s in args.only.split(",") if s.strip()] or list(STATES)
    unknown = [s for s in states if s not in STATES]
    if unknown:
        parser.error(f"unknown states: {unknown}; known: {list(STATES)}")

    raw_dir = Path(args.raw_dir) if args.raw_dir else Path(tempfile.gettempdir()) / "miku-pet-raw"

    if not args.skip_generate:
        run_generation(
            safe_api_key(), args.endpoint, args.model, states, raw_dir, args.size, args.timeout
        )

    if not args.skip_process:
        out_dir = Path(args.out)
        for state in states:
            name = safe_state_name(state)
            process_one(
                path_under(raw_dir, f"{name}.png"),
                path_under(out_dir, f"pet-{name}.png"),
            )


if __name__ == "__main__":
    main()
