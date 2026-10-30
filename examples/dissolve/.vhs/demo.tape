# Challenge a cloud's assumptions, inject against the weak one, and verify
# that it evaporates.
#
# Starts from a complete cloud in examples/dissolve/library so the recording
# can spend its time on the part that matters: finding the assumption that
# does not hold.
#
# Run from the repository root: vhs examples/dissolve/.vhs/demo.tape
Output examples/dissolve/.vhs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1500
Set Height 950
Set Padding 20
Set TypingSpeed 15ms
Set PlaybackSpeed 1.2
Set CursorBlink false

# Build the CLI and seed the library without showing any of it.
Hide
Type "go build -o cloud-break . && rm -rf /tmp/cloud-break-dissolve && cp -r examples/dissolve/library /tmp/cloud-break-dissolve"
Enter
Sleep 3s
Show

Type "./cloud-break -dir /tmp/cloud-break-dissolve"
Enter
Sleep 1200ms

# Open the only cloud in the library.
Enter
Sleep 1200ms

# Move to the assumptions: the "because" behind each arrow.
Type "3"
Sleep 700ms

# Surface one assumption per arrow. After each one the cursor lands on it, so
# a single Down moves to the next arrow heading.
Type "a"
Sleep 350ms
Type@18ms "a regression costs more than a late feature"
Sleep 250ms
Enter
Sleep 700ms

Down
Type "a"
Sleep 350ms
Type@18ms "sales has committed every feature in writing"
Sleep 250ms
Enter
Sleep 700ms

Down
Type "a"
Sleep 350ms
Type@18ms "the only way to avoid regressions is to stop changing code"
Sleep 250ms
Enter
Sleep 700ms

Down
Type "a"
Sleep 350ms
Type@18ms "features cannot be split across releases"
Sleep 250ms
Enter
Sleep 700ms

Down
Type "a"
Sleep 350ms
Type@18ms "code that keeps changing cannot be safe"
Sleep 250ms
Enter
Sleep 900ms

# Back up to the B-D assumption, and challenge it: unassessed, holds,
# questionable, invalid.
Up
Up
Up
Up
Sleep 500ms
Type "v"
Sleep 350ms
Type "v"
Sleep 350ms
Type "v"
Sleep 900ms

# The weak link is found. Inject against it.
Type "i"
Sleep 500ms
Type@18ms "pair every change with an automated regression test"
Sleep 250ms
Enter
Sleep 1000ms

# The breakthrough, listed against what it breaks.
Type "4"
Sleep 2000ms

# Run the method and confirm the cloud evaporates.
Type "5"
Sleep 700ms
Type "V"
Sleep 3500ms

Hide
Type "q"
Sleep 400ms
Type "q"
Sleep 400ms
