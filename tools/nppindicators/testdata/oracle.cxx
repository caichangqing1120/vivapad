#include <algorithm>
#include <array>
#include <cstddef>
#include <cstdint>
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

using Span = std::array<int, 2>;

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

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Scintilla DecorationList: DeleteRange followed by InsertSpace\n";
    output << "length\tbefore\tat\tremoved\tinserted\tafter\n";
    const std::vector<std::vector<Span>> configurations = {
        {}, {{0, 8}}, {{0, 3}}, {{2, 5}}, {{5, 8}}, {{1, 3}, {5, 7}},
    };
    for (const auto &initial : configurations) {
        for (int position = 0; position <= 8; ++position) {
            for (int removed = 0; removed <= 8 - position; ++removed) {
                for (int inserted = 0; inserted <= 3; ++inserted) {
                    auto decorations = DecorationListCreate(false);
                    decorations->InsertSpace(0, 8);
                    decorations->SetCurrentIndicator(25);
                    for (const Span &span : initial) {
                        decorations->FillRange(span[0], 1, span[1] - span[0]);
                    }
                    const std::string before = spansOf(*decorations, 8);
                    if (removed > 0) decorations->DeleteRange(position, removed);
                    if (inserted > 0) decorations->InsertSpace(position, inserted);
                    output << "8\t" << before << "\t" << position << "\t" << removed
                           << "\t" << inserted << "\t" << spansOf(*decorations, 8 - removed + inserted) << "\n";
                }
            }
        }
    }
    return output ? 0 : 3;
}
