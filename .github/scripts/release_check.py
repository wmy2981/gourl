#!/usr/bin/env python3
"""发行检查：版本号从根目录 VERSION 文件读取（手动维护，不再自动 bump）。

规则（复刻 wmy2981/connection-checker）：
- 合法正式版 x.y.z；合法预发行 x.y.z.alpha.n / x.y.z.beta.n
- main 分支：只接受正式版。与仓库最大版本 tag 比较，无变化或倒退则报错退出；前进则发行
- dev 分支：只接受预发行。无变化则跳过（成功退出，不发版）；倒退则报错退出；前进则预发行；
  dev 上出现正式版号 = 即将由 main 发版，跳过
- 发行说明范围：最后一个正式版 tag 到 HEAD（预发行与正式发行一致）

输出（写入 GITHUB_OUTPUT）：
- version: 当前版本号
- is_prerelease: true/false
- skip: true（dev 无变化或正式版号，跳过）
- last_release_tag: 最后一个正式版 tag（空串表示无）
同时生成 .release-notes.md。
"""
import os
import re
import subprocess
import sys
from pathlib import Path

VERSION_FILE = "VERSION"
VERSION_RE = re.compile(r"^(\d+)\.(\d+)\.(\d+)(?:\.(alpha|beta)\.(\d+))?$")
TAG_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)(?:\.(alpha|beta)\.(\d+))?$")


def parse(v: str) -> tuple | None:
    """版本转排序元组：(major, minor, patch, rank, pre_n)，rank: alpha=0 beta=1 正式版=2。"""
    m = VERSION_RE.match(v)
    if not m:
        return None
    major, minor, patch = (int(x) for x in m.group(1, 2, 3))
    pre = m.group(4)
    n = int(m.group(5) or 0)
    rank = {"alpha": 0, "beta": 1}.get(pre, 2)
    return (major, minor, patch, rank, n)


def is_prerelease(v: str) -> bool:
    m = VERSION_RE.match(v)
    return bool(m and m.group(4))


def fail(msg: str) -> None:
    print(f"::error::{msg}")
    print(msg)
    sys.exit(1)


def all_tags() -> list[tuple]:
    out = subprocess.run(["git", "tag"], capture_output=True, text=True).stdout
    tags: list[tuple] = []
    for line in out.splitlines():
        m = TAG_RE.match(line.strip())
        if not m:
            continue
        base = f"{m.group(1)}.{m.group(2)}.{m.group(3)}"
        if m.group(4):
            base += f".{m.group(4)}.{m.group(5)}"
        p = parse(base)
        if p:
            tags.append(p)
    return tags


def fmt(v: tuple) -> str:
    major, minor, patch, rank, n = v
    if rank == 2:
        return f"{major}.{minor}.{patch}"
    pre = "alpha" if rank == 0 else "beta"
    return f"{major}.{minor}.{patch}.{pre}.{n}"


# Conventional Commits 解析与分组（与 connection-checker 产出格式一致）
_COMMIT_RE = re.compile(
    r"^(?P<type>[a-zA-Z_]+)(?:\((?P<scope>[^)]+)\))?(?P<breaking>!)?:\s*(?P<subject>.+)$"
)
_GROUPS = [
    ("Breaking Changes", "breaking"),
    ("Features", "feat"),
    ("Bug Fixes", "fix"),
    ("Documentation", "docs"),
    ("Performance Improvements", "perf"),
    ("Refactorings", "refactor"),
    ("Styles", "style"),
    ("Testing", "test"),
    ("Build System", "build"),
    ("Continuous Integration", "ci"),
    ("Chores", "chore"),
]


def _repo_slug() -> str:
    out = subprocess.run(
        ["git", "remote", "get-url", "origin"], capture_output=True, text=True
    ).stdout.strip()
    m = re.search(r"(?:github\.com[:/])([\w.-]+/[\w.-]+?)(?:\.git)?$", out)
    return m.group(1) if m else "wmy2981/gourl"


def build_notes(version: str, last_release_tag: str) -> None:
    """发行说明（与 connection-checker 格式一致）：## vX.Y.Z (日期) + 按类型分组提交。

    提交范围：最后一个正式版 tag（v 前缀）到 HEAD；无正式版 tag 时取全部提交。
    """
    if last_release_tag:
        range_spec = f"v{last_release_tag}..HEAD"
    else:
        range_spec = "HEAD"
    out = subprocess.run(
        ["git", "log", range_spec, "--pretty=format:%H%x09%s"],
        capture_output=True,
        text=True,
    ).stdout
    repo = _repo_slug()

    grouped: dict[str, list[str]] = {}
    for line in out.splitlines():
        line = line.strip()
        if not line:
            continue
        full_hash, subject = line.split("\t", 1)
        short = full_hash[:7]
        link = f"[`{short}`](https://github.com/{repo}/commit/{full_hash})"
        m = _COMMIT_RE.match(subject)
        if m:
            ctype = m.group("type").lower()
            scope = m.group("scope")
            breaking = bool(m.group("breaking")) or "BREAKING CHANGE" in subject
            text = m.group("subject")
            group = "breaking" if breaking else ctype
        else:
            group = "other"
            text = subject
        entry = f"- **{scope}**: {text} {link}" if m and scope else f"- {text} {link}"
        grouped.setdefault(group, []).append(entry)

    today = subprocess.run(
        ["git", "log", "-1", "--format=%ad", "--date=format:%Y-%m-%d", "HEAD"],
        capture_output=True,
        text=True,
    ).stdout.strip()
    lines = [f"## v{version} ({today})", ""]
    for group_name, key in _GROUPS:
        entries = grouped.get(key)
        if not entries:
            continue
        lines.append(f"### {group_name}")
        lines.append("")
        lines.extend(entries)
        lines.append("")
    others = grouped.get("other")
    if others:
        lines.append("### Other")
        lines.append("")
        lines.extend(others)
        lines.append("")
    if len(lines) <= 2:
        lines.append("- 无提交")
        lines.append("")
    Path(".release-notes.md").write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    branch = sys.argv[1] if len(sys.argv) > 1 else ""
    version = Path(VERSION_FILE).read_text(encoding="utf-8").strip()

    if not VERSION_RE.match(version):
        fail(
            f"非法版本号 {version!r}：正式版必须为 x.y.z，预发行必须为 x.y.z.alpha.n 或 x.y.z.beta.n"
        )

    tags = all_tags()
    last = max(tags) if tags else None
    # 发行说明基准：最后一个正式版 tag（排除预发行）
    last_release = max((t for t in tags if t[3] == 2), default=None)
    notes_tag = fmt(last_release) if last_release else ""
    cur = parse(version)

    if branch == "main":
        if is_prerelease(version):
            fail(f"main 分支只接受正式版（x.y.z），当前 {version} 是预发行版")
        if last is None:
            action = "release"
        elif cur == last:
            # 版本无变化：不发版，成功退出（推送 main 不要求必须发版）
            action = "skip"
        elif cur < last:
            fail(f"版本号倒退：{version} < 已发版 v{fmt(last)}")
        else:
            action = "release"
    elif branch == "dev":
        if last is None or cur > last:
            if is_prerelease(version):
                action = "prerelease"
            else:
                # dev 上正式版号 = 即将推送正式版（由 main 发版），此处跳过发版
                action = "skip"
        elif cur == last:
            action = "skip"
        else:
            fail(f"版本号倒退：{version} < 已发版 v{fmt(last)}")
    else:
        fail(f"不支持的触发分支 {branch!r}（仅 main / dev）")

    if action != "skip":
        build_notes(version, notes_tag)

    payload = (
        f"version={version}\n"
        f"is_prerelease={'true' if action == 'prerelease' else 'false'}\n"
        f"skip={'true' if action == 'skip' else 'false'}\n"
        f"last_release_tag={notes_tag}\n"
    )
    out_path = os.environ.get("GITHUB_OUTPUT")
    if out_path:
        with open(out_path, "a", encoding="utf-8") as f:
            f.write(payload)
    else:
        print(payload, end="")
    print(f"action={action} ({branch})")


if __name__ == "__main__":
    main()
