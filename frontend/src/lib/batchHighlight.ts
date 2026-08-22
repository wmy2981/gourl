import type { Extension } from '@codemirror/state'
import { HighlightStyle, StreamLanguage, language, syntaxHighlighting } from '@codemirror/language'
import { Tag } from '@lezer/highlight'

// Syntax highlighting for the batch-create editor, mirroring the strict
// parser in lib/batch.ts: a leading [code], an immediately following (date),
// then the url; whole-line # comments are dimmed. Custom tags (no default
// colors) keep every token color in the .tok-* classes defined in index.css,
// so the .dark theme switch applies automatically.
const tCode = Tag.define()
const tDate = Tag.define()
const tUrl = Tag.define()
const tComment = Tag.define()

const batchStream = StreamLanguage.define({
  name: 'batch-line',
  tokenTable: { code: tCode, date: tDate, url: tUrl, comment: tComment },
  token(stream) {
    if (stream.eatSpace()) return null
    // A '#' only counts as a comment at the start of the line (after the
    // leading whitespace); '#' inside a url is part of the url.
    if (stream.peek() === '#') {
      stream.skipToEnd()
      return 'comment'
    }
    // [code] — the brackets are included in the token, matching how the
    // parser strips them. A missing ']' (invalid syntax) still highlights
    // to the end of the line.
    if (stream.peek() === '[') {
      stream.next()
      if (!stream.skipTo(']')) stream.skipToEnd()
      else stream.next()
      return 'code'
    }
    // (date) — only valid right after [code] (or at the line start, which
    // the parser accepts as date-without-code).
    if (stream.peek() === '(') {
      stream.next()
      if (!stream.skipTo(')')) stream.skipToEnd()
      else stream.next()
      return 'date'
    }
    stream.skipToEnd()
    return 'url'
  },
  startState: () => ({}),
})

export const batchHighlight: Extension = [
  language.of(batchStream),
  syntaxHighlighting(
    HighlightStyle.define([
      { tag: tCode, class: 'tok-code' },
      { tag: tDate, class: 'tok-date' },
      { tag: tUrl, class: 'tok-url' },
      { tag: tComment, class: 'tok-comment' },
    ]),
  ),
]
