#!/usr/bin/env bash
set -euo pipefail

# Local HTTPS end-to-end acceptance test. It brings up the Compose stack,
# seeds an isolated project, then proves register, accepted, duplicate,
# partial rejection, and policy behavior through Caddy.
#
# Keep the stack for inspection with MORTRIS_SMOKE_KEEP=1.

root_dir=$(cd "$(dirname "$0")/.." && pwd)
compose=(docker compose -f "$root_dir/deploy/compose.yaml")
base_url=https://localhost:8443
project_id=smoke-$(date +%s)-$RANDOM
install_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
session_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
event_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
next_event_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
invalid_event_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
credential=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')

cleanup() {
  if [[ ${MORTRIS_SMOKE_KEEP:-} != 1 ]]; then
    "${compose[@]}" down --volumes --remove-orphans
  fi
}
trap cleanup EXIT

"${compose[@]}" up --build --detach
for _ in {1..60}; do
  if curl --silent --show-error --fail --insecure "$base_url/health/ready" >/dev/null; then
    break
  fi
  sleep 1
done
curl --silent --show-error --fail --insecure "$base_url/health/ready" >/dev/null

"${compose[@]}" exec -T postgres psql -U postgres -d mortris -v ON_ERROR_STOP=1 \
  -c "INSERT INTO projects (id, environment, display_name, strict_catalog, enabled) VALUES ('$project_id', 'test', '$project_id', false, true);" >/dev/null

register_payload=$(cat <<JSON
{"schema_version":1,"project_id":"$project_id","install_id":"$install_id","installation_credential":"$credential","sdk_name":"smoke","sdk_version":"1.0.0","app_version":"1.0.0","build_number":"1","platform":"android"}
JSON
)
curl --silent --show-error --fail --insecure --header 'Content-Type: application/json' \
  --data "$register_payload" "$base_url/v1/installs/register" >/dev/null
# Lost registration response: the identical retry remains successful.
curl --silent --show-error --fail --insecure --header 'Content-Type: application/json' \
  --data "$register_payload" "$base_url/v1/installs/register" >/dev/null

batch_payload=$(cat <<JSON
{"schema_version":1,"project_id":"$project_id","install_id":"$install_id","sdk":{"name":"smoke","version":"1.0.0"},"sent_at_client":"2026-07-16T12:00:00.000Z","events":[{"event_id":"$event_id","session_id":"$session_id","sequence":1,"session_elapsed_ms":1,"name":"level_start","occurred_at_client":"2026-07-16T12:00:00.000Z","app_version":"1.0.0","build_number":"1","platform":"android","os_version":"15","device_class":"phone","locale":"en-US","timezone_offset_minutes":0,"properties":{}}]}
JSON
)
accepted=$(printf %s "$batch_payload" | gzip | curl --silent --show-error --fail --insecure \
  --header 'Content-Type: application/json' --header 'Content-Encoding: gzip' --header "Authorization: Bearer $credential" \
  --data-binary @- "$base_url/v1/events/batch")
[[ $accepted == *"$event_id"* ]] || { echo "accepted ID missing: $accepted" >&2; exit 1; }

duplicate=$(printf %s "$batch_payload" | gzip | curl --silent --show-error --fail --insecure \
  --header 'Content-Type: application/json' --header 'Content-Encoding: gzip' --header "Authorization: Bearer $credential" \
  --data-binary @- "$base_url/v1/events/batch")
[[ $duplicate == *'"duplicates"'* && $duplicate == *"$event_id"* ]] || { echo "duplicate acknowledgement missing: $duplicate" >&2; exit 1; }

partial_payload=$(cat <<JSON
{"schema_version":1,"project_id":"$project_id","install_id":"$install_id","sdk":{"name":"smoke","version":"1.0.0"},"sent_at_client":"2026-07-16T12:00:00.000Z","events":[{"event_id":"$next_event_id","session_id":"$session_id","sequence":2,"session_elapsed_ms":2,"name":"level_start","occurred_at_client":"2026-07-16T12:00:00.000Z","app_version":"1.0.0","build_number":"1","platform":"android","os_version":"15","device_class":"phone","locale":"en-US","timezone_offset_minutes":0,"properties":{}},{"event_id":"$invalid_event_id","session_id":"$session_id","sequence":3,"session_elapsed_ms":3,"name":"BadName","occurred_at_client":"2026-07-16T12:00:00.000Z","app_version":"1.0.0","build_number":"1","platform":"android","os_version":"15","device_class":"phone","locale":"en-US","timezone_offset_minutes":0,"properties":{}}]}
JSON
)
partial=$(printf %s "$partial_payload" | gzip | curl --silent --show-error --fail --insecure \
  --header 'Content-Type: application/json' --header 'Content-Encoding: gzip' --header "Authorization: Bearer $credential" \
  --data-binary @- "$base_url/v1/events/batch")
[[ $partial == *"$next_event_id"* && $partial == *"$invalid_event_id"* && $partial == *'"rejected"'* ]] || { echo "partial acknowledgement invalid: $partial" >&2; exit 1; }

policy=$(curl --silent --show-error --fail --insecure --header 'Content-Type: application/json' --header "Authorization: Bearer $credential" \
  --data "{\"schema_version\":1,\"project_id\":\"$project_id\",\"install_id\":\"$install_id\",\"sdk\":{\"name\":\"smoke\",\"version\":\"1.0.0\"},\"app_version\":\"1.0.0\",\"build_number\":\"1\",\"platform\":\"android\"}" "$base_url/v1/client/policy")
[[ $policy == *'"client_policy"'* ]] || { echo "policy missing: $policy" >&2; exit 1; }

echo "Mortris local HTTPS smoke test passed."
