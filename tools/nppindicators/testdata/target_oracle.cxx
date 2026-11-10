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

void runSequence(std::ofstream &output, const std::string &name, int length,
                 const std::vector<std::array<int, 2>> &spans,
                 const std::vector<std::array<int, 3>> &edits) {
    auto decorations = DecorationListCreate(false);
    decorations->InsertSpace(0, length);
    decorations->SetCurrentIndicator(25);
    for (const auto &span : spans) decorations->FillRange(span[0], 1, span[1] - span[0]);
    for (const auto &edit : edits) {
        const int beforeLength = length;
        const std::string before = spansOf(*decorations, length);
        if (edit[1] > 0) decorations->DeleteRange(edit[0], edit[1]);
        if (edit[2] > 0) decorations->InsertSpace(edit[0], edit[2]);
        length += edit[2] - edit[1];
        output << name << "\t" << beforeLength << "\t" << before << "\t"
               << edit[0] << "\t" << edit[1] << "\t" << edit[2] << "\t"
               << spansOf(*decorations, length) << "\n";
    }
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Scintilla DecorationList target sequences, forward/undo/redo\n";
    output << "case\tlength\tbefore\tat\tremoved\tinserted\tafter\n";
    runSequence(output, "column-eol", 6, {{2, 3}, {5, 6}}, {
        {0, 2, 3}, {4, 2, 3}, {8, 0, 1},
        {8, 1, 0}, {4, 3, 2}, {0, 3, 2},
        {0, 2, 3}, {4, 2, 3}, {8, 0, 1},
    });
    runSequence(output, "adjacent-replace-all", 3, {{0, 1}, {2, 3}}, {
        {1, 1, 1}, {2, 1, 1}, {2, 1, 1}, {1, 1, 1}, {1, 1, 1}, {2, 1, 1},
    });
    return output ? 0 : 3;
}
