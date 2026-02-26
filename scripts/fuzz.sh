#!/usr/bin/env bash
set -euo pipefail

# Duration per fuzz target (default: 30s).
FUZZTIME="${FUZZTIME:-30s}"

# Each entry is "package:FuzzFunctionName".
targets=(
  "./internal/tools/:FuzzHTMLToText"
  "./internal/tools/:FuzzParseBody"
  "./internal/tools/:FuzzFormatMessage"
  "./internal/tools/:FuzzFormatFullMessage"
  "./internal/tools/:FuzzPageRange"
  "./internal/tools/:FuzzBuildCriteria"
  "./internal/server/:FuzzServerRun"
)

passed=0
failed=0

for entry in "${targets[@]}"; do
  pkg="${entry%%:*}"
  func="${entry##*:}"

  echo "=== ${func} (${pkg}, ${FUZZTIME}) ==="
  if go test -fuzz="${func}" -fuzztime="${FUZZTIME}" "${pkg}"; then
    passed=$((passed + 1))
  else
    failed=$((failed + 1))
  fi
  echo ""
done

echo "=== Summary: ${passed} passed, ${failed} failed ==="
exit "${failed}"
