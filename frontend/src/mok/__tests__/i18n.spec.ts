import { describe, expect, it } from 'vitest'

import upstreamEn from '@/i18n/locales/en'
import upstreamZh from '@/i18n/locales/zh'
import en from '../i18n/en'
import zh from '../i18n/zh'

// Upstream's locale completeness test only knows upstream locale files, so fork code
// references its messages through mt()/mokKey() and this test checks those keys instead.

function flattenLeaves(value: unknown, prefix = ''): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return prefix ? { [prefix]: value } : {}
  }
  return Object.entries(value as Record<string, unknown>).reduce<Record<string, unknown>>(
    (acc, [key, child]) => ({ ...acc, ...flattenLeaves(child, prefix ? `${prefix}.${key}` : key) }),
    {}
  )
}

function referencedKeys(): string[] {
  const sources = import.meta.glob('../**/*.{ts,vue}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
  const keys = new Set<string>()
  for (const [path, source] of Object.entries(sources)) {
    if (path.includes('/__tests__/')) continue
    for (const match of source.matchAll(/\b(?:mt|mokKey)\(\s*(['"])([^'"\r\n]+)\1/g)) {
      keys.add(`mok.${match[2]}`)
    }
  }
  return [...keys].sort()
}

describe('fork locale messages', () => {
  const enLeaves = flattenLeaves(en)
  const zhLeaves = flattenLeaves(zh)

  it('keeps English and Chinese keys identical', () => {
    expect(Object.keys(zhLeaves).sort()).toEqual(Object.keys(enLeaves).sort())
  })

  it('has a non-empty string for every key', () => {
    for (const leaves of [enLeaves, zhLeaves]) {
      for (const [key, value] of Object.entries(leaves)) {
        expect(typeof value === 'string' && value.trim() !== '', key).toBe(true)
      }
    }
  })

  it('defines every key the fork code references', () => {
    const keys = referencedKeys()
    expect(keys.length).toBeGreaterThan(0)
    expect(keys.filter((key) => !(key in enLeaves))).toEqual([])
    expect(keys.filter((key) => !(key in zhLeaves))).toEqual([])
  })

  it('stays under the mok namespace, which upstream does not use', () => {
    expect(Object.keys(en)).toEqual(['mok'])
    expect(Object.keys(zh)).toEqual(['mok'])
    expect('mok' in upstreamEn).toBe(false)
    expect('mok' in upstreamZh).toBe(false)
  })
})
