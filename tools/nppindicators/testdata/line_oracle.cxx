#include <algorithm>
#include <array>
#include <cassert>
#include <clocale>
#include <codecvt>
#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <ctime>
#include <fstream>
#include <limits>
#include <locale>
#include <memory>
#include <optional>
#include <stdexcept>
#include <string>
#include <string_view>
#include <unordered_set>
#include <vector>

using _locale_t = void *;
_locale_t _create_locale(int, const char *);
void _free_locale(_locale_t);
int _wcsicmp(const wchar_t *, const wchar_t *);
double stodLocale(const std::wstring &, _locale_t);
std::wstring stringTakeWhileAdmissable(const std::wstring &, const std::wstring &);
std::wstring stringReplace(const std::wstring &, const std::wstring &, const std::wstring &);

#include "Sorters.h"
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
    std::string action;
    std::string before;
    int selectionStart;
    int selectionEnd;
    std::string offsets;
    std::vector<Mutation> edits;
};

std::wstring wideOf(const std::string &value) {
    std::wstring_convert<std::codecvt_utf8_utf16<wchar_t>> converter;
    return converter.from_bytes(value);
}

std::string bytesOf(const std::wstring &value) {
    std::wstring_convert<std::codecvt_utf8_utf16<wchar_t>> converter;
    return converter.to_bytes(value);
}

std::vector<Mutation> orderedReplacement(const Scenario &scenario) {
    std::vector<int> starts{0};
    for (int position = 0; position < static_cast<int>(scenario.before.size()); ++position)
        if (scenario.before[position] == '\n') starts.push_back(position + 1);
    int first = 0;
    int last = starts.size() - 1;
    if (scenario.selectionStart != scenario.selectionEnd) {
        const int start = std::min(scenario.selectionStart, scenario.selectionEnd);
        const int end = std::max(scenario.selectionStart, scenario.selectionEnd);
        first = std::upper_bound(starts.begin(), starts.end(), start) - starts.begin() - 1;
        last = std::upper_bound(starts.begin(), starts.end(), end) - starts.begin() - 1;
        if (first != last && starts[last] == end) --last;
    }
    if (first >= last) return {};
    const int start = starts[first];
    const int end = last + 1 < static_cast<int>(starts.size()) ? starts[last + 1] : scenario.before.size();
    const std::wstring value = wideOf(scenario.before.substr(start, end - start));
    std::vector<std::wstring> lines;
    size_t position = 0;
    for (size_t separator = value.find(L'\n'); separator != std::wstring::npos; separator = value.find(L'\n', position)) {
        lines.push_back(value.substr(position, separator - position));
        position = separator + 1;
    }
    lines.push_back(value.substr(position));
    const bool reachesLastLine = last == static_cast<int>(starts.size()) - 1;
    if (!reachesLastLine && lines.back().empty()) lines.pop_back();
    if (scenario.action == "unique") {
        std::unordered_set<std::wstring> seen;
        const auto endUnique = std::remove_if(lines.begin(), lines.end(), [&](const std::wstring &line) { return !seen.insert(line).second; });
        lines.erase(endUnique, lines.end());
    } else {
        std::unique_ptr<ISorter> sorter;
        if (scenario.action == "reverse") sorter = std::make_unique<ReverseSorter>(false, 0, 0);
        else if (scenario.action == "sort-length") sorter = std::make_unique<LineLengthSorter>(false, 0, 0);
        else if (scenario.action == "sort-integer") sorter = std::make_unique<IntegerSorter>(false, 0, 0);
        else sorter = std::make_unique<LexicographicSorter>(scenario.action == "sort-desc", 0, 0);
        sorter->sort(lines);
    }
    std::wstring joined;
    for (const auto &line : lines) {
        if (!joined.empty() || &line != &lines.front()) joined += L'\n';
        joined += line;
    }
    if (!reachesLastLine) joined += L'\n';
    const std::string result = bytesOf(joined);
    if (result == scenario.before.substr(start, end - start)) return {};
    return {{start, end - start, result}};
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
        output << scenario.name << '\t' << scenario.action << '\t' << hexOf(scenario.before) << '\t'
               << scenario.selectionStart << '\t' << scenario.selectionEnd << '\t' << scenario.offsets << '\t'
               << initial << '\t' << phase << '\t' << hexOf(value) << '\t'
               << spansOf(*decorations, value.size()) << '\t' << !scenario.edits.empty() << '\n';
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

std::vector<Scenario> lineScenarios() {
    std::vector<Scenario> scenarios{
        {"join", "join", "a\nb\nc\n", 0, 4, "-", {{1, 1, " "}}},
        {"join-backward", "join", "a\nb", 3, 0, "-", {{1, 1, " "}}},
        {"join-one-line", "join", "a\nb", 0, 1, "-", {}},
        {"split", "split", "a b c\nz", 0, 0, "2,4,6", {{2, 0, "\n"}, {5, 0, "\n"}}},
        {"split-selected", "split", "ab cd\nef gh\ntail", 6, 12, "3,9", {{9, 0, "\n"}}},
        {"split-unicode", "split", "\xe7\x8c\xab\xe7\x8a\xac\xe9\xb3\xa5\xe8\xaa\x9e\nlast", 0, 0, "1,6", {{6, 0, "\n"}}},
        {"split-no-break", "split", "abc\nnext", 1, 1, "-", {}},
        {"empty", "empty", "a\n\nb\n", 0, 0, "-", {{2, 1, ""}, {3, 1, ""}}},
        {"blank", "blank", "a\n \t\nb\n", 0, 0, "-", {{2, 3, ""}, {3, 1, ""}}},
        {"blank-unicode", "blank", "a\n\xc2\xa0\nb", 0, 0, "-", {}},
        {"empty-terminal", "empty", "a\n", 0, 0, "-", {{1, 1, ""}}},
        {"blank-terminal", "blank", "a\n \t", 0, 0, "-", {{1, 3, ""}}},
        {"blank-single", "blank", " \t", 0, 0, "-", {{0, 2, ""}}},
        {"empty-selected", "empty", "a\n\nb\n", 1, 3, "-", {{2, 1, ""}}},
        {"blank-partial", "blank", "head\n  \nbody", 5, 7, "-", {}},
        {"consecutive", "consecutive", "a\na\nb", 0, 0, "-", {{2, 2, ""}}},
        {"consecutive-eof", "consecutive", "a\na", 0, 0, "-", {{0, 2, ""}}},
        {"consecutive-three-eof", "consecutive", "a\na\na", 0, 0, "-", {{2, 2, ""}, {0, 2, ""}}},
        {"consecutive-nonadjacent", "consecutive", "a\na\nb\na\n", 0, 0, "-", {{2, 2, ""}}},
        {"consecutive-empty", "consecutive", "\n\nx", 0, 0, "-", {{1, 1, ""}}},
        {"consecutive-selected", "consecutive", "z\na\na\nz", 2, 6, "-", {{4, 2, ""}}},
        {"sort-eof", "sort-asc", "z\nb\na\n", 0, 0, "-", {}},
        {"reverse-eof", "reverse", "z\nb\na\n", 0, 0, "-", {}},
        {"sort-selected-eof", "sort-asc", "z\nb\na\n", 0, 6, "-", {}},
        {"sort-partial", "sort-asc", "x\nb\na\nz", 2, 6, "-", {}},
        {"sort-single-eof", "sort-asc", "z\n", 0, 0, "-", {}},
        {"sort-utf16", "sort-asc", "\xee\x80\x80\n\xf0\x90\x80\x80", 0, 0, "-", {}},
        {"integer-utf16", "sort-integer", "x\xee\x80\x80\nx\xf0\x90\x80\x80", 0, 0, "-", {}},
        {"length-eof", "sort-length", "bbb\na\n", 0, 0, "-", {}},
        {"integer-eof", "sort-integer", "v2\nv10\nv1\n", 0, 0, "-", {}},
        {"sort-no-op", "sort-asc", "a\nb", 0, 0, "-", {}},
        {"unique-eof", "unique", "b\n\nx\nb\n", 0, 0, "-", {}},
        {"unique-selected-eof", "unique", "b\n\nx\nb\n", 0, 7, "-", {}},
        {"unique-partial", "unique", "x\na\nb\na\nz", 2, 8, "-", {}},
        {"unique-no-op", "unique", "a\nb\n", 0, 0, "-", {}},
        {"locale-equal", "locale-equal", "a\nb\n", 0, 0, "-", {{0, 4, "a\nb\n"}}},
        {"above", "above", "a\nb", 2, 2, "-", {{1, 0, "\n"}}},
        {"below", "below", "a\nb", 0, 0, "-", {{2, 0, "\n"}}},
        {"below-eof", "below", "a", 1, 1, "-", {{1, 0, "\n"}}},
    };
    for (auto &scenario : scenarios) {
        if (scenario.action.find("sort-") == 0 || scenario.action == "reverse" || scenario.action == "unique")
            scenario.edits = orderedReplacement(scenario);
    }
    return scenarios;
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    if (wideOf("\xf0\x90\x80\x80").size() != 2) return 4;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Notepad++ v8.9.8.1 dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Original Sorters.h UTF-16 units and Decoration; other normalized-LF command targets transcribed from source\n";
    output << "case\taction\tbefore-hex\tselection-start-byte\tselection-end-byte\toffsets\tinitial\tphase\tvalue-hex\tspans\tchanged\n";
    for (const auto &scenario : lineScenarios()) {
        run(output, scenario, {});
        const int length = scenario.before.size();
        for (int start = 0; start < length; ++start) {
            for (int end = start + 1; end <= length; ++end) run(output, scenario, {{start, end}});
            for (int other = start + 2; other < length; ++other) run(output, scenario, {{start, start + 1}, {other, other + 1}});
        }
    }
    return output ? 0 : 3;
}
