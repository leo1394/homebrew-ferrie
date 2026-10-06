#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TEMP="$(mktemp -d)"
trap 'rm -rf "$TEMP"' EXIT
cd "$TEMP"
touch 'app with spaces.apk'
mkdir 'package directory'
"$ROOT/bin/ferrie" __completion bash > "$TEMP/ferrie.bash"
source "$TEMP/ferrie.bash"
COMP_WORDS=(ferrie --tar)
COMP_CWORD=1
_ferrie
[[ "${COMPREPLY[*]}" == '--target' ]]
COMP_WORDS=(ferrie --target 'app w')
COMP_CWORD=2
_ferrie
[[ "${#COMPREPLY[@]}" == 1 && "${COMPREPLY[0]}" == 'app with spaces.apk' ]]
COMP_WORDS=(ferrie --target 'package')
_ferrie
[[ "${COMPREPLY[0]}" == 'package directory' ]]
# The fixture avoids starting adb while checking device completion.
ferrie() { printf 'serial-1\nserial-2\n'; }
COMP_WORDS=(ferrie --device serial-2)
_ferrie
[[ "${COMPREPLY[*]}" == 'serial-2' ]]
COMP_WORDS=(ferrie --ur)
COMP_CWORD=1
_ferrie || true
[[ "${#COMPREPLY[@]}" == 0 ]]
COMP_WORDS=(ferrie --and)
_ferrie
[[ "${COMPREPLY[*]}" == '--android' ]]
COMP_WORDS=(ferrie --target 'https://example.com/app')
COMP_CWORD=2
_ferrie
[[ "${#COMPREPLY[@]}" == 0 ]]
printf 'Bash completion tests passed\n'
