package document

import (
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

type LegacySpec struct {
	Encoding Encoding
	Label    string
	Group    string
	CodePage int
	codec    encoding.Encoding
}

var legacySpecs = [...]LegacySpec{
	{GB18030, "GB18030", "Chinese", 54936, simplifiedchinese.GB18030},
	{Windows1252, "Windows-1252", "Western European", 1252, charmap.Windows1252},
	{Windows1250, "Windows-1250", "Central European", 1250, charmap.Windows1250},
	{Windows1251, "Windows-1251", "Cyrillic", 1251, charmap.Windows1251},
	{Windows1253, "Windows-1253", "Greek", 1253, charmap.Windows1253},
	{Windows1254, "Windows-1254", "Turkish", 1254, charmap.Windows1254},
	{Windows1255, "Windows-1255", "Hebrew", 1255, charmap.Windows1255},
	{Windows1256, "Windows-1256", "Arabic", 1256, charmap.Windows1256},
	{Windows1257, "Windows-1257", "Baltic", 1257, charmap.Windows1257},
	{Windows1258, "Windows-1258", "Vietnamese", 1258, charmap.Windows1258},
	{ISO8859_1, "ISO 8859-1", "Western European", 28591, charmap.ISO8859_1},
	{ISO8859_2, "ISO 8859-2", "Eastern European", 28592, charmap.ISO8859_2},
	{ISO8859_3, "ISO 8859-3", "Turkish", 28593, charmap.ISO8859_3},
	{ISO8859_4, "ISO 8859-4", "Baltic", 28594, charmap.ISO8859_4},
	{ISO8859_5, "ISO 8859-5", "Cyrillic", 28595, charmap.ISO8859_5},
	{ISO8859_6, "ISO 8859-6", "Arabic", 28596, charmap.ISO8859_6},
	{ISO8859_7, "ISO 8859-7", "Greek", 28597, charmap.ISO8859_7},
	{ISO8859_8, "ISO 8859-8", "Hebrew", 28598, charmap.ISO8859_8},
	{ISO8859_9, "ISO 8859-9", "Turkish", 28599, charmap.ISO8859_9},
	{ISO8859_13, "ISO 8859-13", "Baltic", 28603, charmap.ISO8859_13},
	{ISO8859_14, "ISO 8859-14", "Celtic", 28604, charmap.ISO8859_14},
	{ISO8859_15, "ISO 8859-15", "Western European", 28605, charmap.ISO8859_15},
	{DOS437, "OEM 437", "Western European", 437, charmap.CodePage437},
	{DOS850, "OEM 850", "Western European", 850, charmap.CodePage850},
	{DOS852, "OEM 852", "Central European", 852, charmap.CodePage852},
	{DOS855, "OEM 855", "Cyrillic", 855, charmap.CodePage855},
	{DOS858, "OEM 858", "Western European", 858, charmap.CodePage858},
	{DOS860, "OEM 860", "Western European", 860, charmap.CodePage860},
	{DOS862, "OEM 862", "Hebrew", 862, charmap.CodePage862},
	{DOS863, "OEM 863", "Western European", 863, charmap.CodePage863},
	{DOS865, "OEM 865", "North European", 865, charmap.CodePage865},
	{DOS866, "OEM 866", "Cyrillic", 866, charmap.CodePage866},
	{DOS720, "OEM 720", "Arabic", 720, newOEMEncoding("OEM 720", cp720Table)},
	{DOS737, "OEM 737", "Greek", 737, newOEMEncoding("OEM 737", cp737Table)},
	{DOS775, "OEM 775", "Baltic", 775, newOEMEncoding("OEM 775", cp775Table)},
	{DOS857, "OEM 857", "Turkish", 857, newOEMEncoding("OEM 857", cp857Table)},
	{DOS861, "OEM 861", "North European", 861, newOEMEncoding("OEM 861", cp861Table)},
	{DOS869, "OEM 869", "Greek", 869, newOEMEncoding("OEM 869", cp869Table)},
	{Big5, "Big5", "Chinese", 950, traditionalchinese.Big5},
	{GBK, "GB2312 (GBK)", "Chinese", 936, simplifiedchinese.GBK},
	{ShiftJIS, "Shift-JIS", "Japanese", 932, japanese.ShiftJIS},
	{Windows949, "Windows 949", "Korean", 949, korean.EUCKR},
	{EUCKR, "EUC-KR", "Korean", 51949, korean.EUCKR},
	{TIS620, "TIS-620", "Thai", 874, charmap.Windows874},
	{MacCyrillic, "Macintosh Cyrillic", "Cyrillic", 10007, charmap.MacintoshCyrillic},
	{KOI8U, "KOI8-U", "Cyrillic", 21866, charmap.KOI8U},
	{KOI8R, "KOI8-R", "Cyrillic", 20866, charmap.KOI8R},
}

func LegacyEncodings() []LegacySpec {
	return append([]LegacySpec(nil), legacySpecs[:]...)
}

func (format Encoding) IsLegacy() bool { return legacyCodec(format) != nil }

func (format Encoding) LegacySpec() (LegacySpec, bool) {
	for _, spec := range legacySpecs {
		if spec.Encoding == format {
			return spec, true
		}
	}
	return LegacySpec{}, false
}

func (format Encoding) EncodedLength(value string) (int, error) {
	if format <= UTF16BE {
		return len(value), nil
	}
	data, err := encodeLegacy(value, format)
	return len(data), err
}

func (format Encoding) Valid() bool { return format <= UTF16BE || format.IsLegacy() }

func (format Encoding) CodePage() (int, bool) {
	for _, spec := range legacySpecs {
		if spec.Encoding == format {
			return spec.CodePage, true
		}
	}
	return 0, false
}

func EncodingFromCodePage(codePage int) (Encoding, bool) {
	for _, spec := range legacySpecs {
		if spec.CodePage == codePage {
			return spec.Encoding, true
		}
	}
	return 0, false
}

func EncodingFromLabel(label string) (Encoding, bool) {
	for _, spec := range legacySpecs {
		if strings.EqualFold(spec.Label, label) {
			return spec.Encoding, true
		}
	}
	return 0, false
}
