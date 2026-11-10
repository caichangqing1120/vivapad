package document

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

//go:generate go run ../../tools/oemmaps -out oem_tables_generated.go

type oemEncoding struct {
	name    string
	table   [256]rune
	reverse map[rune]byte
}

func newOEMEncoding(name string, table [256]rune) encoding.Encoding {
	reverse := make(map[rune]byte, 256)
	for index, character := range table {
		if character < 0 {
			continue
		}
		if _, exists := reverse[character]; !exists {
			reverse[character] = byte(index)
		}
	}
	return &oemEncoding{name: name, table: table, reverse: reverse}
}

func (codec *oemEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: &oemDecoder{codec: codec}}
}

func (codec *oemEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: &oemEncoder{codec: codec}}
}

type oemDecoder struct{ codec *oemEncoding }

func (decoder *oemDecoder) Reset() {}

func (decoder *oemDecoder) Transform(dst, src []byte, _ bool) (nDst, nSrc int, err error) {
	for nSrc < len(src) {
		character := decoder.codec.table[src[nSrc]]
		if character < 0 {
			return nDst, nSrc, fmt.Errorf("%s undefined byte 0x%02x", decoder.codec.name, src[nSrc])
		}
		width := utf8.RuneLen(character)
		if len(dst)-nDst < width {
			return nDst, nSrc, transform.ErrShortDst
		}
		nDst += utf8.EncodeRune(dst[nDst:], character)
		nSrc++
	}
	return nDst, nSrc, nil
}

type oemEncoder struct{ codec *oemEncoding }

func (encoder *oemEncoder) Reset() {}

func (encoder *oemEncoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for nSrc < len(src) {
		if !utf8.FullRune(src[nSrc:]) {
			if !atEOF {
				return nDst, nSrc, transform.ErrShortSrc
			}
			return nDst, nSrc, fmt.Errorf("%s invalid UTF-8 input", encoder.codec.name)
		}
		character, width := utf8.DecodeRune(src[nSrc:])
		if character == utf8.RuneError && width == 1 {
			return nDst, nSrc, fmt.Errorf("%s invalid UTF-8 input", encoder.codec.name)
		}
		value, ok := encoder.codec.reverse[character]
		if !ok {
			return nDst, nSrc, fmt.Errorf("%s cannot encode U+%04X", encoder.codec.name, character)
		}
		if nDst == len(dst) {
			return nDst, nSrc, transform.ErrShortDst
		}
		dst[nDst] = value
		nDst++
		nSrc += width
	}
	return nDst, nSrc, nil
}
