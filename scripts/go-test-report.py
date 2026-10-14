#!/usr/bin/env python3
"""Формирование JUnit XML и сводки из журналов go test -json."""

import json
import os
import sys
from pathlib import Path
from xml.etree import ElementTree as ET


def convert(log_path: Path) -> tuple[int, int, int]:
    cases = {}
    package_errors = {}
    invalid_lines = []

    for line in log_path.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            invalid_lines.append(line)
            continue

        package = event.get("Package", "")
        action = event.get("Action", "")
        name = event.get("Test")
        if name:
            case = cases.setdefault((package, name), {"status": "", "elapsed": 0, "output": []})
            if action in ("pass", "fail", "skip"):
                case["status"] = action
                case["elapsed"] = event.get("Elapsed", 0) or 0
            if action == "output":
                case["output"].append(event.get("Output", ""))
        else:
            error = package_errors.setdefault(package, {"failed": False, "output": []})
            if action == "fail":
                error["failed"] = True
            if action == "output":
                error["output"].append(event.get("Output", ""))

    suite = ET.Element("testsuite", name=log_path.stem)
    passed = failed = skipped = 0

    for (package, name), case in sorted(cases.items()):
        testcase = ET.SubElement(
            suite, "testcase",
            classname=package, name=name, time=str(case["elapsed"])
        )
        if case["status"] == "fail" or not case["status"]:
            failed += 1
            failure = ET.SubElement(testcase, "failure", message="Go test failed")
            failure.text = "".join(case["output"])[-8000:]
        elif case["status"] == "skip":
            skipped += 1
            ET.SubElement(testcase, "skipped")
        else:
            passed += 1

    for package, error in sorted(package_errors.items()):
        if error["failed"] and not any(
            name_package == package and case["status"] == "fail"
            for (name_package, _), case in cases.items()
        ):
            failed += 1
            testcase = ET.SubElement(suite, "testcase", classname=package, name="package")
            failure = ET.SubElement(testcase, "failure", message="Go package failed")
            failure.text = "".join(error["output"])[-8000:]

    if invalid_lines:
        failed += 1
        testcase = ET.SubElement(suite, "testcase", name="test runner output")
        failure = ET.SubElement(testcase, "failure", message="Non-JSON output from go test")
        failure.text = "\n".join(invalid_lines)[-8000:]

    suite.set("tests", str(passed + failed + skipped))
    suite.set("failures", str(failed))
    suite.set("skipped", str(skipped))
    ET.ElementTree(suite).write(log_path.with_suffix(".xml"), encoding="utf-8", xml_declaration=True)
    return passed, failed, skipped


def main() -> int:
    if len(sys.argv) != 2:
        print("Usage: go-test-report.py <test-results-directory>", file=sys.stderr)
        return 2

    directory = Path(sys.argv[1])
    logs = sorted(directory.glob("*.json"))
    if not logs:
        print("No Go test logs found", file=sys.stderr)
        return 1

    lines = [
        "## Результаты автоматического тестирования Go",
        "",
        "| Модуль | Успешно | Ошибки | Пропущено |",
        "|---|---:|---:|---:|",
    ]
    totals = [0, 0, 0]
    for log in logs:
        counts = convert(log)
        totals = [current + value for current, value in zip(totals, counts)]
        lines.append(f"| {log.stem} | {counts[0]} | {counts[1]} | {counts[2]} |")
    lines.append(f"| **Итого** | **{totals[0]}** | **{totals[1]}** | **{totals[2]}** |")
    lines.append("")
    lines.append("Подробные JSON и JUnit XML находятся в артефакте `go-test-results`.")

    summary = "\n".join(lines) + "\n"
    (directory / "summary.md").write_text(summary, encoding="utf-8")
    print(summary)
    github_summary = os.getenv("GITHUB_STEP_SUMMARY")
    if github_summary:
        with open(github_summary, "a", encoding="utf-8") as output:
            output.write(summary)
    return 0


if __name__ == "__main__":
    sys.exit(main())
