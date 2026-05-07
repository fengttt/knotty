#!/usr/bin/env python3
"""
Build links.json + GIF images for all links in the Thistlethwaite link
table (L2a1 .. L11n459) by scraping katlas.org.

For each link we collect:
  - name           (e.g. "L2a1")
  - pd             planar-diagram code as list of [a,b,c,d] crossings
  - gauss          Gauss code as list-of-lists of signed integers
  - jones          Jones polynomial (LaTeX string)
  - homfly         HOMFLY-PT polynomial (LaTeX string)

The PD presentation page returns "X<sub>4132</sub> X<sub>2314</sub>"
(four single-digit indices); for crossing counts >= 10 the indices are
rendered with separators we still need to handle. We detect that by
trying both single-digit splitting and a comma/whitespace fallback.

GIF images are fetched via Special:FilePath which redirects to the
underlying upload path.

Run from repo root:
    python3 dataset/links/build_dataset.py
"""

from __future__ import annotations

import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

BASE = "https://katlas.org"
HERE = Path(__file__).resolve().parent
NAMES_FILE = HERE / "_names.txt"
GIF_DIR = HERE
JSON_OUT = HERE / "links.json"
RAW_DIR = HERE / "_raw"
RAW_DIR.mkdir(exist_ok=True)

# Reasonable defaults: katlas.org rate-limits aggressive parallelism with
# 403s after ~15 concurrent requests. Stay conservative; the cache makes
# re-runs cheap.
WORKERS = 4
RETRIES = 5


def fetch(url: str) -> bytes:
    last_err = None
    for attempt in range(RETRIES):
        try:
            req = urllib.request.Request(
                url, headers={"User-Agent": "knotty-dataset-builder/1.0"}
            )
            with urllib.request.urlopen(req, timeout=30) as resp:
                return resp.read()
        except urllib.error.HTTPError as e:
            last_err = e
            # 403 from katlas means we're being rate-limited; back off
            # progressively rather than blasting through retries.
            if e.code == 403:
                time.sleep(2 * (attempt + 1))
            else:
                time.sleep(0.5 * (attempt + 1))
        except (urllib.error.URLError, TimeoutError) as e:
            last_err = e
            time.sleep(0.5 * (attempt + 1))
    raise RuntimeError(f"fetch failed for {url}: {last_err}")


def fetch_text(url: str) -> str:
    return fetch(url).decode("utf-8", errors="replace")


def raw_url(title: str) -> str:
    return f"{BASE}/index.php?title={title}&action=raw"


_PD_RE = re.compile(r"X<sub>([^<]*)</sub>", re.IGNORECASE)
_MATH_RE = re.compile(r"<math>(.*?)</math>", re.IGNORECASE | re.DOTALL)


def parse_pd(s: str) -> list[list[int]]:
    """Parse "X<sub>4132</sub> X<sub>2314</sub>" or, for >=10 crossings,
    "X<sub>4,1,3,2</sub> X<sub>2,3,1,4</sub>" into [[4,1,3,2],[2,3,1,4]]."""
    out: list[list[int]] = []
    for m in _PD_RE.finditer(s):
        body = m.group(1).strip()
        if "," in body:
            parts = [p.strip() for p in body.split(",") if p.strip()]
        else:
            # All single digits when none reach 10. We allow this to fail
            # (returning a single 4-char block) below.
            parts = list(body)
        try:
            out.append([int(p) for p in parts])
        except ValueError:
            # Fall back to extracting consecutive digits.
            out.append([int(d) for d in re.findall(r"\d+", body)])
    return out


def parse_gauss(s: str) -> list[list[int]]:
    """Parse "{1, -2}, {2, -1}" into [[1,-2],[2,-1]]."""
    components = []
    for blob in re.findall(r"\{([^{}]*)\}", s):
        nums = [int(n) for n in re.findall(r"-?\d+", blob)]
        components.append(nums)
    return components


def parse_math(s: str) -> str:
    m = _MATH_RE.search(s)
    return (m.group(1) if m else s).strip()


def fetch_link(name: str) -> dict:
    cache = RAW_DIR / f"{name}.json"
    if cache.exists():
        return json.loads(cache.read_text())
    pd_html = fetch_text(raw_url(f"Data:{name}/PD_Presentation"))
    gauss_html = fetch_text(raw_url(f"Data:{name}/Gauss_Code"))
    jones_html = fetch_text(raw_url(f"Data:{name}/Jones_Polynomial"))
    homfly_html = fetch_text(raw_url(f"Data:{name}/HOMFLYPT_Polynomial"))
    rec = {
        "name": name,
        "pd": parse_pd(pd_html),
        "gauss": parse_gauss(gauss_html),
        "jones": parse_math(jones_html),
        "homfly": parse_math(homfly_html),
    }
    cache.write_text(json.dumps(rec))
    return rec


def fetch_gif(name: str) -> None:
    target = GIF_DIR / f"{name}.gif"
    if target.exists() and target.stat().st_size > 0:
        return
    data = fetch(f"{BASE}/wiki/Special:FilePath/{name}.gif")
    if not data.startswith(b"GIF"):
        raise RuntimeError(f"{name}: not a GIF (got {data[:8]!r})")
    target.write_bytes(data)


def main() -> int:
    names = [n.strip() for n in NAMES_FILE.read_text().splitlines() if n.strip()]
    print(f"loading {len(names)} links", file=sys.stderr)

    records: dict[str, dict] = {}
    failures: list[tuple[str, str]] = []

    def task_data(name: str) -> tuple[str, dict | str]:
        try:
            return name, fetch_link(name)
        except Exception as e:
            return name, f"data: {e}"

    def task_gif(name: str) -> tuple[str, str | None]:
        try:
            fetch_gif(name)
            return name, None
        except Exception as e:
            return name, f"gif: {e}"

    with ThreadPoolExecutor(max_workers=WORKERS) as pool:
        futs = [pool.submit(task_data, n) for n in names]
        for i, fut in enumerate(as_completed(futs), 1):
            name, res = fut.result()
            if isinstance(res, dict):
                records[name] = res
            else:
                failures.append((name, res))
            if i % 50 == 0:
                print(f"  data {i}/{len(names)}", file=sys.stderr)

    with ThreadPoolExecutor(max_workers=WORKERS) as pool:
        futs = [pool.submit(task_gif, n) for n in names]
        for i, fut in enumerate(as_completed(futs), 1):
            name, err = fut.result()
            if err:
                failures.append((name, err))
            if i % 50 == 0:
                print(f"  gif  {i}/{len(names)}", file=sys.stderr)

    ordered = [records[n] for n in sorted(names) if n in records]
    JSON_OUT.write_text(json.dumps(ordered, indent=2) + "\n")
    print(f"wrote {JSON_OUT} ({len(ordered)} records)", file=sys.stderr)

    if failures:
        print(f"{len(failures)} failures:", file=sys.stderr)
        for n, e in failures[:20]:
            print(f"  {n}: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
