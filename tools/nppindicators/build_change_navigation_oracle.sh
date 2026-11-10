#!/bin/sh
set -eu

source_root=${1:?pinned original source export required}
output_root=${2:?owned output directory required}
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
mkdir -p "$output_root"

python3 - "$source_root" "$output_root" <<'PY'
import hashlib
from pathlib import Path
import sys

source, output = map(Path, sys.argv[1:])
expected = {
    "PowerEditor/src/Notepad_plus.cpp": "0fcc7b1b2f2650f7f692d96dac4369ff9783e008",
    "scintilla/src/Document.cxx": "e559861f8908747b76aa5762e18ad199182e33d6",
    "scintilla/src/Editor.cxx": "1465bac719489105816601c439440298924011b7",
}
sources = {}
for name, wanted in expected.items():
    raw = (source / name).read_bytes()
    blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
    if blob != wanted:
        raise SystemExit(f"not the pinned dd40fe4 source: {name}: {blob}")
    sources[name] = raw.decode()

def function(name, signature):
    text = sources[name]
    start = text.index(signature)
    brace = text.index("{", start)
    depth = 1
    end = brace + 1
    # These pinned methods contain no braces in quoted strings or comments.
    while depth:
        depth += (text[end] == "{") - (text[end] == "}")
        end += 1
    return text[start:end] + "\n"

(output / "original_line_mark.inc").write_text(function(
    "scintilla/src/Document.cxx", "int Document::GetMark("))
(output / "original_change_navigation.inc").write_text(function(
    "PowerEditor/src/Notepad_plus.cpp", "void Notepad_plus::changedHistoryGoTo("))
text = sources["scintilla/src/Editor.cxx"]
start = text.index("case Message::MarkerPrevious:")
end = text.index("case Message::MarkerDefinePixmap:", start)
(output / "original_marker_previous.inc").write_text(text[start:end])
PY

"${CXX:-c++}" -std=c++20 -O2 -Wall -Wextra -Werror \
    -I "$source_root/scintilla/include" -I "$output_root" \
    "$script_root/testdata/change_navigation_oracle.cxx" \
    -o "$output_root/change-navigation-oracle"
