#include "change_history_observer.h"

#include <cstdarg>
#include <cstdio>
#include <fstream>
#include <functional>
#include <optional>
#include <sstream>
#include <string_view>

#include "CellBuffer.h"

using namespace Scintilla::Internal;

std::vector<std::string> recordedChanges;
std::string activeChangeWorkflow;
int activeChangeStep = 0;

namespace Scintilla::Internal {
void RecordChangeHistory(const std::string &event) {
    if (std::getenv("VIVAPAD_CHANGE_HISTORY_TRACE")) {
        std::fprintf(stderr, "%s/%d %s\n", activeChangeWorkflow.c_str(), activeChangeStep, event.c_str());
    }
    recordedChanges.push_back(event);
}
namespace Platform {
void Assert(const char *condition, const char *file, int line) noexcept {
    std::fprintf(stderr, "%s:%d: %s\n", file, line, condition);
    std::abort();
}
void DebugPrintf(const char *format, ...) noexcept {
    va_list arguments;
    va_start(arguments, format);
    std::vfprintf(stderr, format, arguments);
    va_end(arguments);
}
}
}

std::string hexChangeText(const std::string &value) {
    if (value.empty()) return "-";
    constexpr char digits[] = "0123456789abcdef";
    std::string encoded;
    for (const unsigned char character : value) {
        encoded.push_back(digits[character >> 4]);
        encoded.push_back(digits[character & 15]);
    }
    return encoded;
}

class Workflow {
    std::ofstream &output;
    std::string name;
    CellBuffer buffer{false, false};
    int step = 0;
public:
    Workflow(std::ofstream &output_, std::string name_, const std::string &initial) : output(output_), name(std::move(name_)) {
        activeChangeWorkflow = name;
        activeChangeStep = 0;
        bool sequence = false;
        buffer.SetUndoCollection(false);
        buffer.InsertString(0, initial.data(), initial.size(), sequence);
        buffer.SetUndoCollection(true);
        buffer.DeleteUndoHistory();
        buffer.SetSavePoint();
        buffer.ChangeHistorySet(true);
        recordedChanges.clear();
        recordedChanges.push_back("reset:" + std::to_string(buffer.Length()));
        Row("initial");
    }
    Sci::Position Length() const { return buffer.Length(); }
    void Insert(Sci::Position at, const std::string &value) {
        bool sequence = false;
        buffer.InsertString(at, value.data(), value.size(), sequence);
    }
    void Delete(Sci::Position at, Sci::Position length) {
        bool sequence = false;
        buffer.DeleteChars(at, length, sequence);
    }
    void Save() { buffer.SetSavePoint(); }
    void Collect(bool value) { buffer.SetUndoCollection(value); }
    void Begin() { buffer.BeginUndoAction(); }
    void End() { buffer.EndUndoAction(); }
    void Undo() {
        if (buffer.CanUndo()) {
            const int count = buffer.StartUndo();
            for (int index = 0; index < count; ++index) buffer.PerformUndoStep();
        }
    }
    void Redo() {
        if (buffer.CanRedo()) {
            const int count = buffer.StartRedo();
            for (int index = 0; index < count; ++index) buffer.PerformRedoStep();
        }
    }
    void Clear() {
        buffer.DeleteUndoHistory();
        buffer.ChangeHistorySet(false);
        buffer.ChangeHistorySet(true);
        recordedChanges.push_back("reset:" + std::to_string(buffer.Length()));
    }
    void Row(const std::string &phase) {
        const std::string value(buffer.BufferPointer(), buffer.Length());
        output << name << '\t' << step++ << '/' << phase << '\t' << hexChangeText(value) << '\t';
        if (recordedChanges.empty()) output << '-';
        for (size_t index = 0; index < recordedChanges.size(); ++index) {
            if (index) output << ';';
            output << recordedChanges[index];
        }
        for (int query = 0; query < 4; ++query) {
            output << '\t';
            for (Sci::Position position = 0; position <= buffer.Length(); ++position) {
                if (position) output << ',';
                switch (query) {
                    case 0: output << buffer.EditionAt(position); break;
                    case 1: output << buffer.EditionEndRun(position); break;
                    case 2: output << buffer.EditionDeletesAt(position); break;
                    case 3: output << buffer.EditionNextDelete(position); break;
                }
            }
        }
        output << '\n';
        output.flush();
        recordedChanges.clear();
        activeChangeStep = step;
    }
};

int main(int argumentCount, char **arguments) {
    if (argumentCount < 2 || argumentCount > 3) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
#ifdef VIVAPAD_UNOBSERVED_ORACLE
    output << "# Actual Notepad++ dd40fe4 CellBuffer/UndoHistory/ChangeHistory without an observing subclass\n";
#else
    output << "# Actual Notepad++ dd40fe4 CellBuffer/UndoHistory/ChangeHistory with an observing subclass, not a rewritten history algorithm\n";
#endif
    if (argumentCount == 3) {
        if (std::string_view(arguments[2]) != "--span-boundary-fixed") return 1;
        output << "# This reference includes the separately documented one-line remaining-span correction; it is NOT unmodified-original acceptance\n";
    }
    output << "case\tstep\ttext-hex\tevents\teditions\trun-ends\tdeletion-flags\tnext-deletions\n";
    {
        Workflow work(output, "saved-reversion", "abc");
        work.Insert(1, "XY"); work.Row("insert");
        work.Save(); work.Row("save");
        work.Delete(1, 2); work.Row("delete");
        work.Undo(); work.Row("undo-delete");
        work.Undo(); work.Row("undo-before-save");
        work.Redo(); work.Row("redo-to-save");
        work.Redo(); work.Row("redo-after-save");
    }
    {
        Workflow work(output, "detached", "abcd");
        work.Insert(1, "x"); work.Row("insert");
        work.Insert(2, "y"); work.Row("coalesce");
        work.Save(); work.Row("save");
        work.Undo(); work.Row("undo");
        work.Insert(0, "z"); work.Row("branch");
        work.Delete(2, 1); work.Row("detached-delete");
        work.Undo(); work.Row("undo-detached-delete");
        work.Undo(); work.Row("undo-detached-insert");
        work.Redo(); work.Row("redo-branch");
        work.Save(); work.Row("new-save");
    }
    {
        Workflow work(output, "deletion-boundaries", "abcdef");
        work.Delete(1, 2); work.Row("delete");
        work.Delete(1, 1); work.Row("adjacent-delete");
        work.Save(); work.Row("save");
        work.Insert(1, "PQ"); work.Row("insert-at-deletion");
        work.Delete(0, work.Length()); work.Row("delete-all");
        work.Undo(); work.Row("undo-delete-all");
        work.Undo(); work.Row("undo-insert");
        work.Undo(); work.Row("undo-old-delete");
        work.Redo(); work.Row("redo-old-delete");
    }
    {
        Workflow work(output, "grouped", "a\nb");
        work.Begin();
        work.Delete(1, 1);
        work.Insert(0, "\n");
        work.End(); work.Row("group");
        work.Save(); work.Row("save");
        work.Undo(); work.Row("undo-group");
        work.Redo(); work.Row("redo-group");
        work.Clear(); work.Row("clear");
        work.Insert(work.Length(), "\n"); work.Row("post-clear");
    }
    {
        Workflow work(output, "without-undo", "abc");
        work.Collect(false);
        work.Insert(1, "x"); work.Row("uncollected-insert");
        work.Delete(2, 1); work.Row("uncollected-delete");
        work.Save(); work.Row("save");
        work.Clear(); work.Row("clear");
        work.Collect(true);
        work.Insert(0, "q"); work.Row("collected-insert");
        work.Undo(); work.Row("undo");
    }
    for (unsigned seed = 1; seed <= 64; ++seed) {
        Workflow work(output, "script-" + std::to_string(seed), "a\nbc");
        uint32_t randomState = seed * 2654435761u;
        auto next = [&]() { randomState ^= randomState << 13; randomState ^= randomState >> 17; randomState ^= randomState << 5; return randomState; };
        for (int index = 0; index < 128; ++index) {
            const unsigned operation = next() % 9;
            if (operation < 3 && work.Length() < 64) {
                const std::string value = operation == 0 ? "x" : operation == 1 ? "\n" : std::string("\0z", 2);
                work.Insert(next() % (work.Length() + 1), value);
            } else if (operation == 3 && work.Length()) {
                const Sci::Position at = next() % work.Length();
                work.Delete(at, 1 + next() % std::min<Sci::Position>(4, work.Length() - at));
            } else if (operation == 4) {
                work.Save();
            } else if (operation == 5) {
                work.Undo();
            } else if (operation == 6) {
                work.Redo();
            } else if (operation == 7) {
                work.Begin();
                work.Insert(0, "A");
                if (work.Length() > 2) work.Delete(work.Length() - 2, 1);
                work.End();
            } else {
                work.Clear();
            }
            work.Row("step-" + std::to_string(operation));
        }
    }
    return output ? 0 : 3;
}
