#!/usr/bin/env python3
"""
Generate cliff-registry seeding candidates from GitHub search.

This script is intentionally conservative:
- It always writes a review queue (CSV + JSON).
- It only emits ready-to-lint manifests for Go repos when asked.
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import re
import subprocess
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import tomllib


CLI_TOPICS = {
    "cli",
    "tui",
    "terminal",
    "command-line",
    "commandline",
    "shell",
    "console",
}

NEGATIVE_TERMS = {
    "library",
    "sdk",
    "framework",
    "api",
    "boilerplate",
    "plugin",
    "template",
    "dotfiles",
}

POSITIVE_TEXT_PATTERNS = (
    r"\bcli\b",
    r"\btui\b",
    r"\bterminal\b",
    r"\bcommand[- ]line\b",
)


@dataclass(frozen=True)
class Rules:
    deny_terms: list[str]
    deny_owners: list[str]
    deny_name_patterns: list[str]
    allow_terms: list[str]
    allow_owners: list[str]
    allow_name_patterns: list[str]


@dataclass(frozen=True)
class Candidate:
    rank: int
    full_name: str
    name: str
    owner: str
    html_url: str
    description: str
    stars: int
    language: str
    topics: list[str]
    created_at: str
    pushed_at: str
    default_branch: str
    license_spdx: str
    score: int
    confidence: str
    suggested_install_type: str
    suggested_package: str
    why: str


def run_gh_search(query: str, limit: int, since: str, min_stars: int) -> list[dict[str, Any]]:
    cmd = [
        "gh",
        "search",
        "repos",
        "--limit",
        str(limit),
        "--sort",
        "stars",
        "--order",
        "desc",
        "--created",
        f">={since}",
        "--stars",
        f">={min_stars}",
        "--archived=false",
        "--include-forks=false",
        "--visibility",
        "public",
        "--match",
        "name,description",
        "--json",
        ",".join(
            [
                "fullName",
                "name",
                "owner",
                "url",
                "description",
                "stargazersCount",
                "language",
                "createdAt",
                "pushedAt",
                "defaultBranch",
                "license",
                "isArchived",
                "isFork",
            ]
        ),
    ]
    if query:
        cmd.insert(3, query)
    proc = subprocess.run(cmd, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or "gh search repos failed")
    return json.loads(proc.stdout)


def default_since() -> str:
    return (dt.date.today() - dt.timedelta(days=365)).isoformat()


def load_rules(path: Path | None) -> Rules:
    if path is None:
        return Rules([], [], [], [], [], [])
    if not path.exists():
        raise FileNotFoundError(f"rules file not found: {path}")

    data = tomllib.loads(path.read_text(encoding="utf-8"))
    deny = data.get("deny") or {}
    allow = data.get("allow") or {}
    return Rules(
        deny_terms=[str(x).lower() for x in deny.get("terms", [])],
        deny_owners=[str(x).lower() for x in deny.get("owners", [])],
        deny_name_patterns=[str(x) for x in deny.get("name_patterns", [])],
        allow_terms=[str(x).lower() for x in allow.get("terms", [])],
        allow_owners=[str(x).lower() for x in allow.get("owners", [])],
        allow_name_patterns=[str(x) for x in allow.get("name_patterns", [])],
    )


def apply_rules(repo: dict[str, Any], rules: Rules) -> tuple[bool, str]:
    full_name = str(repo.get("fullName") or "").lower()
    owner = str((repo.get("owner") or {}).get("login") or "").lower()
    name = str(repo.get("name") or "").lower()
    desc = str(repo.get("description") or "").lower()
    haystack = f"{full_name} {name} {desc}"

    for denied_owner in rules.deny_owners:
        if owner == denied_owner:
            return False, f"deny.owner:{denied_owner}"

    for term in rules.deny_terms:
        if term and term in haystack:
            return False, f"deny.term:{term}"

    for pat in rules.deny_name_patterns:
        if pat and re.search(pat, full_name, re.IGNORECASE):
            return False, f"deny.pattern:{pat}"

    allow_reasons: list[str] = []
    for allowed_owner in rules.allow_owners:
        if owner == allowed_owner:
            allow_reasons.append(f"allow.owner:{allowed_owner}")
    for term in rules.allow_terms:
        if term and term in haystack:
            allow_reasons.append(f"allow.term:{term}")
    for pat in rules.allow_name_patterns:
        if pat and re.search(pat, full_name, re.IGNORECASE):
            allow_reasons.append(f"allow.pattern:{pat}")

    return True, "; ".join(allow_reasons)


def load_existing_registry(registry_dir: Path) -> tuple[set[str], set[str]]:
    if not registry_dir.exists():
        raise FileNotFoundError(f"registry dir not found: {registry_dir}")
    apps_dir = registry_dir / "apps"
    if not apps_dir.exists():
        raise FileNotFoundError(f"apps dir not found: {apps_dir}")

    existing_names: set[str] = set()
    existing_homepages: set[str] = set()
    for toml_file in apps_dir.glob("*.toml"):
        try:
            data = tomllib.loads(toml_file.read_text(encoding="utf-8"))
        except Exception:
            continue
        name = str(data.get("name", "")).strip().lower()
        homepage = str(data.get("homepage", "")).strip().lower()
        if name:
            existing_names.add(name)
        if homepage:
            existing_homepages.add(homepage)
    return existing_names, existing_homepages


def score_repo(repo: dict[str, Any]) -> tuple[int, str, str]:
    name = str(repo.get("name") or "")
    desc = str(repo.get("description") or "")
    language = str(repo.get("language") or "")
    topics = [str(t).lower() for t in repo.get("topics") or []]
    source_query = str(repo.get("_source_query") or "")

    text = f"{name} {desc}".lower()
    score = 0
    reasons: list[str] = []

    topic_hits = sorted(set(topics).intersection(CLI_TOPICS))
    if topic_hits:
        score += 4 + len(topic_hits)
        reasons.append(f"topics={','.join(topic_hits)}")

    if any(re.search(pat, text) for pat in POSITIVE_TEXT_PATTERNS):
        score += 3
        reasons.append("cli-like text")

    if "topic:tui" in source_query or "topic:cli" in source_query:
        score += 2
        reasons.append("topic-based search hit")

    if "tool" in text or "manager" in text:
        score += 1
        reasons.append("tooling keywords")

    neg_hits = [term for term in NEGATIVE_TERMS if term in text]
    if neg_hits:
        score -= min(4, len(neg_hits))
        reasons.append(f"possible-library={','.join(sorted(neg_hits))}")

    if language.lower() in {"go", "rust"}:
        score += 1
        reasons.append(f"language={language.lower()}")

    if score >= 7:
        confidence = "high"
    elif score >= 4:
        confidence = "medium"
    else:
        confidence = "low"

    why = "; ".join(reasons) if reasons else "no strong signals"
    return score, confidence, why


def suggest_install(repo: dict[str, Any]) -> tuple[str, str]:
    language = str(repo.get("language") or "").lower()
    full_name = str(repo.get("full_name") or repo.get("fullName") or "")
    repo_name = str(repo.get("name") or "")
    if language == "go":
        return "go", f"github.com/{full_name}@latest"
    if language == "rust":
        return "cargo", repo_name
    if language in {"python"}:
        return "pipx", repo_name
    if language in {"javascript", "typescript"}:
        return "npm", repo_name
    return "unknown", ""


def fetch_candidates(queries: list[str], limit: int, since: str, min_stars: int) -> list[dict[str, Any]]:
    collected: dict[str, dict[str, Any]] = {}
    per_query_limit = max(100, min(1000, limit))

    for q in queries:
        items = run_gh_search(q, per_query_limit, since, min_stars)
        for item in items:
            key = str(item.get("fullName", "")).lower()
            if key:
                item["_source_query"] = q
                collected[key] = item

    repos = list(collected.values())
    repos = [r for r in repos if not r.get("isArchived") and not r.get("isFork")]
    repos.sort(key=lambda r: int(r.get("stargazersCount") or 0), reverse=True)
    return repos[:limit]


def derive_queries(since: str, min_stars: int) -> list[str]:
    _ = (since, min_stars)
    return [
        "topic:tui",
        "topic:cli",
        "topic:terminal",
        '"terminal ui"',
        '"command line" cli',
    ]


def choose_slug(base_name: str, owner: str, used: set[str]) -> str:
    slug = re.sub(r"[^a-z0-9-]+", "-", base_name.lower()).strip("-")
    slug = re.sub(r"-{2,}", "-", slug) or "app"
    if slug not in used:
        used.add(slug)
        return slug
    fallback = f"{slug}-{owner.lower()}"
    fallback = re.sub(r"[^a-z0-9-]+", "-", fallback).strip("-")
    if fallback not in used:
        used.add(fallback)
        return fallback
    i = 2
    while f"{fallback}-{i}" in used:
        i += 1
    final = f"{fallback}-{i}"
    used.add(final)
    return final


INSTALL_TYPE_DIRS = {
    "go": "manifests-go",
    "cargo": "manifests-cargo",
    "pipx": "manifests-pipx",
    "npm": "manifests-npm",
}


_REGISTRY_CACHE: dict[tuple[str, str], bool] = {}


def _http_head_ok(url: str, timeout: float = 5.0) -> bool:
    req = urllib.request.Request(url, method="HEAD")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return 200 <= resp.status < 300
    except urllib.error.HTTPError as e:
        return 200 <= e.code < 300
    except Exception:
        return False


def package_published(install_type: str, package: str) -> bool:
    if not package:
        return False
    key = (install_type, package.lower())
    if key in _REGISTRY_CACHE:
        return _REGISTRY_CACHE[key]

    ok = False
    if install_type == "pipx":
        ok = _http_head_ok(f"https://pypi.org/pypi/{package}/json")
    elif install_type == "npm":
        ok = _http_head_ok(f"https://registry.npmjs.org/{package}")
    elif install_type == "cargo":
        ok = _http_head_ok(f"https://crates.io/api/v1/crates/{package}")
    else:
        ok = True

    _REGISTRY_CACHE[key] = ok
    return ok


def emit_outputs(
    out_dir: Path,
    ranked: list[Candidate],
    emit_install_types: list[str],
    max_manifests: int,
    existing_names: set[str],
    existing_homepages: set[str],
    verify_registry: bool = False,
) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)

    json_path = out_dir / "candidates.json"
    csv_path = out_dir / "review.csv"

    json_path.write_text(
        json.dumps([c.__dict__ for c in ranked], indent=2, ensure_ascii=True) + "\n",
        encoding="utf-8",
    )

    with csv_path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(
            f,
            fieldnames=[
                "rank",
                "full_name",
                "stars",
                "language",
                "created_at",
                "pushed_at",
                "topics",
                "html_url",
                "description",
                "confidence",
                "score",
                "suggested_install_type",
                "suggested_package",
                "why",
            ],
        )
        writer.writeheader()
        for c in ranked:
            writer.writerow(
                {
                    "rank": c.rank,
                    "full_name": c.full_name,
                    "stars": c.stars,
                    "language": c.language,
                    "created_at": c.created_at,
                    "pushed_at": c.pushed_at,
                    "topics": ",".join(c.topics),
                    "html_url": c.html_url,
                    "description": c.description,
                    "confidence": c.confidence,
                    "score": c.score,
                    "suggested_install_type": c.suggested_install_type,
                    "suggested_package": c.suggested_package,
                    "why": c.why,
                }
            )

    if not emit_install_types:
        return

    emitted_rows: list[dict[str, Any]] = []
    used_names = set(existing_names)
    counts: dict[str, int] = {t: 0 for t in emit_install_types}

    for itype in emit_install_types:
        (out_dir / INSTALL_TYPE_DIRS[itype]).mkdir(parents=True, exist_ok=True)

    for c in ranked:
        if c.suggested_install_type not in emit_install_types:
            continue
        if c.confidence == "low":
            continue
        if c.html_url.lower() in existing_homepages:
            continue
        if max_manifests > 0 and counts[c.suggested_install_type] >= max_manifests:
            continue
        if verify_registry and c.suggested_install_type in {"pipx", "npm", "cargo"}:
            if not package_published(c.suggested_install_type, c.suggested_package):
                continue

        manifest_name = choose_slug(c.name, c.owner, used_names)
        desc = c.description.strip().replace('"', "'").replace("\n", " ")
        desc = re.sub(r"\s+", " ", desc)
        if len(desc) > 120:
            desc = desc[:117].rstrip(" .,;:-—") + "..."
        if len(desc) > 120:
            desc = desc[:120]

        filtered_tags = [t for t in c.topics if re.fullmatch(r"[a-z0-9-]+", t)]
        if "cli" not in filtered_tags and "tui" not in filtered_tags:
            if "tui" in c.why:
                filtered_tags.append("tui")
            else:
                filtered_tags.append("cli")
        filtered_tags = sorted(set(filtered_tags))[:8]
        tags_literal = ", ".join(f'"{t}"' for t in filtered_tags)

        readme_url = (
            f"https://raw.githubusercontent.com/{c.full_name}/{c.default_branch}/README.md"
        )
        license_line = f'license = "{c.license_spdx}"\n' if c.license_spdx else ""

        manifest = (
            f'name = "{manifest_name}"\n'
            f'description = "{desc}"\n'
            f'author = "{c.owner}"\n'
            f'homepage = "{c.html_url}"\n'
            f'readme = "{readme_url}"\n'
            f"tags = [{tags_literal}]\n"
            f"{license_line}"
            "\n"
            "[install]\n"
            f'type = "{c.suggested_install_type}"\n'
            f'package = "{c.suggested_package}"\n'
        )
        target_dir = out_dir / INSTALL_TYPE_DIRS[c.suggested_install_type]
        (target_dir / f"{manifest_name}.toml").write_text(manifest, encoding="utf-8")
        counts[c.suggested_install_type] += 1
        emitted_rows.append(
            {
                "manifest": f"{manifest_name}.toml",
                "rank": c.rank,
                "full_name": c.full_name,
                "stars": c.stars,
                "language": c.language,
                "html_url": c.html_url,
                "description": c.description,
                "confidence": c.confidence,
                "score": c.score,
                "why": c.why,
            }
        )

    if emitted_rows:
        batch_csv = out_dir / "manifest-batch.csv"
        with batch_csv.open("w", newline="", encoding="utf-8") as f:
            writer = csv.DictWriter(
                f,
                fieldnames=[
                    "manifest",
                    "rank",
                    "full_name",
                    "stars",
                    "language",
                    "html_url",
                    "description",
                    "confidence",
                    "score",
                    "why",
                ],
            )
            writer.writeheader()
            for row in emitted_rows:
                writer.writerow(row)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--since",
        default=default_since(),
        help="Only include repos created on/after this date (YYYY-MM-DD).",
    )
    parser.add_argument(
        "--min-stars",
        type=int,
        default=75,
        help="GitHub search lower bound for stars.",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=1000,
        help="Final number of candidates after dedupe and sorting.",
    )
    parser.add_argument(
        "--out-dir",
        type=Path,
        required=True,
        help="Output directory for review files and optional manifests.",
    )
    parser.add_argument(
        "--registry-dir",
        type=Path,
        default=None,
        help="Optional local path to cliff-registry checkout for dedupe.",
    )
    parser.add_argument(
        "--emit-go-manifests",
        action="store_true",
        help="Emit ready-to-lint TOML manifests for medium/high-confidence Go repos.",
    )
    parser.add_argument(
        "--emit-cargo-manifests",
        action="store_true",
        help="Emit cargo manifests for medium/high-confidence Rust repos.",
    )
    parser.add_argument(
        "--emit-pipx-manifests",
        action="store_true",
        help="Emit pipx manifests for medium/high-confidence Python repos.",
    )
    parser.add_argument(
        "--emit-npm-manifests",
        action="store_true",
        help="Emit npm manifests for medium/high-confidence JS/TS repos.",
    )
    parser.add_argument(
        "--max-manifests",
        type=int,
        default=0,
        help="Cap emitted manifests per install type; 0 means no cap.",
    )
    parser.add_argument(
        "--verify-registry",
        action="store_true",
        help="Verify pipx/npm/cargo packages exist in their registry before emitting (HEAD requests).",
    )
    parser.add_argument(
        "--rules-file",
        type=Path,
        default=Path("scripts/seeding-rules.toml"),
        help="TOML rules with deny/allow filters (defaults to scripts/seeding-rules.toml).",
    )
    args = parser.parse_args()

    try:
        queries = derive_queries(args.since, args.min_stars)
        repos = fetch_candidates(queries, args.limit, args.since, args.min_stars)
        rules = load_rules(args.rules_file)
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    existing_names: set[str] = set()
    existing_homepages: set[str] = set()
    if args.registry_dir:
        existing_names, existing_homepages = load_existing_registry(args.registry_dir)

    ranked: list[Candidate] = []
    seen_homepages = set(existing_homepages)
    dropped_by_rules = 0
    rank = 0
    for repo in repos:
        keep, rules_reason = apply_rules(repo, rules)
        if not keep:
            dropped_by_rules += 1
            continue

        score, confidence, why = score_repo(repo)
        if rules_reason:
            why = f"{why}; {rules_reason}"
        install_type, install_pkg = suggest_install(repo)
        homepage = str(repo.get("url") or repo.get("html_url") or "").strip()
        if not homepage:
            continue
        if homepage.lower() in seen_homepages:
            continue
        seen_homepages.add(homepage.lower())

        rank += 1
        ranked.append(
            Candidate(
                rank=rank,
                full_name=str(repo.get("fullName") or repo.get("full_name") or ""),
                name=str(repo.get("name") or ""),
                owner=str((repo.get("owner") or {}).get("login") or ""),
                html_url=homepage,
                description=str(repo.get("description") or "").strip(),
                stars=int(repo.get("stargazersCount") or repo.get("stargazers_count") or 0),
                language=str(repo.get("language") or ""),
                topics=[str(t).lower() for t in repo.get("topics") or []],
                created_at=str(repo.get("createdAt") or repo.get("created_at") or ""),
                pushed_at=str(repo.get("pushedAt") or repo.get("pushed_at") or ""),
                default_branch=str(repo.get("defaultBranch") or repo.get("default_branch") or "main"),
                license_spdx=str((repo.get("license") or {}).get("key") or "").upper(),
                score=score,
                confidence=confidence,
                suggested_install_type=install_type,
                suggested_package=install_pkg,
                why=why,
            )
        )

    emit_install_types: list[str] = []
    if args.emit_go_manifests:
        emit_install_types.append("go")
    if args.emit_cargo_manifests:
        emit_install_types.append("cargo")
    if args.emit_pipx_manifests:
        emit_install_types.append("pipx")
    if args.emit_npm_manifests:
        emit_install_types.append("npm")

    emit_outputs(
        args.out_dir,
        ranked,
        emit_install_types=emit_install_types,
        max_manifests=args.max_manifests,
        existing_names=existing_names,
        existing_homepages=existing_homepages,
        verify_registry=args.verify_registry,
    )

    print(f"Wrote {len(ranked)} candidates to {args.out_dir}")
    print(f"Dropped {dropped_by_rules} repos via rules")
    for itype in emit_install_types:
        print(f"Wrote {itype} manifests to {args.out_dir / INSTALL_TYPE_DIRS[itype]}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
