# Mortris backups (section 13.3)

## What's running

- **Continuous WAL archiving**: PostgreSQL's `archive_command` pushes every
  WAL segment to the local pgBackRest repo (`/var/lib/pgbackrest`) as it's
  generated — this is what gets the RPO target down near-zero, not the
  periodic full/diff backups below.
- **Monthly full backup** (`mortris-backup-full.timer`) and **weekly
  differential backup** (`mortris-backup-diff.timer`) — restore points.
  Retention: `repo1-retention-full=6` (6 months), `repo1-retention-diff=4`
  (4 weeks) in `/etc/pgbackrest.conf`.
- **Off-host sync** (`mortris-backup-sync.timer`, every 30 min): mirrors
  the local pgBackRest repo to Google Drive via an rclone crypt remote
  (encrypted at rest off-host, per section 13.3).

See `pgbackrest.conf`'s header comment for how this maps onto the plan's
"7 daily, 4 weekly, 6 monthly recovery points" — short version: continuous
WAL archiving gives point-in-time recovery to *any* moment, which is
strictly better than 7 fixed daily snapshots, so there's no separate daily
full/diff schedule.

## One-time setup: rclone + Google Drive (must be done interactively)

This needs a real OAuth consent in a browser — it can't be scripted.
Run as the `postgres` user (that's who owns the local repo and runs the
sync):

```sh
sudo -u postgres rclone config
```

1. `n` (new remote), name it `mortris-gdrive`, type `drive` (Google
   Drive). Accept defaults for scope (`drive` = full access) unless you'd
   rather scope it to a folder — either works since sync targets a
   specific subfolder anyway. When it asks to use auto-config, if this
   session has no local browser, answer `n` and it prints a URL + a
   `rclone authorize "drive"` command — run that command on a machine
   that *does* have a browser, approve access, then paste the resulting
   token back into this prompt.
2. `n` again for a second remote, name it `mortris-gdrive-crypt`, type
   `crypt`. Remote: `mortris-gdrive:mortris-pgbackrest` (this creates the
   folder on Drive automatically on first sync). Filename/directory name
   encryption: `standard`. Password: generate one when prompted (`g` for
   random) and **save it somewhere durable outside this VPS** — losing it
   means the off-host backups are permanently unreadable, which defeats
   the entire point of having them.
3. Verify: `sudo -u postgres rclone lsd mortris-gdrive-crypt:` should
   succeed (empty listing is fine, means it's connected).
4. Enable the sync timer: `sudo systemctl enable --now
   mortris-backup-sync.timer`.

## Restore

See `deploy/backup/restore-drill.md` for the exact commands — practiced
and documented on 2026-07-17, not just theoretical.
