#pragma once

#include <algorithm>
#include <cstdint>
#include <cstring>
#include <limits>
#include <stdexcept>
#include <string>
#include <string_view>
#include <vector>
#include "Scintilla.h"

using UINT = unsigned int;
using DWORD = unsigned long;
using LPCWSTR = const wchar_t *;
using LPWSTR = wchar_t *;
using LPARAM = intptr_t;

constexpr UINT CP_ACP = 0;
constexpr UINT CP_UTF8 = 65001;
constexpr DWORD LCMAP_SORTKEY = 1;
constexpr DWORD NORM_LINGUISTIC_CASING = 2;
constexpr DWORD LINGUISTIC_IGNORECASE = 4;
constexpr DWORD SORT_DIGITSASNUMBERS = 8;
constexpr DWORD LINGUISTIC_IGNOREDIACRITIC = 16;
constexpr DWORD NORM_IGNORESYMBOLS = 32;
constexpr UINT MB_ICONWARNING = 0x30;
constexpr UINT MB_ICONERROR = 0x10;
constexpr UINT WM_SETREDRAW = 0x0b;
constexpr int FALSE = 0;
constexpr LPCWSTR LOCALE_NAME_USER_DEFAULT = nullptr;

int MultiByteToWideChar(UINT, DWORD, const char *, int, wchar_t *, int);
int LCMapStringEx(LPCWSTR, DWORD, LPCWSTR, int, LPWSTR, int, void *, void *, intptr_t);
inline intptr_t SendMessage(void *, UINT, int, intptr_t) { return 0; }

struct LocaleSelectionRow {
    intptr_t anchor, caret, anchorVirtual, caretVirtual;
};

class ScintillaEditView {
public:
    std::string value;
    std::vector<LocaleSelectionRow> rows;
    intptr_t anchor = 0, caret = 0, anchorVirtual = 0, caretVirtual = 0;
    intptr_t targetStart = 0, targetEnd = 0, replacements = 0;

    std::vector<intptr_t> starts() const {
        std::vector<intptr_t> result{0};
        for (size_t index = 0; index < value.size(); ++index)
            if (value[index] == '\n') result.push_back(index+1);
        return result;
    }

    intptr_t execute(unsigned int message, intptr_t first = 0, intptr_t second = 0) {
        switch (message) {
            case SCI_GETCODEPAGE: return CP_UTF8;
            case SCI_GETSELECTIONMODE: return SC_SEL_RECTANGLE;
            case SCI_GETSELECTIONS: return rows.size();
            case SCI_GETSELECTIONEMPTY:
                return std::all_of(rows.begin(), rows.end(), [](const LocaleSelectionRow &row) {
                    return row.anchor == row.caret && row.anchorVirtual == row.caretVirtual;
                });
            case SCI_GETRECTANGULARSELECTIONANCHOR: return anchor;
            case SCI_GETRECTANGULARSELECTIONCARET: return caret;
            case SCI_GETLINECOUNT: return starts().size();
            case SCI_GETLENGTH: return value.size();
            case SCI_LINEFROMPOSITION: {
                const auto positions = starts();
                return std::upper_bound(positions.begin(), positions.end(), first) - positions.begin() - 1;
            }
            case SCI_POSITIONFROMLINE: return starts().at(first);
            case SCI_GETLINEENDPOSITION: {
                const auto position = value.find('\n', starts().at(first));
                return position == std::string::npos ? value.size() : position;
            }
            case SCI_GETSELECTIONNSTART: return std::min(rows.at(first).anchor, rows.at(first).caret);
            case SCI_GETSELECTIONNEND: return std::max(rows.at(first).anchor, rows.at(first).caret);
            case SCI_GETSELECTIONNANCHOR: return rows.at(first).anchor;
            case SCI_GETSELECTIONNCARET: return rows.at(first).caret;
            case SCI_GETSELECTIONNANCHORVIRTUALSPACE: return rows.at(first).anchorVirtual;
            case SCI_GETSELECTIONNCARETVIRTUALSPACE: return rows.at(first).caretVirtual;
            case SCI_GETEOLMODE: return SC_EOL_LF;
            case SCI_GETRANGEPOINTER: return reinterpret_cast<intptr_t>(value.data()+first);
            case SCI_SETTARGETRANGE: targetStart = first; targetEnd = second; return 0;
            case SCI_SETSTATUS: return 0;
            case SCI_GETSTATUS: return SC_STATUS_OK;
            case SCI_REPLACETARGET:
                value.replace(targetStart, targetEnd-targetStart, reinterpret_cast<const char *>(second), first);
                replacements++;
                return first;
            case SCI_SETRECTANGULARSELECTIONANCHOR: anchor = first; return 0;
            case SCI_SETRECTANGULARSELECTIONCARET: caret = first; return 0;
            case SCI_SETRECTANGULARSELECTIONANCHORVIRTUALSPACE: anchorVirtual = first; return 0;
            case SCI_SETRECTANGULARSELECTIONCARETVIRTUALSPACE: caretVirtual = first; return 0;
            default: throw std::runtime_error("unexpected Scintilla operation in Locale adapter");
        }
    }

    void *getHSelf() const { return nullptr; }
};

class SortLocale {
public:
    struct Result {
        UINT status = 0;
        std::string tagName;
        std::wstring message;
    };
    std::wstring localeName;
    bool caseSensitive = false;
    bool digitsAsNumbers = true;
    bool ignoreDiacritics = false;
    bool ignoreSymbols = false;
    Result sort(ScintillaEditView *, bool) const;
};
