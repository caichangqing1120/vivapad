#include <forward_list>
#include <cstdarg>
#include <cstdio>

#define main lineDecorationMain
#include "line_oracle.cxx"
#undef main

#include "CellBuffer.h"
#include "PerLine.h"
#include "UndoHistory.h"

bool includeLineFacts = false;

namespace Scintilla::Internal::Platform {
void DebugPrintf(const char *format, ...) noexcept {
    va_list arguments;
    va_start(arguments, format);
    std::vfprintf(stderr, format, arguments);
    va_end(arguments);
}
}

std::string markedLines(const LineMarkers &markers, Sci::Line lines) {
    std::string result;
    for (Sci::Line line = 0; line < lines; ++line) {
        if (markers.MarkValue(line) == 0) continue;
        if (!result.empty()) result += ",";
        result += std::to_string(line);
    }
    return result.empty() ? "-" : result;
}

void appendEvent(std::string &events, const std::string &value, const Mutation &edit) {
    if (!edit.removed && edit.inserted.empty()) return;
    if (!events.empty()) events += ",";
    const bool lineStart = edit.at == 0 || value.at(edit.at - 1) == '\n';
    const bool insertedLF = edit.inserted.find('\n') != std::string::npos;
    events += std::to_string(edit.at) + ":" + std::to_string(edit.removed) + ":" +
        std::to_string(edit.inserted.size()) + ":" + (lineStart ? "1" : "0") + ":" +
        (insertedLF ? "1" : "0");
    if (includeLineFacts) {
        const auto line = std::count(value.begin(), value.begin()+edit.at, '\n');
        const auto removed = std::count(value.begin()+edit.at, value.begin()+edit.at+edit.removed, '\n');
        const auto inserted = std::count(edit.inserted.begin(), edit.inserted.end(), '\n');
        events += ":" + std::to_string(line) + ":" + std::to_string(removed) + ":" + std::to_string(inserted);
    }
}

void markerHistory(std::ofstream &output, const Scenario &scenario, unsigned subset, bool toggleLiveMarks) {
    LineMarkers markers;
    CellBuffer buffer(false, false);
    buffer.SetPerLine(&markers);
    buffer.SetUndoCollection(false);
    bool sequence = false;
    buffer.InsertString(0, scenario.before.data(), scenario.before.size(), sequence);
    buffer.SetUndoCollection(true);
    for (Sci::Line line = 0; line < buffer.Lines(); ++line)
        if (subset & (1U << line)) markers.AddMark(line, 24, buffer.Lines());
    std::string value = scenario.before;
    std::string before = value;
    std::string initial = markedLines(markers, buffer.Lines());
    std::string events;
    const std::string name = scenario.name + "/" + std::to_string(subset) + (toggleLiveMarks ? "/live" : "");
    auto row = [&](const std::string &phase) {
        std::string actual(buffer.Length(), '\0');
        buffer.GetCharRange(actual.data(), 0, buffer.Length());
        if (actual != value) std::abort();
        output << name << '\t' << phase << '\t' << hexOf(before) << '\t' << hexOf(value) << '\t'
               << initial << '\t' << (events.empty() ? "-" : events) << '\t'
               << markedLines(markers, buffer.Lines());
        if (includeLineFacts) {
            output << '\t';
            if (scenario.edits.empty()) output << '-';
            for (size_t index = 0; index < scenario.edits.size(); ++index) {
                if (index) output << ',';
                const auto &edit = scenario.edits[index];
                output << edit.at << ':' << edit.removed << ':' << hexOf(edit.inserted);
            }
        }
        output << '\n';
        before = value;
        initial = markedLines(markers, buffer.Lines());
        events.clear();
    };
    buffer.BeginUndoAction();
    for (const auto &edit : scenario.edits) {
        appendEvent(events, value, edit);
        if (edit.removed) buffer.DeleteChars(edit.at, edit.removed, sequence);
        if (!edit.inserted.empty()) buffer.InsertString(edit.at, edit.inserted.data(), edit.inserted.size(), sequence);
        value.replace(edit.at, edit.removed, edit.inserted);
    }
    buffer.EndUndoAction();
    row("forward");
    if (toggleLiveMarks) {
        for (Sci::Line line = 0; line < buffer.Lines(); ++line) {
            if (markers.MarkValue(line) == 0) markers.AddMark(line, 24, buffer.Lines());
            else markers.DeleteMark(line, 24, true);
        }
        initial = markedLines(markers, buffer.Lines());
    }
    const int undoSteps = buffer.StartUndo();
    for (int step = 0; step < undoSteps; ++step) {
        const Action action = buffer.GetUndoStep();
        Mutation edit{static_cast<int>(action.position), 0, ""};
        if (action.at == ActionType::insert) edit.removed = static_cast<int>(action.lenData);
        else if (action.at == ActionType::remove) edit.inserted.assign(action.data, action.lenData);
        else std::abort();
        appendEvent(events, value, edit);
        value.replace(edit.at, edit.removed, edit.inserted);
        buffer.PerformUndoStep();
    }
    row("undo");
    if (value != scenario.before) std::abort();
    const int redoSteps = buffer.StartRedo();
    for (int step = 0; step < redoSteps; ++step) {
        const Action action = buffer.GetRedoStep();
        Mutation edit{static_cast<int>(action.position), 0, ""};
        if (action.at == ActionType::insert) edit.inserted.assign(action.data, action.lenData);
        else if (action.at == ActionType::remove) edit.removed = static_cast<int>(action.lenData);
        else std::abort();
        appendEvent(events, value, edit);
        value.replace(edit.at, edit.removed, edit.inserted);
        buffer.PerformRedoStep();
    }
    row("redo");
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2 && argumentCount != 3) return 1;
    if (argumentCount == 3) {
        if (std::string(arguments[2]) != "--line-facts") return 1;
        includeLineFacts = true;
    }
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Original Notepad++ dd40fe4 CellBuffer/LineMarkers actual forward, Undo and Redo; normalized LF scope\n";
    output << "case\tphase\tbefore-hex\tafter-hex\tinitial-markers\tevents\tafter-markers";
    if (includeLineFacts) output << "\tforward-mutations";
    output << '\n';
    auto scenarios = lineScenarios();
    scenarios.push_back({"transient-text", "marker", "ab", 0, 0, "-", {{0, 0, "x"}, {1, 0, "\n"}, {0, 1, ""}}});
    scenarios.push_back({"transient-lf", "marker", "ab", 0, 0, "-", {{0, 0, "\n"}, {1, 0, "\n"}, {0, 1, ""}}});
    scenarios.push_back({"whole-equal", "marker", "a\nb", 0, 0, "-", {{0, 3, "a\nb"}}});
    scenarios.push_back({"whole-delete", "marker", "a\nb", 0, 0, "-", {{0, 3, ""}}});
    scenarios.push_back({"from-empty", "marker", "", 0, 0, "-", {{0, 0, "a\nb"}}});
    scenarios.push_back({"unicode-merge", "marker", "\xe7\x8c\xab\n\xc3\xa9\nlast", 0, 0, "-", {{3, 1, ""}, {6, 0, "\n"}}});
    if (includeLineFacts) {
        scenarios.push_back({"merge-then-split-before-anchor", "marker", "aa\nbb", 0, 0, "-", {{2, 1, ""}, {1, 0, "\n"}}});
        scenarios.push_back({"merge-then-split-at-anchor", "marker", "aa\nbb", 0, 0, "-", {{2, 1, ""}, {2, 0, "\n"}}});
        scenarios.push_back({"merge-then-prefix-lines", "marker", "aa\nbb\ncc", 0, 0, "-", {{2, 1, ""}, {1, 0, "x\ny\n"}, {0, 1, ""}}});
        scenarios.push_back({"insert-transient-lines", "marker", "aa\nbb", 0, 0, "-", {{0, 0, "x\ny"}, {1, 0, "\n"}, {0, 2, ""}}});
        scenarios.push_back({"delete-then-shift-in-merged-line", "marker", "head\nbody\ntail", 0, 0, "-", {{4, 1, ""}, {2, 0, "\n"}, {1, 2, ""}}});
        scenarios.push_back({"two-merges-then-inner-insert", "marker", "aa\nbb\ncc", 0, 0, "-", {{5, 1, ""}, {2, 1, ""}, {1, 0, "\n\n"}}});
        scenarios.push_back({"merge-prefix-unicode", "marker", "\xe7\x8c\xab\xe7\x8c\xab\n\xc3\xa9\xc3\xa9", 0, 0, "-", {{6, 1, ""}, {3, 0, "\n"}}});
        std::mt19937 random(20261109);
        const std::vector<std::string> sources{"aa\nbb", "aa\nbb\ncc", "x\ny\n"};
        const std::vector<std::string> insertions{"", "x", "\n", "x\ny", "\n\n", "\xe7\x8c\xab", std::string("!\0?", 3)};
        for (int index = 0; index < 64; ++index) {
            Scenario scenario{"random-line-facts-"+std::to_string(index), "marker", sources[index%sources.size()], 0, 0, "-", {}};
            std::string value = scenario.before;
            for (int step = 0; step < 7; ++step) {
                std::vector<size_t> positions{0};
                for (size_t position = 1; position < value.size(); ++position)
                    if ((static_cast<unsigned char>(value[position]) & 0xc0) != 0x80) positions.push_back(position);
                if (!value.empty()) positions.push_back(value.size());
                size_t first = positions[random()%positions.size()], last = positions[random()%positions.size()];
                if (first > last) std::swap(first, last);
                const std::string inserted = insertions[random()%insertions.size()];
                scenario.edits.push_back({static_cast<int>(first), static_cast<int>(last-first), inserted});
                value.replace(first, last-first, inserted);
            }
            scenarios.push_back(scenario);
        }
        Scenario large{"large-atomic-line-insertions", "marker", "a\nb", 0, 0, "-", {}};
        for (int index = 0; index < 2049; ++index) large.edits.push_back({0, 0, "\n"});
        scenarios.push_back(large);
    }
    for (const auto &scenario : scenarios) {
        const int lines = std::count(scenario.before.begin(), scenario.before.end(), '\n') + 1;
        for (unsigned subset = 0; subset < (1U << lines); ++subset) {
            markerHistory(output, scenario, subset, false);
            markerHistory(output, scenario, subset, true);
        }
    }
    return output ? 0 : 3;
}
