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

## Example clouds

[`library/`](library) holds ten clouds to browse, each in a different state.
Five are everyday conflicts; five are the classic hard problems, the ones where
the usual answer is a trade-off and the way out is a change of assumption.

| Cloud | Conflict | State |
|:--|:--|:--|
| Change control | release cadence vs the change board | dissolving |
| Tech debt quarter | the rewrite vs the roadmap | open |
| On-call load | paging for everything vs resting the team | dissolved |
| Hiring bar | years of Go vs potential | dissolving |
| Split the monolith | services vs one deployable | draft |
| Consistency or availability | linearizable reads vs answering during a partition | dissolving |
| Cache invalidation | instant pages vs never stale | dissolved |
| Retries without duplicates | retrying vs charging twice | dissolving |
| Schema change without downtime | compatibility vs a clean schema | open |
| Rewrite or refactor | replacing the service vs shipping through it | dissolving |

The hard five each turn on an assumption that is not true as stated: that every
read needs the same consistency, that staleness has to be bounded by time, that
a server cannot tell a retry from a new request, that a rename must land with
its code, that a structure cannot be improved in place.

Copy them into your own library and open them, or let the binary bring its own:

```sh
cloud-break --demo   # a temporary library seeded with all of these
```
