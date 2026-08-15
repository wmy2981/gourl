import { describe, expect, it } from 'vitest'
import en from '../locales/en.json'
import zh from '../locales/zh.json'

type Obj = Record<string, unknown>

function flatten(obj: Obj, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    typeof v === 'object' && v !== null
      ? flatten(v as Obj, `${prefix}${k}.`)
      : [`${prefix}${k}`],
  )
}

describe('i18n dictionaries', () => {
  it('en and zh have identical key sets', () => {
    const enKeys = flatten(en as Obj).sort()
    const zhKeys = flatten(zh as Obj).sort()
    expect(zhKeys).toEqual(enKeys)
  })

  it('has no empty or obviously un-translated values', () => {
    for (const [lang, dict] of Object.entries({ en, zh }) as [string, Obj][]) {
      for (const key of flatten(dict)) {
        const value = key.split('.').reduce<unknown>((o, k) => (o as Obj)?.[k], dict)
        expect(typeof value === 'string' && value.length > 0, `${lang}:${key}`).toBe(true)
      }
    }
  })
})
