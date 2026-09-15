#!/usr/bin/env bash
# Mirrors the local pgBackRest repo to Google Drive, encrypted at rest via
# an rclone crypt remote wrapping the Drive remote (section 13.3:
# "encrypted off-host storage"). Requires `rclone config` to have already
# set up the "mortris-gdrive" (type: drive) and "mortris-gdrive-crypt"
# (type: crypt, remote: mortris-gdrive:mortris-pgbackrest) remotes — see
# deploy/backup/README.md for the one-time interactive setup.
set -euo pipefail

REMOTE="${MORTRIS_BACKUP_RCLONE_REMOTE:-mortris-gdrive-crypt:}"
SOURCE="${MORTRIS_BACKUP_SOURCE:-/var/lib/pgbackrest}"

rclone sync "$SOURCE" "$REMOTE" \
  --fast-list \
  --transfers 4 \
  --checkers 8 \
  --log-level INFO
