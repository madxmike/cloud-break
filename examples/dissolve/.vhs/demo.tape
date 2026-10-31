# Walk a completed cloud through the challenge pane: expand an arrow, judge a
# claim, change the alternative, then verify the cloud.
#
# Starts from a filled cloud in examples/dissolve/library so the recording can
# spend its time on the work rather than on typing five statements.
#
# Run from the repository root: vhs examples/dissolve/.vhs/demo.tape
Output examples/dissolve/.vhs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1620
Set Height 950
Set Padding 20
Set TypingSpeed 15ms
Set PlaybackSpeed 0.8
Set CursorBlink false

# Build the CLI and seed the library without showing any of it.
Hide
Type "go build -o cloud-break . && rm -rf /tmp/cloud-break-dissolve && cp -r examples/dissolve/library /tmp/cloud-break-dissolve"
Enter
Sleep 3s
Show

# The library holds one cloud, already complete.
Type "./cloud-break -dir /tmp/cloud-break-dissolve"
Enter
Sleep 2s

Enter
Sleep 2s

# The challenge pane opens as a list of the five arrows.
Type "3"
Sleep 3s

# Step to B-D. It opens under its own row as the cursor lands on it.
Down
Sleep 1200ms
Down
Sleep 3s

# Enter the expansion; the marker moves from the arrow to its first row.
Enter
Sleep 2s

# Walk to the alternative that answers this claim.
Down
Sleep 1500ms
Down
Sleep 2s

# Change the alternative where it sits.
Type "i"
Sleep 1200ms
Type " on every pull request"
Sleep 1200ms
Enter
Sleep 3s

# Back out to the arrows, then judge a different claim.
Escape
Sleep 2s
Down
Sleep 2s

Enter
Sleep 2s
Type "v"
Sleep 2s
Type "v"
Sleep 3s

# Back out to the arrow list, look at the process, and confirm the cloud
# evaporates.
Escape
Sleep 1500ms
Type "4"
Sleep 3s
Type "V"
Sleep 3s

Hide
Type "q"
Sleep 500ms
Type "q"
Sleep 500ms
