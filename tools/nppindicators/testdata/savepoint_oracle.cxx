#include <algorithm>
#include <array>
#include <cassert>
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
#include "SparseVector.h"
#include "ChangeHistory.h"
#include "CellBuffer.h"
#include "UndoHistory.h"

using namespace Scintilla::Internal;

struct Operation {
    std::string name;
    int at = 0;
    int removed = 0;
    std::string inserted;
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

void replace(UndoHistory &history, std::string &value, int at, int removed,
             const std::string &inserted, bool coalesce) {
    if (at < 0 || removed < 0 || at + removed > static_cast<int>(value.size())) std::abort();
    bool start = false;
    if (removed > 0) {
        const std::string deleted = value.substr(at, removed);
        history.AppendAction(ActionType::remove, at, deleted.data(), deleted.size(), start, coalesce);
        value.erase(at, removed);
    }
    if (!inserted.empty()) {
        history.AppendAction(ActionType::insert, at, inserted.data(), inserted.size(), start, coalesce);
        value.insert(at, inserted);
    }
}

void undo(UndoHistory &history, std::string &value) {
    const int count = history.StartUndo();
    for (int step = 0; step < count; ++step) {
        const Action action = history.GetUndoStep();
        if (action.at == ActionType::insert) value.erase(action.position, action.lenData);
        else if (action.at == ActionType::remove) value.insert(action.position, action.data, action.lenData);
        history.CompletedUndoStep();
    }
}

void redo(UndoHistory &history, std::string &value) {
    const int count = history.StartRedo();
    for (int step = 0; step < count; ++step) {
        const Action action = history.GetRedoStep();
        if (action.at == ActionType::insert) value.insert(action.position, action.data, action.lenData);
        else if (action.at == ActionType::remove) value.erase(action.position, action.lenData);
        history.CompletedRedoStep();
    }
}

void scenario(std::ofstream &output, const std::string &name, const std::string &initial,
              const std::vector<Operation> &operations) {
    UndoHistory history;
    std::string value = initial;
    int step = 0;
    auto row = [&](const Operation &operation) {
        output << name << '\t' << step++ << '\t' << operation.name << '\t'
               << operation.at << '\t' << operation.removed << '\t' << hexOf(operation.inserted) << '\t'
               << hexOf(value) << '\t' << history.IsSavePoint() << '\t'
               << history.CanUndo() << '\t' << history.CanRedo() << '\n';
    };
    row({"start", 0, 0, initial});
    for (const Operation &operation : operations) {
        if (operation.name == "save") history.SetSavePoint();
        else if (operation.name == "clear") history.DeleteUndoHistory();
        else if (operation.name == "undo") undo(history, value);
        else if (operation.name == "redo") redo(history, value);
        else if (operation.name == "type") replace(history, value, operation.at, 0, operation.inserted, true);
        else if (operation.name == "backspace") replace(history, value, operation.at, operation.removed, "", true);
        else if (operation.name == "replace") {
            history.BeginUndoAction();
            replace(history, value, operation.at, operation.removed, operation.inserted, false);
            history.EndUndoAction();
        } else if (operation.name == "zero-transaction") {
            history.BeginUndoAction();
            replace(history, value, 0, 0, "x", false);
            replace(history, value, 0, 1, "", false);
            history.EndUndoAction();
        } else if (operation.name == "equal-batch") {
            history.BeginUndoAction();
            replace(history, value, 0, 1, "a", false);
            replace(history, value, 2, 1, "a", false);
            history.EndUndoAction();
        } else std::abort();
        row(operation);
    }
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Original UndoHistory committed-edit/save-point states, not native caret or notification parity\n";
    output << "case\tstep\top\tat\tremoved\tinserted-hex\tvalue-hex\tat-save\tcan-undo\tcan-redo\n";
    scenario(output, "net-zero", "seed", {{"type",4,0,"X"},{"backspace",4,1,""},{"undo"},{"undo"},{"redo"},{"redo"}});
    scenario(output, "save-boundary", "", {{"type",0,0,"a"},{"type",1,0,"b"},{"save"},{"type",2,0,"c"},{"type",3,0,"d"},{"undo"},{"undo"},{"redo"},{"redo"}});
    scenario(output, "branch", "seed", {{"replace",0,4,"first"},{"save"},{"replace",0,5,"second"},{"undo"},{"undo"},{"replace",0,4,"branch"},{"undo"},{"redo"},{"save"}});
    scenario(output, "equal-replace", "cat", {{"replace",0,3,"cat"},{"undo"},{"redo"},{"save"},{"undo"},{"redo"}});
    scenario(output, "equal-cuts-redo", "aaa", {{"replace",3,0,"!"},{"undo"},{"replace",1,1,"a"},{"redo"},{"undo"}});
    scenario(output, "clear", "base", {{"replace",4,0,"x"},{"clear"},{"undo"},{"save"},{"replace",0,1,"B"},{"undo"}});
    scenario(output, "unicode", "\xe7\x8c\xab", {{"type",3,0,"\xc3\xa9"},{"save"},{"replace",0,3,"\xe7\x8a\xac"},{"undo"},{"redo"},{"save"}});
    scenario(output, "zero-group", "", {{"zero-transaction"},{"undo"},{"redo"},{"save"},{"undo"},{"redo"}});
    scenario(output, "equal-targets", "a a", {{"equal-batch"},{"undo"},{"redo"},{"save"},{"undo"},{"redo"}});
    scenario(output, "empty-edit", "a", {{"replace",1,0,""},{"save"},{"undo"},{"redo"}});
    return output ? 0 : 3;
}
