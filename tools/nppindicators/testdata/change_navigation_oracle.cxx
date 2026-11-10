// Compile the pinned original methods, rather than rewriting their decisions.
// This adapter supplies normalized-LF line positions and original query rows.
#include <algorithm>
#include <cstdint>
#include <fstream>
#include <iostream>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

#include "Scintilla.h"
#include "ScintillaTypes.h"

using Scintilla::MarkerOutline;
namespace Sci { using Position = intptr_t; using Line = intptr_t; }

struct OrdinaryMarkers {
    int MarkValue(Sci::Line) const { return 1; } // An unrelated bookmark bit.
};

class Document {
    OrdinaryMarkers markers;
public:
    std::string value;
    std::vector<Sci::Position> starts;
    std::vector<int> editions, runEnds, deletes, nextDeletes;

    void SetText(std::string text) {
        value = std::move(text);
        starts = {0};
        for (size_t at = 0; at < value.size(); ++at)
            if (value[at] == '\n') starts.push_back(at + 1);
    }
    Sci::Line LinesTotal() const { return starts.size(); }
    Sci::Position LineStart(Sci::Line line) const {
        return line < LinesTotal() ? starts.at(line) : value.size();
    }
    Sci::Position LineEnd(Sci::Line line) const {
        return line + 1 < LinesTotal() ? LineStart(line + 1) - 1 : value.size();
    }
    int EditionAt(Sci::Position at) const { return editions.at(at); }
    int EditionEndRun(Sci::Position at) const { return runEnds.at(at); }
    int EditionDeletesAt(Sci::Position at) const { return deletes.at(at); }
    int EditionNextDelete(Sci::Position at) const { return nextDeletes.at(at); }
    const OrdinaryMarkers *Markers() const { return &markers; }
    int GetMark(Sci::Line line, bool includeChangeHistory) const;
};

#include "original_line_mark.inc"

enum class Message { MarkerPrevious };
struct View {
    std::vector<int> markers;
    intptr_t current = 0, goTo = -1, visible = -1;
    intptr_t getCurrentLineNumber() const { return current; }
    Sci::Line LineFromUPtr(intptr_t value) const { return value; }
    int GetMark(Sci::Line line) const {
        return line >= 0 && line < static_cast<Sci::Line>(markers.size()) ? markers[line] : 0;
    }
    intptr_t MarkerPrevious(intptr_t wParam, intptr_t lParam) const {
        switch (Message::MarkerPrevious) {
#include "original_marker_previous.inc"
        }
        return -1;
    }
    intptr_t execute(int message, intptr_t wParam = 0, intptr_t lParam = 0) {
        switch (message) {
        case SCI_GETLINECOUNT: return markers.size();
        case SCI_MARKERGET: return GetMark(wParam);
        case SCI_MARKERPREVIOUS: return MarkerPrevious(wParam, lParam);
        case SCI_ENSUREVISIBLEENFORCEPOLICY: visible = wParam; return 0;
        case SCI_GOTOLINE: goTo = current = wParam; return 0;
        default: throw std::runtime_error("unexpected original editor message");
        }
    }
};

constexpr int IDM_SEARCH_CHANGED_NEXT = 1;
constexpr int MB_ICONEXCLAMATION = 0;
void MessageBeep(int) { throw std::runtime_error("unexpected audible oracle"); }
struct NppParameters {
    struct GUI { bool _muteSounds = true; } gui;
    static NppParameters &getInstance() { static NppParameters parameters; return parameters; }
    const GUI &getNppGUI() const { return gui; }
};
class Notepad_plus {
public:
    View *_pEditView;
    void changedHistoryGoTo(int idGoTo);
};

#include "original_change_navigation.inc"

std::vector<std::string> Split(const std::string &value, char separator) {
    std::vector<std::string> result;
    std::istringstream input(value);
    for (std::string part; std::getline(input, part, separator);) result.push_back(part);
    return result;
}

std::vector<int> Integers(const std::string &value) {
    std::vector<int> result;
    for (const auto &part : Split(value, ',')) result.push_back(std::stoi(part));
    return result;
}

std::string Unhex(const std::string &value) {
    if (value == "-") return {};
    if (value.size() % 2) throw std::runtime_error("odd hex input");
    std::string result;
    for (size_t at = 0; at < value.size(); at += 2)
        result.push_back(std::stoi(value.substr(at, 2), nullptr, 16));
    return result;
}

void LineFixture(const char *inputPath, std::ostream &output) {
    std::ifstream input(inputPath);
    if (!input) throw std::runtime_error("cannot open original metadata fixture");
    output << "# Pinned original Document::GetMark, normalized-LF adapter; not native Document acceptance\n";
    bool header = false;
    for (std::string row; std::getline(input, row);) {
        if (row.empty()) continue;
        if (row[0] == '#') { output << "# Input: " << row.substr(1) << '\n'; continue; }
        if (!header) { header = true; output << "case\tstep\tline-masks\n"; continue; }
        const auto fields = Split(row, '\t');
        if (fields.size() != 8) throw std::runtime_error("incomplete metadata row");
        Document document;
        document.SetText(Unhex(fields[2]));
        document.editions = Integers(fields[4]);
        document.runEnds = Integers(fields[5]);
        document.deletes = Integers(fields[6]);
        document.nextDeletes = Integers(fields[7]);
        output << fields[0] << '\t' << fields[1] << '\t';
        for (Sci::Line line = 0; line < document.LinesTotal(); ++line) {
            if (line) output << ',';
            output << ((document.GetMark(line, true) >> SC_MARKNUM_HISTORY_REVERTED_TO_ORIGIN) & 15);
        }
        output << '\n';
    }
    if (!input.eof()) throw std::runtime_error("failed to read metadata fixture");
}

void NavigationFixture(std::ostream &output) {
    output << "# Pinned original changedHistoryGoTo and Editor::MarkerPrevious case, synthetic marker interface\n";
    output << "line-count\tchanged-bits\tcurrent\tprevious\ttarget\n";
    for (int count = 1; count <= 9; ++count) {
        for (unsigned bits = 0; bits < (1u << count); ++bits) {
            for (int current = 0; current < count; ++current) {
                for (int previous = 0; previous <= 1; ++previous) {
                    View view;
                    view.current = current;
                    for (int line = 0; line < count; ++line)
                        view.markers.push_back(1 | ((bits & (1u << line)) ? (1 << (SC_MARKNUM_HISTORY_REVERTED_TO_ORIGIN + line % 4)) : 0));
                    Notepad_plus application{&view};
                    application.changedHistoryGoTo(previous ? 0 : IDM_SEARCH_CHANGED_NEXT);
                    if (view.goTo != view.visible || (view.goTo == -1 && view.current != current))
                        throw std::runtime_error("unexpected navigation side effects");
                    output << count << '\t' << bits << '\t' << current << '\t' << previous << '\t' << view.goTo << '\n';
                }
            }
        }
    }
}

int main(int count, char **arguments) {
    try {
        if (count < 3) return 1;
        const std::string mode = arguments[1];
        if (mode != "navigation" && mode != "lines") return 1;
        if (count != (mode == "lines" ? 4 : 3)) return 1;
        std::ofstream output(arguments[count - 1]);
        if (!output) return 2;
        if (mode == "lines") LineFixture(arguments[2], output);
        else NavigationFixture(output);
        return output ? 0 : 3;
    } catch (const std::exception &error) {
        std::cerr << error.what() << '\n';
        return 4;
    }
}
