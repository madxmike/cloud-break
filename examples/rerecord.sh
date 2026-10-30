#!/usr/bin/env bash
# Re-record every example demo GIF and commit the result.
#
# Run from anywhere. Needs vhs, ttyd and ffmpeg on PATH.
set -euo pipefail

cd "$(dirname "$0")/.."

for tape in examples/*/.vhs/*.tape; do
	echo "rerecord: $tape"
	vhs "$tape"
done

git add examples
if git diff --cached --quiet; then
	echo "rerecord: GIFs unchanged"
	exit 0
fi

git commit -m "Re-record example GIFs"
