#!/usr/bin/env bash
set -euo pipefail

namespace=ktrace-e2e

kubectl apply -f test/integration/manifests.yaml
sleep 60

assert_findings() {
  local kind=$1
  local name=$2
  local expected=$3
  local output
  local code

  set +e
  output=$(./bin/ktrace "$kind" "$name" -n "$namespace" --json 2>&1)
  code=$?
  set -e

  if [[ $code -ne 3 && $code -ne 4 ]]; then
    echo "Expected findings/partial exit for $kind/$name, got $code"
    echo "$output"
    return 1
  fi
  if ! grep -q "$expected" <<<"$output"; then
    echo "Expected $expected in $kind/$name result"
    echo "$output"
    return 1
  fi
}

assert_findings deployment missing-secret MissingSecret
assert_findings deployment failing-init InitContainerFailed
assert_findings deployment failing-probe ProbeFailed
assert_findings deployment oom-regression OOMKilled
assert_findings job failed-job JobFailed
assert_findings statefulset pending-storage PVC

echo "ktrace integration scenarios passed"
