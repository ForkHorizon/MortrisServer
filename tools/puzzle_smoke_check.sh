#!/usr/bin/env bash
set -euo pipefail

# puzzle_smoke_check.sh — Automated QA smoke verification gate for Puzzle schema-v2 builds.
# Evaluates target candidate build against the 13-point Android smoke checklist in Section 10.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/bin/analytics-server"

usage() {
  cat << 'HELP'
Usage:
  puzzle_smoke_check.sh -b <build_number> [-p <project_id>] [-f <from_rfc3339>] [-t <to_rfc3339>] [--json]
  puzzle_smoke_check.sh --fixture [--json]

Environment:
  MORTRIS_WRITER_DSN   Database write connection string (or MORTRIS_READER_DSN)

Options:
  -b, --build <num>     Candidate build number to verify (required unless --fixture is used)
  -p, --project <id>    Target project ID (default: puzzle)
  -f, --from <time>     Start of evaluation window (default: 24h ago)
  -t, --to <time>       End of evaluation window (default: now)
      --fixture         Execute self-verification against master schema-v2 fixture
      --json            Output report in JSON format
  -h, --help            Show this help message
HELP
  exit 2
}

PROJECT="puzzle"
BUILD=""
FROM=""
TO=""
JSON_FLAG=""
RUN_FIXTURE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -b|-build|--build)
      if [[ $# -lt 2 ]]; then echo "Error: $1 requires an argument" >&2; usage; fi
      BUILD="$2"; shift 2 ;;
    -p|-project|--project)
      if [[ $# -lt 2 ]]; then echo "Error: $1 requires an argument" >&2; usage; fi
      PROJECT="$2"; shift 2 ;;
    -f|-from|--from)
      if [[ $# -lt 2 ]]; then echo "Error: $1 requires an argument" >&2; usage; fi
      FROM="$2"; shift 2 ;;
    -t|-to|--to)
      if [[ $# -lt 2 ]]; then echo "Error: $1 requires an argument" >&2; usage; fi
      TO="$2"; shift 2 ;;
    -fixture|--fixture)
      RUN_FIXTURE="true"; shift ;;
    -json|--json)
      JSON_FLAG="--json"; shift ;;
    -h|-help|--help)
      usage ;;
    *)
      echo "Unknown option: $1" >&2
      usage ;;
  esac
done

if [[ -z "$BUILD" && -z "$RUN_FIXTURE" ]]; then
  echo "Error: -b/--build is required (or specify --fixture)" >&2
  usage
fi

# Ensure analytics-server binary is built and up-to-date with Go sources
NEED_BUILD=0
if [[ ! -x "$BIN_PATH" ]]; then
  NEED_BUILD=1
elif find "${ROOT_DIR}/cmd" "${ROOT_DIR}/internal" -name '*.go' -newer "$BIN_PATH" 2>/dev/null | grep -q .; then
  NEED_BUILD=1
fi

if [[ "$NEED_BUILD" -eq 1 ]]; then
  echo "Building analytics-server binary..." >&2
  mkdir -p "${ROOT_DIR}/bin"
  go build -o "$BIN_PATH" "${ROOT_DIR}/cmd/analytics-server"
fi

ARGS=()
if [[ -n "$RUN_FIXTURE" ]]; then
  ARGS+=("-run-fixture")
else
  ARGS+=("-project" "$PROJECT" "-build" "$BUILD")
  [[ -n "$FROM" ]] && ARGS+=("-from" "$FROM")
  [[ -n "$TO" ]] && ARGS+=("-to" "$TO")
fi
[[ -n "$JSON_FLAG" ]] && ARGS+=("-json")

exec "$BIN_PATH" smoke-check "${ARGS[@]}"
