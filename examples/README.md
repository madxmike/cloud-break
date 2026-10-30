# Examples

Two recordings of the CLI in use. Each GIF is generated from the VHS tape next
to it, so the demos stay in step with the interface.

## [Build a cloud](build/.vhs/demo.gif)

Starts from an empty library and fills in all five statements of a real
conflict: a shared objective, a need on each side, and the two wants that
collide. The figure draws itself as the statements land, and the last step
gives the diagram the full width.

Tape: [`build/.vhs/demo.tape`](build/.vhs/demo.tape)

## [Dissolve a cloud](dissolve/.vhs/demo.gif)

Starts from a complete cloud in [`dissolve/library`](dissolve/library), surfaces
one assumption behind each arrow, then challenges the B-D assumption — *the only
way to avoid regressions is to stop changing code* — until it is marked invalid.
An injection against it, paired with an automated regression test, breaks the
arrow. The process pane runs the five-stage method and the cloud is verified as
evaporated.

Tape: [`dissolve/.vhs/demo.tape`](dissolve/.vhs/demo.tape)

## Re-recording

The tapes need [VHS](https://github.com/charmbracelet/vhs), which in turn needs
`ttyd` and `ffmpeg` on `PATH`. Each tape builds the CLI and seeds a throwaway
library under `/tmp`, so nothing outside the repository is touched.

Record everything and commit the GIFs:

```sh
./examples/rerecord.sh
```

Or record one:

```sh
vhs examples/dissolve/.vhs/demo.tape
```
