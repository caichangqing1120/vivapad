package document

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
)

type Encoding uint8

const (
	UTF8 Encoding = iota
	UTF8BOM
	UTF16LE
	UTF16BE
	GB18030
	Windows1252
	Windows1250
	Windows1251
	Windows1253
	Windows1254
	Windows1255
	Windows1256
	Windows1257
	Windows1258
	ISO8859_1
	ISO8859_2
	ISO8859_3
	ISO8859_4
	ISO8859_5
	ISO8859_6
	ISO8859_7
	ISO8859_8
	ISO8859_9
	ISO8859_13
	ISO8859_14
	ISO8859_15
	DOS437
	DOS850
	DOS852
	DOS855
	DOS858
	DOS860
	DOS862
	DOS863
	DOS865
	DOS866
	Big5
	GBK
	ShiftJIS
	Windows949
	EUCKR
	TIS620
	MacCyrillic
	KOI8U
	KOI8R
	DOS720
	DOS737
	DOS775
	DOS857
	DOS861
	DOS869
)

func (doc *Document) SetEncoding(encoding Encoding) {
	if encoding.Valid() {
		doc.encoding = encoding
	}
}

func (doc *Document) Encoding() Encoding { return doc.encoding }

func (doc *Document) SavedEncoding() Encoding { return doc.savedEncoding }

func (doc *Document) EncodingLabel() string {
	switch doc.encoding {
	case UTF8BOM:
		return "UTF-8 BOM"
	case UTF16LE:
		return "UTF-16 LE"
	case UTF16BE:
		return "UTF-16 BE"
	case UTF8:
		return "UTF-8"
	}
	return formatLabel(doc.encoding)
}

func legacyCodec(format Encoding) encoding.Encoding {
	for _, spec := range legacySpecs {
		if spec.Encoding == format {
			return spec.codec
		}
	}
	return nil
}

func decodeLegacy(data []byte, format Encoding) (string, Encoding, error) {
	codec := legacyCodec(format)
	if codec == nil {
		return "", 0, fmt.Errorf("unsupported legacy encoding %d", format)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", 0, errors.New("binary NUL in text")
	}
	decoded, err := codec.NewDecoder().Bytes(data)
	if err != nil {
		return "", 0, err
	}
	encoded, err := codec.NewEncoder().Bytes(decoded)
	if err != nil || !bytes.Equal(encoded, data) {
		return "", 0, fmt.Errorf("invalid or non-round-trippable %s text", formatLabel(format))
	}
	return string(decoded), format, nil
}

func formatLabel(format Encoding) string {
	for _, spec := range legacySpecs {
		if spec.Encoding == format {
			return spec.Label
		}
	}
	return "unknown"
}

func decodeText(data []byte) (string, Encoding, error) {
	encoding := UTF8
	switch {
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		encoding, data = UTF8BOM, data[3:]
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		encoding, data = UTF16LE, data[2:]
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		encoding, data = UTF16BE, data[2:]
	}
	if encoding == UTF8 || encoding == UTF8BOM {
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return "", 0, errors.New("not a UTF-8 text file")
		}
		return string(data), encoding, nil
	}
	if len(data)%2 != 0 {
		return "", 0, errors.New("odd-length UTF-16 text")
	}
	var order binary.ByteOrder = binary.LittleEndian
	if encoding == UTF16BE {
		order = binary.BigEndian
	}
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = order.Uint16(data[index*2:])
		if units[index] == 0 {
			return "", 0, errors.New("binary NUL in UTF-16 text")
		}
		if units[index] >= 0xd800 && units[index] <= 0xdbff {
			if index+1 >= len(units) || order.Uint16(data[(index+1)*2:]) < 0xdc00 || order.Uint16(data[(index+1)*2:]) > 0xdfff {
				return "", 0, errors.New("unpaired UTF-16 surrogate")
			}
		} else if units[index] >= 0xdc00 && units[index] <= 0xdfff && (index == 0 || units[index-1] < 0xd800 || units[index-1] > 0xdbff) {
			return "", 0, errors.New("unpaired UTF-16 surrogate")
		}
	}
	return string(utf16.Decode(units)), encoding, nil
}

func encodeText(value string, encoding Encoding) ([]byte, error) {
	switch encoding {
	case UTF8BOM:
		return append([]byte{0xef, 0xbb, 0xbf}, value...), nil
	case UTF16LE, UTF16BE:
		var order binary.ByteOrder = binary.LittleEndian
		bom := []byte{0xff, 0xfe}
		if encoding == UTF16BE {
			order, bom = binary.BigEndian, []byte{0xfe, 0xff}
		}
		units := utf16.Encode([]rune(value))
		data := make([]byte, 2+len(units)*2)
		copy(data, bom)
		for index, unit := range units {
			order.PutUint16(data[2+index*2:], unit)
		}
		return data, nil
	case UTF8:
		return []byte(value), nil
	default:
		codec := legacyCodec(encoding)
		if codec == nil {
			return nil, fmt.Errorf("unsupported text encoding %d", encoding)
		}
		data, err := codec.NewEncoder().Bytes([]byte(value))
		if err != nil {
			return nil, fmt.Errorf("cannot save text as %s: %w", formatLabel(encoding), err)
		}
		return data, nil
	}
}
