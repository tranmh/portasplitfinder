#!/usr/bin/env python3
"""Coverage floor for the CI safety net (written by aqaa).

usage: coverage_gate.py <cobertura|gocover|jacoco> <report glob> <floor percent>
Exits 1 when the line coverage of the matching reports is below the floor.
"""
import glob
import os
import re
import sys
import xml.etree.ElementTree as ET


def gocover(files):
    blocks = {}
    for name in files:
        with open(name, encoding="utf-8") as f:
            for line in f:
                m = re.match(r"(.+:\d+\.\d+,\d+\.\d+) (\d+) (\d+)$", line.strip())
                if m:
                    stmts, count = int(m.group(2)), int(m.group(3))
                    _, seen = blocks.get(m.group(1), (stmts, 0))
                    blocks[m.group(1)] = (stmts, seen + count)
    total = sum(s for s, _ in blocks.values())
    covered = sum(s for s, c in blocks.values() if c > 0)
    return covered, total


def cobertura(files):
    covered = total = 0
    for name in files:
        root = ET.parse(name).getroot()
        if root.get("lines-valid") is not None:
            total += int(root.get("lines-valid"))
            covered += int(root.get("lines-covered"))
        else:
            lines = root.findall(".//line")
            total += len(lines)
            covered += sum(1 for line in lines if int(line.get("hits", "0")) > 0)
    return covered, total


def jacoco(files):
    covered = total = 0
    for name in files:
        root = ET.parse(name).getroot()
        for counter in root.findall("counter"):
            if counter.get("type") == "LINE":
                covered += int(counter.get("covered"))
                total += int(counter.get("covered")) + int(counter.get("missed"))
    return covered, total


def main():
    fmt, pattern, floor = sys.argv[1], sys.argv[2], float(sys.argv[3])
    files = sorted(glob.glob(pattern, recursive=True))
    if not files:
        print(f"no coverage report matches {pattern}")
        return 1
    covered, total = {"gocover": gocover, "jacoco": jacoco}.get(fmt, cobertura)(files)
    pct = 100.0 * covered / total if total else 0.0
    verdict = "ok" if pct + 1e-9 >= floor else "below the floor"
    line = f"Line coverage {pct:.1f}% ({covered}/{total} lines), floor {floor:.0f}%: {verdict}"
    print(line)
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as f:
            f.write(f"- {line}\n")
    return 0 if verdict == "ok" else 1


if __name__ == "__main__":
    sys.exit(main())
