# Build a complete evaporating cloud from an empty library.
#
# Run from the repository root: vhs examples/build/.vhs/demo.tape
Output examples/build/.vhs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1500
Set Height 950
Set Padding 20
Set TypingSpeed 15ms
Set PlaybackSpeed 0.8
Set CursorBlink false

# Build the CLI and start from an empty library without showing any of it.
Hide
Type "go build -o cloud-break . && rm -rf /tmp/cloud-break-build"
Enter
Sleep 3s
Show

Type "./cloud-break -dir /tmp/cloud-break-build"
Enter
Sleep 2200ms

# Name the conflict.
Type "n"
Sleep 400ms
Type@50ms "Steady pace"
Sleep 300ms
Enter
Sleep 800ms

# A: the objective both sides share.
Enter
Sleep 400ms
Type@20ms "keep the team shipping at a steady pace"
Sleep 300ms
Enter
Sleep 600ms

# B: what our side needs.
Down
Enter
Sleep 300ms
Type@20ms "protect focus time"
Sleep 300ms
Enter
Sleep 600ms

# C: what their side needs.
Down
Enter
Sleep 300ms
Type@20ms "answer every customer escalation"
Sleep 300ms
Enter
Sleep 600ms

# D: what our side wants to do about it.
Down
Enter
Sleep 300ms
Type@20ms "hold one no-meeting day each week"
Sleep 300ms
Enter
Sleep 600ms

# D': what their side wants to do.
Down
Enter
Sleep 300ms
Type@20ms "take every escalation the day it lands"
Sleep 300ms
Enter
Sleep 1200ms

# The figure is complete. Give it the whole width for a moment.
Type "2"
Sleep 4s

Hide
Type "q"
Sleep 500ms
Type "q"
Sleep 500ms
