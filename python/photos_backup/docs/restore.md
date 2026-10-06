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
