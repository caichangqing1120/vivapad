# Noto Sans SC

`NotoSansSC.ttf` is the Noto Sans SC variable TrueType font from
[Google Fonts](https://github.com/google/fonts/tree/main/ofl/notosanssc),
licensed under the SIL Open Font License 1.1 in `OFL.txt`. It supplies
embedded Chinese glyphs for Linux PDF printing without depending on the
desktop's installed fonts. The upstream file is named `NotoSansSC[wght].ttf`;
only the local filename differs so Go's embed pattern is unambiguous.

`NotoMusic-Regular.ttf` is the [Noto Music font](https://github.com/google/fonts/tree/cc39c4c2c304bc368c9b7c311ea2bcc6013299fe/ofl/notomusic),
licensed under the separate SIL Open Font License 1.1 in `NotoMusic-OFL.txt`.
Linux PDF printing embeds it only for graphemes the primary font lacks, such
as U+1D11E. Unknown glyphs still cause an explicit print failure.
