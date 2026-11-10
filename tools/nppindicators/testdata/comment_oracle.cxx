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
    std::string before;
    int selectionStart;
    int selectionEnd;
    std::vector<Mutation> edits;
};

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
        if (marked && start < 0) {
            start = position;
        } else if (!marked && start >= 0) {
            if (!encoded.empty()) encoded += ",";
            encoded += std::to_string(start) + ":" + std::to_string(position);
            start = -1;
        }
    }
    return encoded.empty() ? "-" : encoded;
}

void apply(IDecorationList &decorations, std::string &value, const Mutation &edit) {
    if (edit.at < 0 || edit.removed < 0 || edit.at + edit.removed > static_cast<int>(value.size())) std::abort();
    if (edit.removed > 0) decorations.DeleteRange(edit.at, edit.removed);
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
    auto row = [&](const char *phase) {
        output << scenario.name << '\t' << hexOf(scenario.before) << '\t'
               << scenario.selectionStart << '\t' << scenario.selectionEnd << '\t'
               << initial << '\t' << phase << '\t' << hexOf(value) << '\t'
               << spansOf(*decorations, value.size()) << '\n';
    };
    std::vector<Mutation> inverses;
    for (const Mutation &edit : scenario.edits) {
        inverses.push_back({edit.at, static_cast<int>(edit.inserted.size()), value.substr(edit.at, edit.removed)});
        apply(*decorations, value, edit);
    }
    row("forward");
    for (auto inverse = inverses.rbegin(); inverse != inverses.rend(); ++inverse) apply(*decorations, value, *inverse);
    row("undo");
    for (const Mutation &edit : scenario.edits) apply(*decorations, value, edit);
    row("redo");
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# doBlockComment/doStreamComment/undoStreamComment primitive boundaries; original Decoration forward/undo/redo\n";
    output << "case\tbefore-hex\tselection-start\tselection-end\tinitial\tphase\tvalue-hex\tspans\n";
    const std::vector<Scenario> scenarios{
        {"line-add", "a\nb", 0, 3, {{0, 0, "// "}, {5, 0, "// "}}},
        {"line-toggle", "// a\nb", 0, 6, {{0, 3, ""}, {2, 0, "// "}}},
        {"stream-line-add", "a\nb", 0, 3, {{0, 0, "/* "}, {4, 0, " */"}, {8, 0, "/* "}, {12, 0, " */"}}},
        {"block-add", "x a y", 2, 3, {{2, 0, "/* "}, {6, 0, " */"}}},
        {"empty-block-add", "", 0, 0, {{0, 0, "/* "}, {3, 0, " */"}}},
        {"blank-block-add", "   \n", 0, 0, {{3, 0, "/* "}, {6, 0, " */"}}},
        {"block-remove", "x /* a */ gap /* b */ y", 0, 23, {{6, 3, ""}, {2, 3, ""}, {12, 3, ""}, {8, 3, ""}}},
        {"empty-block-remove", "a/* */b/**/c", 0, 12, {{3, 3, ""}, {1, 2, ""}, {4, 2, ""}, {2, 2, ""}}},
    };
    for (const Scenario &scenario : scenarios) {
        run(output, scenario, {});
        const int length = scenario.before.size();
        for (int start = 0; start < length; ++start) {
            for (int end = start + 1; end <= length; ++end) run(output, scenario, {{start, end}});
            for (int other = start + 2; other < length; ++other) run(output, scenario, {{start, start + 1}, {other, other + 1}});
        }
    }
    return output ? 0 : 3;
}
