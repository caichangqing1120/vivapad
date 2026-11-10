#include <codecvt>
#include <fstream>
#include <iomanip>
#include <locale>
#include <sstream>
#include "locale_adapter/SortLocale.h"

int MultiByteToWideChar(UINT, DWORD, const char *source, int length, wchar_t *destination, int capacity) {
    if (length < 0) length = std::strlen(source)+1;
    std::wstring_convert<std::codecvt_utf8_utf16<wchar_t>> converter;
    const auto wide = converter.from_bytes(source, source+length);
    if (destination) {
        if (capacity < static_cast<int>(wide.size())) return 0;
        std::copy(wide.begin(), wide.end(), destination);
    }
    return wide.size();
}

int LCMapStringEx(LPCWSTR, DWORD options, LPCWSTR source, int length, LPWSTR destination, int capacity, void *, void *, intptr_t) {
    std::ostringstream key;
    for (int index = 0; index < length;) {
        wchar_t unit = source[index];
        if ((options & SORT_DIGITSASNUMBERS) && unit >= L'0' && unit <= L'9') {
            std::string number;
            while (index < length && source[index] >= L'0' && source[index] <= L'9') number += static_cast<char>(source[index++]);
            const auto first = number.find_first_not_of('0');
            number = first == std::string::npos ? "0" : number.substr(first);
            key << "U0030N" << std::setw(8) << std::setfill('0') << number.size() << ':' << number << '.';
            continue;
        }
        if ((options & LINGUISTIC_IGNORECASE) && unit >= L'A' && unit <= L'Z') unit += L'a'-L'A';
        key << 'U' << std::hex << std::setw(4) << std::setfill('0') << static_cast<unsigned int>(unit) << std::dec;
        index++;
    }
    std::string bytes = key.str();
    bytes.push_back('\0');
    if (destination) {
        if (capacity < static_cast<int>(bytes.size())) return 0;
        std::memcpy(destination, bytes.data(), bytes.size());
    }
    return bytes.size();
}

struct Point {
    intptr_t offset, virtualSpace, column;
};

Point atColumn(const std::string &line, intptr_t target, int tabWidth) {
    std::wstring_convert<std::codecvt_utf8<char32_t>, char32_t> converter;
    const auto characters = converter.from_bytes(line);
    intptr_t column = 0, offset = 0;
    for (const char32_t character : characters) {
        const int width = character == U'\t' ? tabWidth-column%tabWidth : character == 0x732b || character == 0x1f600 ? 2 : 1;
        if (target <= column || (target < column+width && target*2 < 2*column+width)) return {offset, 0, column};
        column += width;
        offset += converter.to_bytes(character).size();
        if (target < column) return {offset, 0, column};
    }
    const auto virtualSpace = std::max<intptr_t>(0, target-column);
    return {offset, virtualSpace, column+virtualSpace};
}

std::string hexOf(const std::string &value) {
    std::ostringstream result;
    for (const unsigned char character : value) result << std::hex << std::setw(2) << std::setfill('0') << static_cast<unsigned int>(character);
    return result.str();
}

int main(int argumentCount, char **arguments) {
    if (argumentCount != 2) return 1;
    std::ofstream output(arguments[1]);
    if (!output) return 2;
    output << "# Original SortLocale.cpp at Notepad++ dd40fe45598791e9aa38f3aeb200e10fb3d6338a\n";
    output << "# Explicit fixed-cell Scintilla adapter and ASCII/numeric host-key stand-in; not Windows NLS or native geometry acceptance\n";
    output << "before-hex\tanchor-byte\tcaret-byte\tanchor-virtual\tcaret-virtual\ttab-width\tascending\tafter-hex\tafter-anchor-byte\tafter-caret-byte\tafter-anchor-virtual\tafter-caret-virtual\treplacements\ttarget-start\ttarget-end\n";
    struct Document { std::string value; bool outside = false; bool single = false; };
    const std::vector<Document> documents{
        {"outside\nz 22 keep\na 1 keep\nm 3 keep\ntail", true},
        {"z\t22\na\t1\nm\t3"},
        {"\xe7\x8c\xab" "22\n\xe7\x8c\xab" "1\n\xe7\x8c\xab" "3"},
        {"\xf0\x9f\x98\x80" "22\n\xf0\x9f\x98\x80" "1\n\xf0\x9f\x98\x80" "3"},
        {"z 22\nx\nm 3"},
        {"z2\na1\n"},
        {"z 1\na 2"},
        {"z 22\na 1\nm 3", false, true},
        {"\n\n"},
        {"z A2\na a2\nm B1"},
    };
    const std::vector<std::pair<int, int>> columns{{0, 0}, {1, 3}, {2, 4}, {4, 2}, {2, 2}, {8, 10}, {0, 6}};
    for (const auto &document : documents) {
        ScintillaEditView input;
        input.value = document.value;
        const auto starts = input.starts();
        const size_t first = document.outside ? 1 : 0;
        const size_t last = document.single ? first : starts.size()-(document.outside ? 2 : 1);
        const auto lineAt = [&](size_t line) {
            const auto end = input.value.find('\n', starts[line]);
            return input.value.substr(starts[line], end == std::string::npos ? end : end-starts[line]);
        };
        for (const auto &column : columns)
        for (const int tabWidth : {2, 4, 8})
        for (const bool reverse : {false, true})
        for (const bool ascending : {false, true}) {
            const auto anchorPoint = atColumn(lineAt(first), column.first, tabWidth);
            const auto caretPoint = atColumn(lineAt(last), column.second, tabWidth);
            ScintillaEditView view;
            view.value = input.value;
            view.anchor = starts[first]+anchorPoint.offset;
            view.caret = starts[last]+caretPoint.offset;
            view.anchorVirtual = anchorPoint.virtualSpace;
            view.caretVirtual = caretPoint.virtualSpace;
            for (size_t line = first; line <= last; ++line) {
                const auto anchor = atColumn(lineAt(line), anchorPoint.column, tabWidth);
                const auto caret = atColumn(lineAt(line), caretPoint.column, tabWidth);
                view.rows.push_back({starts[line]+anchor.offset, starts[line]+caret.offset, anchor.virtualSpace, caret.virtualSpace});
            }
            if (reverse) {
                std::swap(view.anchor, view.caret);
                std::swap(view.anchorVirtual, view.caretVirtual);
                for (auto &row : view.rows) {
                    std::swap(row.anchor, row.caret);
                    std::swap(row.anchorVirtual, row.caretVirtual);
                }
                std::reverse(view.rows.begin(), view.rows.end());
            }
            const auto anchor = view.anchor, caret = view.caret;
            const auto anchorVirtual = view.anchorVirtual, caretVirtual = view.caretVirtual;
            SortLocale sorter;
            const auto result = sorter.sort(&view, !ascending);
            if (result.status != 0) return 3;
            output << hexOf(input.value) << '\t' << anchor << '\t' << caret << '\t' << anchorVirtual << '\t' << caretVirtual
                   << '\t' << tabWidth << '\t' << ascending << '\t' << hexOf(view.value) << '\t' << view.anchor << '\t' << view.caret
                   << '\t' << view.anchorVirtual << '\t' << view.caretVirtual << '\t' << view.replacements << '\t' << view.targetStart << '\t' << view.targetEnd << '\n';
        }
    }
    return output ? 0 : 4;
}
