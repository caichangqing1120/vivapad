#define main wholeLineOracleMain
#include "line_oracle.cxx"
#undef main

#include <cwctype>
#include "Selection.h"

_locale_t _create_locale(int, const char *) { return nullptr; }
void _free_locale(_locale_t) {}
int _wcsicmp(const wchar_t *left, const wchar_t *right) {
    while (*left && *right) {
        const auto first = std::towlower(*left++);
        const auto second = std::towlower(*right++);
        if (first != second) return first < second ? -1 : 1;
    }
    return (*left != 0) - (*right != 0);
}
double stodLocale(const std::wstring &value, _locale_t) { return std::stod(value); }
std::wstring stringTakeWhileAdmissable(const std::wstring &value, const std::wstring &allowed) {
    return value.substr(0, value.find_first_not_of(allowed));
}
std::wstring stringReplace(const std::wstring &value, const std::wstring &from, const std::wstring &to) {
    std::wstring result = value;
    for (size_t position = result.find(from); position != std::wstring::npos; position = result.find(from, position + to.size()))
        result.replace(position, from.size(), to);
    return result;
}

std::vector<size_t> boundaries(const std::string &line) {
    std::vector<size_t> offsets{0};
    for (size_t position = 1; position < line.size(); ++position)
        if ((static_cast<unsigned char>(line[position]) & 0xc0) != 0x80) offsets.push_back(position);
    offsets.push_back(line.size());
    std::vector<size_t> chosen{offsets[0], offsets[std::min<size_t>(1, offsets.size()-1)], offsets[offsets.size()/2], offsets.back()};
    std::sort(chosen.begin(), chosen.end());
    chosen.erase(std::unique(chosen.begin(), chosen.end()), chosen.end());
    return chosen;
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2 || wideOf("\xf0\x90\x80\x80").size() != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Original Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a Sorters.h and SelectionRange\n";
    output << "# Normalized LF; source-transcribed byte column extraction; host C-locale numeric/case shims are not Windows CRT acceptance\n";
    output << "before-hex\tanchor-byte\tcaret-byte\tanchor-virtual\tcaret-virtual\tmode\tascending\tafter-hex\tchanged\tfailed-line\tafter-caret-byte\tafter-rectangle-anchor\tafter-rectangle-caret\tafter-virtual\n";
    struct Document { std::string value; bool outside; bool singleRow = false; };
    const std::vector<Document> documents{
        {"outside\nz 22 keep\na 1 keep\nm 1 keep\ntail", true},
        {"z\t2\na\t1\nm\t3", false},
        {"z\xe7\x8c\xab" "a9Q\nm\xe7\x8c\xabz1B\nx\xf0\x90\x80\x80x2", false},
        {"z\xee\x80\x80\nx\xf0\x90\x80\x80\ny\xe7\x8c\xab", false},
        {"a2\nz10\nb02\n", false},
        {"z 2.5\na -1.5\nm 2.5", false},
        {"z 2,5\na -1,5\nm 2,5", false},
        {"z --\na 1\nm .", false},
        {"z\n\nm\n", false},
        {std::string("z A\0z\na a\0b\nm B", 15), false},
        {"z\nb\na", false, true},
        {"--", false, true},
    };
    for (const auto &document : documents) {
        const auto &before = document.value;
        const bool outside = document.outside;
        std::vector<size_t> starts{0};
        for (size_t position = 0; position < before.size(); ++position)
            if (before[position] == '\n') starts.push_back(position+1);
        const size_t first = outside ? 1 : 0;
        const size_t last = document.singleRow ? first : starts.size() - (outside ? 2 : 1);
        const size_t start = starts[first];
        const auto lineAt = [&](size_t line) {
            const auto endLine = before.find('\n', starts[line]);
            return before.substr(starts[line], endLine == std::string::npos ? endLine : endLine - starts[line]);
        };
        for (const auto firstOffset : boundaries(lineAt(first)))
        for (const auto lastOffset : boundaries(lineAt(last)))
        for (const int virtualSpace : {0, 3})
        for (const bool reversed : {false, true})
        for (int mode = 1; mode <= 7; ++mode)
        for (const bool ascending : {false, true}) {
            size_t anchor = start + firstOffset, caret = starts[last] + lastOffset;
            int anchorVirtual = virtualSpace, caretVirtual = 0;
            if (reversed) { std::swap(anchor, caret); std::swap(anchorVirtual, caretVirtual); }
            const bool wholeDocument = document.singleRow && anchor == caret;
            const size_t firstLine = wholeDocument ? 0 : first;
            const size_t lastLine = wholeDocument ? starts.size()-1 : last;
            const size_t targetStart = starts[firstLine];
            const size_t targetEnd = lastLine+1 < starts.size() ? starts[lastLine+1] : before.size();
            const size_t from = document.singleRow ? 0 : std::min(firstOffset + virtualSpace, lastOffset);
            const size_t to = document.singleRow ? 0 : std::max(firstOffset + virtualSpace, lastOffset);
            std::unique_ptr<ISorter> sorter;
            switch (mode) {
                case 1: sorter = std::make_unique<IntegerSorter>(!ascending, from, to); break;
                case 2: sorter = std::make_unique<DecimalCommaSorter>(!ascending, from, to); break;
                case 3: sorter = std::make_unique<DecimalDotSorter>(!ascending, from, to); break;
                case 4: sorter = std::make_unique<LineLengthSorter>(!ascending, from, to); break;
                case 5: sorter = std::make_unique<LexicographicSorter>(!ascending, from, to); break;
                case 6: sorter = std::make_unique<LexicographicCaseInsensitiveSorter>(!ascending, from, to); break;
                case 7: sorter = std::make_unique<ReverseSorter>(!ascending, from, to); break;
            }
            std::vector<std::wstring> lines;
            for (size_t line = firstLine; line <= lastLine; ++line) lines.push_back(wideOf(lineAt(line)));
            size_t failedLine = 0;
            if (firstLine != lastLine) {
                try { sorter->sort(lines); } catch (size_t index) { failedLine = firstLine + index + 1; }
            }
            std::string after = before;
            if (!failedLine) {
                std::wstring joined;
                for (size_t index = 0; index < lines.size(); ++index) {
                    if (index) joined += L'\n';
                    joined += lines[index];
                }
                if (lastLine+1 < starts.size()) joined += L'\n';
                after.replace(targetStart, targetEnd-targetStart, bytesOf(joined));
            }
            SelectionRange primary(caret, caret);
            SelectionRange rectangle(SelectionPosition(caret, caretVirtual), SelectionPosition(anchor, anchorVirtual));
            const bool changed = after != before;
            if (changed) {
                primary.MoveForInsertDelete(false, targetStart, targetEnd-targetStart);
                rectangle.MoveForInsertDelete(false, targetStart, targetEnd-targetStart);
                primary.MoveForInsertDelete(true, targetStart, targetEnd-targetStart);
                rectangle.MoveForInsertDelete(true, targetStart, targetEnd-targetStart);
            }
            output << hexOf(before) << '\t' << anchor << '\t' << caret << '\t' << anchorVirtual << '\t' << caretVirtual
                   << '\t' << mode << '\t' << ascending << '\t' << hexOf(after) << '\t' << changed << '\t' << failedLine
                   << '\t' << primary.caret.Position() << '\t' << rectangle.anchor.Position() << '\t' << rectangle.caret.Position()
                   << '\t' << rectangle.anchor.VirtualSpace() + rectangle.caret.VirtualSpace() << '\n';
        }
    }
    return output ? 0 : 3;
}
