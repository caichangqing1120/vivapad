`vivapad-icon.png`, `vivapad-icon-128.png`, `vivapad-icon-64.png` and
`vivapad-icon-32.png` are the original, user-approved VivaPad PNG assets from
the final Figma resource board (nodes 2038:2831, 2038:2833, 2038:2835 and
2038:2837). Preserve their original bytes, transparent pixels and proportions.

The SVG files in `icons/` are the Lucide assets exported from the same approved
Figma design. Lucide uses the ISC license; some icons derive from Feather
under MIT. See `icons/LICENSE`. `tools/icon` compiles the unmodified SVG paths
to Viva's native `ui.Icon` values. The 16px export corresponds to Lucide's
24-unit grid and 2-unit stroke; its effective stroke is 4/3px.

`viva-mark.png` is retained as an older reference. It is rasterized from Viva's
`website/src/assets/viva-mark-only.svg` (Viva Authors, Apache-2.0), and is no
longer composed into the application icon.
