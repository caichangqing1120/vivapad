# Noto Sans SC

`NotoSansSC.ttf` and `NotoSansSC-Bold.ttf` are static instances of the Noto
Sans SC variable font under the SIL Open Font License 1.1 in `OFL.txt`.
The normal file uses the original bundled font's default weight (`100`),
with identical codepoint mappings and horizontal metrics. The bold file
uses weight `700`. The input `NotoSansSC[wght].ttf` is available in this
repository at `a3fc1828:internal/printing/fonts/NotoSansSC.ttf`.

`NotoSans-Italic.ttf` and `NotoSans-BoldItalic.ttf` are width `100`, weight
`400` and `700` instances of `NotoSans-Italic[wdth,wght].ttf` from
[Google Fonts at 2984c575](https://github.com/google/fonts/tree/2984c575fdce412ee02b2baaba67672b9a9434d8/ofl/notosans).
They carry `NotoSans-OFL.txt`. Linux PDF printing uses them for covered
header and footer text, and slants the available CJK face when needed.

To regenerate the four files in a staging directory, use FontTools 4.58.5:

```sh
python3 derive.py "$noto_sc_variable" "$noto_italic_variable" "$output_dir"
```

The script verifies both source hashes, fixes all variation axes, and
preserves source timestamps. FontTools is only needed to regenerate assets.

`NotoMusic-Regular.ttf` is the [Noto Music font](https://github.com/google/fonts/tree/cc39c4c2c304bc368c9b7c311ea2bcc6013299fe/ofl/notomusic),
licensed under the separate SIL Open Font License 1.1 in `NotoMusic-OFL.txt`.
Linux PDF printing embeds it only for graphemes the primary font lacks, such
as U+1D11E. Unknown glyphs still cause an explicit print failure.
