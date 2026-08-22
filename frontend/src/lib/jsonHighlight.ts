import type { Extension } from '@codemirror/state'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags as t } from '@lezer/highlight'

// JSON token colors in the VSCode palettes the batch editor already uses:
// One Dark in dark mode (keys orange, strings green), deepened Light+
// equivalents on light backgrounds. Applied via .tok-* classes in index.css
// so the .dark theme switch flips both palettes automatically.
export const jsonHighlight: Extension = syntaxHighlighting(
  HighlightStyle.define([
    { tag: [t.propertyName], class: 'tok-code' },
    { tag: [t.string], class: 'tok-url' },
    { tag: [t.number, t.bool, t.null], class: 'tok-date' },
    { tag: [t.bracket], class: 'tok-comment' },
  ]),
)
