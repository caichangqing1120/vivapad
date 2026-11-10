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

#include "Debugging.h"
#include "Position.h"
#include "Selection.h"

using namespace Scintilla::Internal;

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Scintilla SelectionRange: delete followed by insert, no virtual space\n";
    output << "anchor\tcaret\tat\tremoved\tinserted\tafterAnchor\tafterCaret\n";
    const std::vector<std::array<int, 2>> configurations = {
        {0, 0}, {4, 4}, {8, 8}, {1, 6}, {6, 1}, {0, 8}, {8, 0},
    };
    for (const auto &initial : configurations) {
        for (int position = 0; position <= 8; ++position) {
            for (int removed = 0; removed <= 8 - position; ++removed) {
                for (int inserted = 0; inserted <= 3; ++inserted) {
                    SelectionRange range(initial[1], initial[0]);
                    if (removed > 0) range.MoveForInsertDelete(false, position, removed);
                    if (inserted > 0) range.MoveForInsertDelete(true, position, inserted);
                    output << initial[0] << "\t" << initial[1] << "\t" << position << "\t" << removed
                           << "\t" << inserted << "\t" << range.anchor.Position()
                           << "\t" << range.caret.Position() << "\n";
                }
            }
        }
    }
    return output ? 0 : 3;
}
