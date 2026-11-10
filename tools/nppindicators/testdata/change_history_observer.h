#pragma once

#include <cstddef>
#include <cstdlib>
#include <cstdint>
#include <cassert>
#include <cstring>
#include <stdexcept>
#include <string>
#include <vector>
#include <set>
#include <algorithm>
#include <memory>

#include "ScintillaTypes.h"
#include "Debugging.h"
#include "Position.h"
#include "SplitVector.h"
#include "Partitioning.h"
#include "RunStyles.h"
#include "SparseVector.h"
#include "ChangeHistory.h"

namespace Scintilla::Internal {
void RecordChangeHistory(const std::string &event);

class RecordingChangeHistory final : public ChangeHistory {
public:
    using ChangeHistory::ChangeHistory;

    void Insert(Sci::Position position, Sci::Position length, bool collecting, bool beforeSave) {
        RecordChangeHistory("insert:" + std::to_string(position) + ":" + std::to_string(length) + ":" + std::to_string(collecting) + ":" + std::to_string(beforeSave));
        ChangeHistory::Insert(position, length, collecting, beforeSave);
    }
    void DeleteRangeSavingHistory(Sci::Position position, Sci::Position length, bool beforeSave, bool detached) {
        RecordChangeHistory("delete:" + std::to_string(position) + ":" + std::to_string(length) + ":" + std::to_string(beforeSave) + ":" + std::to_string(detached));
        ChangeHistory::DeleteRangeSavingHistory(position, length, beforeSave, detached);
    }
    void DeleteRange(Sci::Position position, Sci::Position length, bool reverting) {
        RecordChangeHistory("undo-insert:" + std::to_string(position) + ":" + std::to_string(length) + ":" + std::to_string(reverting));
        ChangeHistory::DeleteRange(position, length, reverting);
    }
    void UndoDeleteStep(Sci::Position position, Sci::Position length, bool detached) {
        RecordChangeHistory("undo-delete:" + std::to_string(position) + ":" + std::to_string(length) + ":" + std::to_string(detached));
        ChangeHistory::UndoDeleteStep(position, length, detached);
    }
    void SetSavePoint() {
        RecordChangeHistory("save");
        ChangeHistory::SetSavePoint();
    }
    void StartReversion() {
        RecordChangeHistory("start");
        ChangeHistory::StartReversion();
    }
    void EndReversion() noexcept {
        RecordChangeHistory("end");
        ChangeHistory::EndReversion();
    }
};
}

#ifndef VIVAPAD_UNOBSERVED_ORACLE
#define ChangeHistory RecordingChangeHistory
#endif
