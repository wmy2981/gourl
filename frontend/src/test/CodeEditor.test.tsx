import { render } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import CodeEditor from '../components/CodeEditor'

// CodeMirror tracks the editor size through ResizeObserver; jsdom lacks it.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  // Test-only polyfill: jsdom ships no ResizeObserver.
  globalThis.ResizeObserver = ResizeObserverStub
})

it('puts the batch-editor class on the CodeMirror element', () => {
  // The dialog editor's styles in index.css are scoped to
  // .cm-editor.batch-editor (fixed height, dark-mode gutter, tok-* token
  // colors); without the class the editor grows to its content height and
  // overflows the dialog and the batch-create highlighting never applies.
  const { container } = render(
    <CodeEditor value="[a](2030/1/1)https://example.com/page" onChange={() => {}} />,
  )
  expect(container.querySelector('.cm-editor')).toHaveClass('batch-editor')
})

it('renders the document into the content area', () => {
  const { container } = render(<CodeEditor value="hello" onChange={() => {}} />)
  expect(container.querySelector('.cm-content')).toHaveTextContent('hello')
})

it('fires onBlur when the editable area loses focus', async () => {
  const onBlur = vi.fn()
  const { container } = render(
    <CodeEditor value="hello" onChange={() => {}} onBlur={onBlur} />,
  )
  const content = container.querySelector('.cm-content') as HTMLElement
  content.focus()
  // CodeMirror dispatches focus changes on a ~10ms timer; let the focus
  // transaction settle before blurring or the two timers race.
  await new Promise((r) => setTimeout(r, 30))
  expect(onBlur).not.toHaveBeenCalled()
  content.blur()
  await new Promise((r) => setTimeout(r, 30))
  expect(onBlur).toHaveBeenCalledTimes(1)
})
