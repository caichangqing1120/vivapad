#include <forward_list>
#include <cstdarg>
#include <cstdio>

#define main lineDecorationMain
#include "line_oracle.cxx"
#undef main

#include "CellBuffer.h"
#include "PerLine.h"

namespace Scintilla::Internal::Platform {
void DebugPrintf(const char *format, ...) noexcept {
    va_list arguments;
    va_start(arguments, format);
    std::vfprintf(stderr, format, arguments);
    va_end(arguments);
}
}

std::string markerLines(const LineMarkers &markers, Sci::Line lines) {
    std::string result;
    for (Sci::Line line = 0; line < lines; ++line) {
        if (markers.MarkValue(line) == 0) continue;
        if (!result.empty()) result += ",";
        result += std::to_string(line);
    }
    return result.empty() ? "-" : result;
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Original Notepad++ dd40fe4 CellBuffer/LineMarkers forward line mutations, not complete native Undo acceptance\n";
    output << "case\tbefore-hex\tafter-hex\tkind\tedits\tinitial-markers\tafter-markers\n";
    for (const auto &scenario : lineScenarios()) {
        const int lines = std::count(scenario.before.begin(), scenario.before.end(), '\n') + 1;
        for (unsigned subset = 0; subset < (1U << lines); ++subset) {
            LineMarkers markers;
            CellBuffer buffer(false, false);
            buffer.SetPerLine(&markers);
            buffer.SetUndoCollection(false);
            bool sequence = false;
            buffer.InsertString(0, scenario.before.data(), scenario.before.size(), sequence);
            for (int line = 0; line < lines; ++line)
                if (subset & (1U << line)) markers.AddMark(line, 24, buffer.Lines());
            const std::string initial = markerLines(markers, buffer.Lines());
            std::string value = scenario.before;
            std::string encodedEdits;
            for (const auto &edit : scenario.edits) {
                if (!encodedEdits.empty()) encodedEdits += ",";
                encodedEdits += std::to_string(edit.at) + ":" + std::to_string(edit.removed) + ":" + hexOf(edit.inserted);
                if (edit.removed) buffer.DeleteChars(edit.at, edit.removed, sequence);
                if (!edit.inserted.empty()) buffer.InsertString(edit.at, edit.inserted.data(), edit.inserted.size(), sequence);
                value.replace(edit.at, edit.removed, edit.inserted);
            }
            std::string kind = "replace";
            if (scenario.action == "split" || scenario.action == "join") kind = scenario.action;
            else if (scenario.action == "empty" || scenario.action == "blank" || scenario.action == "consecutive") kind = "delete";
            output << scenario.name << '\t' << hexOf(scenario.before) << '\t' << hexOf(value) << '\t' << kind << '\t'
                   << (encodedEdits.empty() ? "-" : encodedEdits) << '\t' << initial << '\t'
                   << markerLines(markers, buffer.Lines()) << '\n';
        }
    }
    return output ? 0 : 3;
}
