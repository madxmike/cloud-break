# cloud-break

A terminal tool for building and processing **evaporating clouds** — the
conflict resolution diagrams from Goldratt's Theory of Constraints.

A cloud states a disagreement without proposing a solution: a shared objective,
one need on each side, and the two wants that collide. Behind each of those five
statements sits an assumption. The cloud evaporates when one of those
assumptions turns out to be false and an injection makes it irrelevant.

```
                    ╭─ A · Objective ─────╮
                    │ ship on time        │
                    ╰──────────┬──────────╯
                               │
             ┌─────────────────┴─────────────────┐
             │                                   │
╭─ B · Need (Us) ─────╮             ╭─ C · Need (Them) ───╮
│ no regressions      │             │ promised features   │
╰──────────┬──────────╯             ╰──────────┬──────────╯
╭─ D · Want (Us) ─────╮             ╭─ D' · Want (Them) ──╮
│ freeze the scope    │◄─────×─────►│ add every feature   │
╰─────────────────────╯             ╰─────────────────────╯

assumptions  A-B 0 · A-C 0 · B-D 2 · C-D' 0 · D-D' 1
```

Read down: in order to achieve A you must satisfy B and C; in order to satisfy
B you must do D, and in order to satisfy C you must do D'. The `×` is the
conflict. The ledger under the figure counts the assumptions on each arrow,
coloured by the weakest verdict among them.

## Installation

```sh
go install github.com/madxmike/cloud-break@latest
```

Prebuilt binaries for Linux, macOS and Windows (amd64 and arm64) are on the
[releases page](https://github.com/madxmike/cloud-break/releases). Building from
source is just:

```sh
go build -o cloud-break .
```

## Running it

```sh
cloud-break              # opens the library
cloud-break -dir ./clouds
cloud-break -version
```

Clouds live in one JSON file each, under the first of:

1. `-dir` on the command line
2. `$CLOUD_BREAK_HOME`
3. `$XDG_DATA_HOME/cloud-break`
4. `~/.local/share/cloud-break`

Saves are atomic — written to a temporary file and renamed into place — so an
interrupted write cannot truncate a cloud.

## Processing a cloud

The **process** pane tracks the five stages and derives each one from the
cloud's own contents:

1. **State the conflict** — all five statements are filled in.
2. **Surface the assumptions** — every arrow has at least one "because".
3. **Find the weak link** — an assumption has been judged invalid.
4. **Inject a breakthrough** — an injection breaks that assumption.
5. **Verify** — you confirm the injection dissolves the conflict.

A cloud's status follows the same evidence: `draft` until the statements are
complete, `open` while nothing has fallen, `dissolving` once an assumption is
invalid, and `dissolved` only after an injection breaks it *and* you verify it.
Nothing is ever marked dissolved on your behalf.

## Keys

| Where | Keys |
|:--|:--|
| Library | `↑/k` `↓/j` move · `enter` open · `n` new · `d` delete · `q` quit |
| Statements | `↑/k` `↓/j` move · `enter` edit · `t` title · `s` name the sides |
| Cloud figure | `2` — the diagram has its own pane on narrow terminals |
| Assumptions | `↑/k` `↓/j` move · `a` add · `enter` edit · `v` verdict · `e` evidence · `i` inject · `x` delete |
| Injections | `↑/k` `↓/j` move · `a` add · `enter` edit · `r` why it works · `x` delete · `V` verify |
| Process | `V` verify |
| Anywhere | `1`–`5` jump to pane · `tab` `shift+tab` cycle · `?` help · `esc/q` back · `^C` quit |

Verdicts cycle through unassessed, holds, questionable and invalid. The verdict
is a judgement about the *assumption*, not about the person who holds it: mark
one invalid when you can show it does not hold, then say what breaks it.

## Development

```sh
go test ./...        # 100+ tests, including the layout invariants
go vet ./...
```

Built with [Bubble Tea v2](https://charm.land), Bubbles and Lip Gloss. The
layout is pinned by tests rather than by hand: every screen renders exactly the
terminal height, no line exceeds the terminal width, and the help sheet
degrades from two columns to one until its box fits, so it can never lose its
bottom border.

The package is deliberately flat: one `package main` at the repository root,
one file per concern (shell, library, cloud screen, domain, storage, editor).

## License

MIT. See [LICENSE](LICENSE).
