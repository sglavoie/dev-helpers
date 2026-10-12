# goback: ejecting

Part of the [goback README](../README.md).

## Ejecting

`goback eject` unmounts the volume holding the active profile's destination.

`goback eject --all` unmounts every external volume the configuration points at:
each profile's source and destination plus the mirror's endpoints, restricted to
paths under `/Volumes` and unmounted once each, in mount-point order. It needs no
profile, so it also works on a machine whose hostname matches none. A volume that
is not mounted is skipped rather than failed, one volume refusing never stops the
others, and the command exits nonzero only when a mounted volume could not be
ejected.

`goback eject --volume NAME` unmounts exactly one configured volume, named
either `Elements` or `/Volumes/Elements`, whichever profile or mirror references
it. It needs no profile, so it is how a drive only the mirror uses is ejected on
its own. A volume the configuration does not reference is refused with the list
of configured ones, a volume that is not mounted is skipped without failing, and
`--volume` cannot be combined with `--profile`, `--all`, or `--list`.

`goback eject --list` ejects nothing. It prints every configured volume that is
currently mounted, each once and in mount-point order, with every profile or
mirror path that references it and the commands that would eject it:

```text
$ goback eject --list
MOUNT POINT        REFERENCED BY                                   EJECT WITH
/Volumes/Elements  mirror destination (/Volumes/Elements/Media)    goback eject --volume Elements
/Volumes/SanDisk   profile default destination (/Volumes/SanDisk)  goback eject --profile default
                   mirror source (/Volumes/SanDisk/Media)

goback eject --all ejects every configured volume that is mounted, whichever profile or mirror references it.
```

`goback eject --profile NAME` only ever unmounts the volume holding that
profile's destination, so it is suggested only for destinations; a volume
referenced solely as a source or by the mirror gets `goback eject --volume NAME`
instead. The listing always covers every profile, needs no hostname
match, rejects `--profile`, and treats `--all` as redundant. An explicit
`--config` is carried into the suggested commands, shell-quoted. When nothing
qualifies it prints `No configured volumes are currently mounted.` and succeeds.

A volume counts as mounted only when `/Volumes/<name>` is a real directory, not a
symlink, on a different filesystem device than `/Volumes` itself, so a leftover
empty directory is neither listed nor ejected. Being listed does not guarantee
the eject succeeds: a busy drive can still refuse.

Ejection is always explicit: `goback mirror` ignores `ejectOnExit` and unmounts
nothing, whatever its outcome.

