# Examples

Two recordings of the CLI in use. Each GIF is generated from the VHS tape next
to it, so the demos stay in step with the interface.

## [Build a cloud](build/.vhs/demo.gif)

Starts from an empty library and fills in all five statements of a real
conflict: a shared objective, a need on each side, and the two wants that
collide. The figure draws itself as the statements land, and the last step
gives the diagram the full width.

Tape: [`build/.vhs/demo.tape`](build/.vhs/demo.tape)

## [Challenge a cloud](dissolve/.vhs/demo.gif)

Starts from a completed cloud in [`dissolve/library`](dissolve/library): five
statements, five claims already surfaced, one of them marked invalid with
evidence, and an alternative that answers it.

The recording opens the challenge pane as a list of the five arrows, steps onto
B-D so it expands under its own row, enters it, walks to the `instead` row and
extends the alternative in place. It then backs out, judges a different claim,
and finishes in the process pane, verifying that the cloud evaporates.

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
