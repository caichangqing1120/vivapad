#include <algorithm>
#include <array>
#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <fstream>
#include <memory>
#include <optional>
#include <stdexcept>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

#include "ScintillaTypes.h"
#include "Debugging.h"
#include "Position.h"
#include "SplitVector.h"
#include "Partitioning.h"
#include "RunStyles.h"
#include "Decoration.h"

using namespace Scintilla::Internal;

struct Mutation {
    int at;
    int removed;
    std::string inserted;
};

struct Scenario {
    std::string name;
    int mode;
    std::string before;
    int selectionStart;
    int selectionEnd;
    std::vector<Mutation> edits;
};

std::pair<std::string, int> convertLine(const std::string &line, int width, int mode, int caret) {
    std::string output;
    int column = 0;
    int position = 0;
    if (mode == 6) {
        if (line.find('\t') == std::string::npos) return {line, 0};
        for (int index = 0; index < static_cast<int>(line.size()); ++index) {
            if (line[index] == '\t') {
                const int inserted = width - column % width;
                output.append(inserted, ' ');
                column += inserted;
                if (index < caret) position += inserted;
            } else {
                output += line[index];
                if ((line[index] & 0xc0) != 0x80) ++column;
                if (index < caret) ++position;
            }
        }
        return {output, position};
    }
    int tabStop = width - 1;
    int counter = 0;
    bool nonSpaceFound = false;
    const auto characterAt = [&](int index) { return index < static_cast<int>(line.size()) ? line[index] : '\0'; };
    for (int index = 0; index < static_cast<int>(line.size()); ++index) {
        bool nextCharacter = false;
        if (!nonSpaceFound) {
            while (characterAt(index + counter) == ' ') {
                if (column + counter == tabStop) {
                    tabStop += width;
                    if (counter >= 1) {
                        output += '\t';
                        index += counter;
                        column += counter + 1;
                        counter = 0;
                        nextCharacter = true;
                        if (index <= caret) ++position;
                        break;
                    } else if (characterAt(index + 1) == ' ' || characterAt(index + 1) == '\t') {
                        output += '\t';
                        ++index;
                        ++column;
                        counter = 0;
                        if (index <= caret) ++position;
                    } else {
                        output += line[index];
                        ++column;
                        counter = 0;
                        nextCharacter = true;
                        if (index <= caret) ++position;
                        break;
                    }
                } else {
                    ++counter;
                }
            }
            if (nextCharacter) continue;
            if (characterAt(index) == ' ' && characterAt(index + counter) == '\t') {
                output += '\t';
                index += counter;
                column = tabStop + 1;
                tabStop += width;
                counter = 0;
                if (index <= caret) ++position;
                continue;
            }
        }
        if (mode == 8 && !nonSpaceFound) nonSpaceFound = true;
        const char character = line[index];
        output += character;
        if (character == '\n' || character == '\r') {
            column = 0;
            tabStop = width - 1;
            nonSpaceFound = false;
        } else if (character == '\t') {
            column = tabStop + 1;
            tabStop += width;
            counter = 0;
        } else {
            counter = 0;
            if ((character & 0xc0) != 0x80) {
                ++column;
                if (column > 0 && column % width == 0) tabStop += width;
            }
        }
        if (index < caret) ++position;
    }
    return {output, position};
}

std::string hexOf(const std::string &value) {
    const char digits[] = "0123456789abcdef";
    std::string encoded;
    for (unsigned char character : value) {
        encoded += digits[character >> 4];
        encoded += digits[character & 15];
    }
    return encoded.empty() ? "-" : encoded;
}

std::string spansOf(IDecorationList &decorations, int length) {
    std::string encoded;
    int start = -1;
    for (int position = 0; position <= length; ++position) {
        const bool marked = position < length && decorations.ValueAt(25, position) != 0;
        if (marked && start < 0) start = position;
        else if (!marked && start >= 0) {
            if (!encoded.empty()) encoded += ",";
            encoded += std::to_string(start) + ":" + std::to_string(position);
            start = -1;
        }
    }
    return encoded.empty() ? "-" : encoded;
}

void apply(IDecorationList &decorations, std::string &value, const Mutation &edit) {
    if (edit.at < 0 || edit.removed < 0 || edit.at + edit.removed > static_cast<int>(value.size())) std::abort();
    if (edit.removed) decorations.DeleteRange(edit.at, edit.removed);
    if (!edit.inserted.empty()) decorations.InsertSpace(edit.at, edit.inserted.size());
    value.replace(edit.at, edit.removed, edit.inserted);
}

void run(std::ofstream &output, const Scenario &scenario, const std::vector<std::array<int, 2>> &spans) {
    auto decorations = DecorationListCreate(false);
    std::string value = scenario.before;
    decorations->InsertSpace(0, value.size());
    decorations->SetCurrentIndicator(25);
    for (const auto &span : spans) decorations->FillRange(span[0], 1, span[1] - span[0]);
    const std::string initial = spansOf(*decorations, value.size());
    const auto row = [&](const char *phase) {
        output << scenario.name << '\t' << scenario.mode << '\t' << hexOf(scenario.before) << '\t'
               << scenario.selectionStart << '\t' << scenario.selectionEnd << '\t'
               << initial << '\t' << phase << '\t' << hexOf(value) << '\t'
               << spansOf(*decorations, value.size()) << '\n';
    };
    std::vector<Mutation> inverses;
    for (const auto &edit : scenario.edits) {
        inverses.push_back({edit.at, static_cast<int>(edit.inserted.size()), value.substr(edit.at, edit.removed)});
        apply(*decorations, value, edit);
    }
    row("forward");
    for (auto inverse = inverses.rbegin(); inverse != inverses.rend(); ++inverse) apply(*decorations, value, *inverse);
    row("undo");
    for (const auto &edit : scenario.edits) apply(*decorations, value, edit);
    row("redo");
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 3) return 1;
    std::ofstream output(arguments[1]);
    std::ofstream tabs(arguments[2]);
    if (!output || !tabs) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Source-transcribed doTrim/wsTabConvert/LinesJoin targets; original Decoration forward/undo/redo\n";
    output << "case\tmode\tbefore-hex\tselection-start\tselection-end\tinitial\tphase\tvalue-hex\tspans\n";
    const std::vector<Scenario> scenarios{
        {"leading", 2, "  a\n b", 0, 6, {{0, 2, ""}, {2, 1, ""}}},
        {"trailing", 1, "a \nb\t", 0, 5, {{1, 1, ""}, {3, 1, ""}}},
        {"both", 3, " a \n b ", 0, 7, {{0, 1, ""}, {1, 1, ""}, {2, 1, ""}, {3, 1, ""}}},
        {"blank-line", 3, " \t\n a ", 0, 6, {{0, 2, ""}, {1, 1, ""}, {2, 1, ""}}},
        {"join", 4, "a \nb\n\nc", 0, 7, {{2, 1, ""}, {3, 1, " "}, {4, 1, ""}}},
        {"join-selection", 4, "x a\nb y\nz", 2, 5, {{3, 1, " "}}},
        {"trim-join", 5, " a \n b \n", 0, 8, {{0, 1, ""}, {1, 1, ""}, {2, 1, ""}, {3, 1, ""}, {1, 1, " "}, {3, 1, " "}}},
        {"tabs", 6, "a\tb\nkeep\n\tc", 0, 11, {{0, 3, convertLine("a\tb", 4, 6, -1).first}, {11, 2, convertLine("\tc", 4, 6, -1).first}}},
        {"spaces-all", 7, "a   b\nkeep\n    c", 0, 16, {{0, 5, convertLine("a   b", 4, 7, -1).first}, {9, 5, convertLine("    c", 4, 7, -1).first}}},
        {"spaces-leading", 8, "    a  b\nkeep\n    c", 0, 19, {{0, 8, convertLine("    a  b", 4, 8, -1).first}, {11, 5, convertLine("    c", 4, 8, -1).first}}},
        {"space-before-tab", 7, "abc \tX\nkeep", 0, 11, {{0, 6, convertLine("abc \tX", 4, 7, -1).first}}},
        {"empty-trim-selection", 5, " \t\nkeep", 0, 2, {{0, 2, ""}}},
        {"tab-selection-end", 6, "a\tb\n\tc", 1, 4, {{0, 3, convertLine("a\tb", 4, 6, -1).first}}},
    };
    for (const auto &scenario : scenarios) {
        run(output, scenario, {});
        const int length = scenario.before.size();
        for (int start = 0; start < length; ++start) {
            for (int end = start + 1; end <= length; ++end) run(output, scenario, {{start, end}});
            for (int other = start + 2; other < length; ++other) run(output, scenario, {{start, start + 1}, {other, other + 1}});
        }
    }
    run(output, {"trim-join-words", 5, "  keep \t\n\tkeep  \n", 0, 17,
        {{0, 2, ""}, {4, 2, ""}, {5, 1, ""}, {9, 2, ""}, {4, 1, " "}, {9, 1, " "}}},
        {{2, 6}, {10, 14}});
    tabs << "# Source-transcribed wsTabConvert UTF-8 loop and per-line byte caret calculation\n";
    tabs << "mode\tbefore-hex\twidth\tcaret-byte\tafter-hex\tmapped-caret-byte\n";
    for (const auto &prefix : {std::string(""), std::string("a"), std::string("abc"), std::string("\xce\xb1")}) {
        for (const auto &whitespace : {" ", "  ", "   ", "    ", "\t", " \t", "\t ", " \t  ", "\t    "}) {
            for (const auto &suffix : {"", "X", " \tX"}) {
                const std::string value = prefix + whitespace + suffix;
                for (const int width : {1, 2, 4, 8}) {
                    for (const int mode : {6, 7, 8}) {
                        for (int caret = 0; caret <= static_cast<int>(value.size()); ++caret) {
                            if (caret < static_cast<int>(value.size()) && (value[caret] & 0xc0) == 0x80) continue;
                            const auto converted = convertLine(value, width, mode, caret);
                            tabs << mode << '\t' << hexOf(value) << '\t' << width << '\t' << caret << '\t'
                                 << hexOf(converted.first) << '\t' << converted.second << '\n';
                        }
                    }
                }
            }
        }
    }
    return output && tabs ? 0 : 3;
}
