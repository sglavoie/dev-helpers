# goback

Revamped version of [rsync backup](../../python/rsync_backup/README.md), ported to Go.

## Usage

See available commands:

```bash
make
```

---

## TODO

- [ ] Add command to remove logs.
  - [ ] Should have an option to keep latest N logs.
- [ ] Add a way to run it as a service (e.g., `gocron`).
- [ ] Should allow specifying which source/destination pair to use at runtime for each backup type. Can just do an incremental daily backup for now.
