import { useEffect, useRef, useState } from 'react'
import { RangeSet, StateEffect, StateField, type Extension, type Range } from '@codemirror/state'
import {
  Decoration,
  EditorView,
  GutterMarker,
  lineNumberMarkers,
  placeholder,
  type DecorationSet,
} from '@codemirror/view'
import { basicSetup } from 'codemirror'

// Flags 1-based line numbers as invalid: the line number in the gutter turns
// danger-red and the content line gets a faint red wash. The marker sets are
// rebuilt from a StateEffect so they stay in sync with the document. The
// gutter marker carries only an elementClass, which the line-number gutter
// applies to its element (no DOM node is rendered).
class ErrorGutterMarker extends GutterMarker {
  elementClass = 'cm-error-gutter'
}

const setErrorLines = StateEffect.define<readonly number[]>()

const errorField = StateField.define<{ lines: DecorationSet; markers: RangeSet<GutterMarker> }>({
  create: () => ({ lines: Decoration.none, markers: RangeSet.empty }),
  update(value, tr) {
    let { lines, markers } = value
    if (tr.docChanged) {
      lines = lines.map(tr.changes)
      markers = markers.map(tr.changes)
    }
    for (const e of tr.effects) {
      if (!e.is(setErrorLines)) continue
      const lineRanges: Range<Decoration>[] = []
      const markerRanges: Range<GutterMarker>[] = []
      for (const n of e.value) {
        if (n < 1 || n > tr.state.doc.lines) continue
        const from = tr.state.doc.line(n).from
        lineRanges.push(Decoration.line({ class: 'cm-error-line' }).range(from))
        markerRanges.push(new ErrorGutterMarker().range(from))
      }
      return { lines: Decoration.set(lineRanges, true), markers: RangeSet.of(markerRanges, true) }
    }
    return { lines, markers }
  },
})

// CodeMirror 6 editor used by the batch-create and batch-import dialogs:
// always-visible line numbers, theme-aware through CSS variables (the .dark
// class switch applies automatically), and the same visual language as the
// Textarea it replaces. Pass `errorLines` (1-based) to flag invalid lines.
export default function CodeEditor({
  value,
  onChange,
  placeholder: ph,
  ariaLabel,
  id,
  className = '',
  errorLines,
  extensions = [],
  onBlur,
}: {
  value: string
  onChange: (value: string) => void
  /** Shown while the document is empty. */
  placeholder?: string
  /** Accessible name for the editable area. */
  ariaLabel?: string
  /** Id on the wrapper (label association); the editable area carries ariaLabel. */
  id?: string
  /** Height and other layout classes; the editor fills the wrapper. */
  className?: string
  /** 1-based line numbers flagged as invalid (red line number + red wash). */
  errorLines?: readonly number[]
  /** Extra CodeMirror extensions (language highlighting, …). */
  extensions?: Extension[]
  /** Fired when the editable area loses focus. */
  onBlur?: () => void
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [view, setView] = useState<EditorView | null>(null)

  // Mounted once; later prop changes flow through the sync effects below.
  // onBlur is captured in a ref so callers passing an inline closure don't
  // remount (and wipe) the editor on every render.
  const onBlurRef = useRef(onBlur)
  onBlurRef.current = onBlur
  useEffect(() => {
    const parent = containerRef.current
    if (!parent) return
    const view = new EditorView({
      doc: value,
      parent,
      extensions: [
        // The dialog editor's look lives in index.css scoped to
        // .cm-editor.batch-editor (height 100% so the wrapper's h-* size
        // wins, the dark-mode gutter and the tok-* token colors) — without
        // this class the editor grows to its content height and overflows
        // the dialog.
        EditorView.editorAttributes.of({ class: 'batch-editor' }),
        ...(ariaLabel ? [EditorView.contentAttributes.of({ 'aria-label': ariaLabel })] : []),
        basicSetup,
        errorField,
        EditorView.decorations.from(errorField, (f) => f.lines),
        lineNumberMarkers.compute([errorField], (state) => state.field(errorField).markers),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) onChange(u.state.doc.toString())
          if (u.focusChanged && !u.view.hasFocus) onBlurRef.current?.()
        }),
        ...(ph != null ? [placeholder(ph)] : []),
        ...extensions,
      ],
    })
    setView(view)
    return () => view.destroy()
  }, [])

  // External value changes (submit cleanup, file load, dialog reset) replace
  // the document in place.
  useEffect(() => {
    if (!view) return
    const doc = view.state.doc.toString()
    if (doc !== value) view.dispatch({ changes: { from: 0, to: doc.length, insert: value } })
  }, [view, value])

  // Keep the invalid-line flags in sync with the caller's validation.
  useEffect(() => {
    if (!view) return
    view.dispatch({ effects: setErrorLines.of(errorLines ?? []) })
  }, [view, errorLines])

  return <div id={id} ref={containerRef} className={className} />
}
