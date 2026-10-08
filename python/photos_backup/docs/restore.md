# Rehearse restoring exported media

[Back to the user guide](../README.md)

An export preserves media files and exported metadata. It is not a complete
backup of the Photos library database: copying these files back does not recreate
all albums, sharing relationships, or nondestructive editing history. Keep a
separate Photos-library backup if you need to restore that application state.

## Try a small recovery

1. Choose a few known items in the primary archive: an original photo, an edited
   version, a video, and (if used) both components of a Live Photo or RAW/JPEG pair.
   Include the associated JSON and AAE sidecars. Preserve their filenames and
   relative year/month directories when gathering them.
2. Find the same relative paths on the SSD backup. The SSD layout includes the
   archive directory name beneath `ssd.destination`. For cloud recovery, locate
   them beneath the configured remote using its usual download interface. Check
   the configured routes with `photos-backup status` first.
3. Create a new empty folder, such as `~/Desktop/photos-restore-rehearsal`, and
   copy or download only these sample files into it. Do not use a mirror or sync
   operation, replace your archive, or point an export at this folder.
4. Open the recovered photos and play the videos. Check resolution, capture dates,
   and the edited appearance where applicable. Open JSON sidecars in a text editor
   and confirm that the expected metadata is present. Keep paired files and AAE
   sidecars together; retaining them does not guarantee every application will
   reconstruct the original Photos behavior.
5. Compare each recovered file with the corresponding primary-archive file:

   ```sh
   shasum -a 256 "/path/to/archive/2026/03/example.jpg" \
     "$HOME/Desktop/photos-restore-rehearsal/2026/03/example.jpg"
   ```

   The two hashes should match. Repeat for the video and sidecars. This checks
   equality with today's primary copy; it does not establish that the original
   file was undamaged before backup.
6. Record the date, backup source, sample paths, and result somewhere outside the
   archive. Repeat from the cloud separately so that SSD success is not mistaken
   for cloud recovery success. Remove the temporary recovery folder when finished.

`photos-backup verify` checks the primary archive's records, sizes, and timestamps.
It does not perform this recovery exercise or verify SSD/cloud file contents.

## Replace a lost primary drive

The SSD copy includes the archive's `.photos-backup/` directory: its export
database, state, reports, and any pending cleanup. A new drive filled from that
copy therefore continues the same history rather than starting a fresh
bootstrap. The export database records file paths relative to the archive, so the
archive works from a new location. `ssd` holds the archive lock while it copies,
so the database and the files in the copy match each other.

1. Check how current the SSD copy is. `photos-backup status` shows when the
   `SSD: All Photos` copy last succeeded. Anything exported after that is not in
   the copy, but the next export fills the gap (step 5).
2. Check that no `[ssd] exclude_file` pattern matches `.photos-backup`. If one
   does, the copy has media but no history; use `photos-backup bootstrap`
   instead.
3. Copy the archive directory from the SSD to the new drive. Leave out the
   trailing slash on the source so that the directory itself is copied:

   ```sh
   mkdir -p "/Volumes/NEW/Media"
   rsync -ah --stats -- "/Volumes/Data/Pictures/Apple Photos" "/Volumes/NEW/Media"
   ```

   Do not point `[apple_photos] archive` at the SSD copy itself. The SSD would
   then be both the archive and its own backup.
4. Give the new drive the old volume name so the configuration still applies, or
   update `[apple_photos] volume` and `archive` and `[ssd] source`. For a single
   trial run, `photos-backup --volume /Volumes/NEW ...` re-roots the archive
   without editing the configuration.
5. Check the archive before writing to it:

   ```sh
   photos-backup doctor
   photos-backup verify             # every database record should match a file
   photos-backup daily --dry-run    # shows the export that will fill the gap
   ```

   `rsync -a` keeps file sizes and modification times, so `verify` should
   report no changed signatures. The copied `archive.lock` is harmless: locks
   are held only by running processes, not by the file's contents. Then run
   `photos-backup daily`. Incremental exports start
   `incremental_overlap_days` before the last export that the copied state
   records, so they also cover photos added since the SSD copy was made.
