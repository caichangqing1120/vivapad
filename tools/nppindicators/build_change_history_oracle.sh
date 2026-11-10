#!/bin/sh
set -eu

source_root=${1:?original source export required}
output_root=${2:?owned output directory required}
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
compiler=${CXX:-c++}
build_mode=${3:-release}
sanitizer_flag=-fno-sanitize=all
case "$build_mode" in
    release) mode_flag=-DNDEBUG ;;
    debug) mode_flag=-D_DEBUG ;;
    asan) mode_flag=-DNDEBUG; sanitizer_flag=-fsanitize=address ;;
    *) printf '%s\n' 'build mode must be release, debug or asan' >&2; exit 1 ;;
esac
case "${4:-observe}" in
    observe) observer_flag=-DVIVAPAD_OBSERVED_ORACLE=1 ;;
    plain) observer_flag=-DVIVAPAD_UNOBSERVED_ORACLE=1 ;;
    *) printf '%s\n' 'observation mode must be observe or plain' >&2; exit 1 ;;
esac
mkdir -p "$output_root"

for source in CellBuffer UndoHistory PerLine RunStyles UniConversion; do
    "$compiler" -std=c++20 -O2 "$mode_flag" "$observer_flag" "$sanitizer_flag" \
        -I "$source_root/scintilla/include" -I "$source_root/scintilla/src" \
        -include "$script_root/testdata/change_history_observer.h" \
        -c "$source_root/scintilla/src/$source.cxx" -o "$output_root/$source.o"
done
"$compiler" -std=c++20 -O2 "$mode_flag" "$observer_flag" "$sanitizer_flag" \
    -I "$source_root/scintilla/include" -I "$source_root/scintilla/src" \
    -c "$source_root/scintilla/src/ChangeHistory.cxx" -o "$output_root/ChangeHistory.o"
"$compiler" -std=c++20 -O2 "$mode_flag" "$observer_flag" "$sanitizer_flag" \
    -I "$source_root/scintilla/include" -I "$source_root/scintilla/src" \
    "$script_root/testdata/change_history_oracle.cxx" \
    "$output_root/CellBuffer.o" "$output_root/UndoHistory.o" \
    "$output_root/PerLine.o" "$output_root/RunStyles.o" \
    "$output_root/UniConversion.o" "$output_root/ChangeHistory.o" \
    -o "$output_root/change-history-oracle"
