import { describe, expect, it } from 'vitest'

import { i18n } from '@/i18n'
import upstreamEn from '@/i18n/locales/en'
import upstreamZh from '@/i18n/locales/zh'
import { setAffiliateUsageWording } from '../i18n'
import affiliateUsageWording from '../i18n/affiliateUsageWording'
import en from '../i18n/en'
import zh from '../i18n/zh'

// Upstream's locale completeness test only knows upstream locale files, so fork code
// references its messages through mt()/mokKey() and this test checks those keys instead.

function flattenLeaves(value: unknown, prefix = '', leaves: Record<string, unknown> = {}): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    if (prefix) leaves[prefix] = value
    return leaves
  }
  for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
    flattenLeaves(child, prefix ? `${prefix}.${key}` : key, leaves)
  }
  return leaves
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

describe('affiliate usage wording', () => {
  const upstream: Record<string, Record<string, unknown>> = { en: upstreamEn, zh: upstreamZh }
  const placeholders = (text: unknown) => [...String(text).matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort()

  it('rewords the same keys in English and Chinese', () => {
    expect(Object.keys(affiliateUsageWording).sort()).toEqual(['en', 'zh'])
    expect(Object.keys(flattenLeaves(affiliateUsageWording.zh)).sort()).toEqual(
      Object.keys(flattenLeaves(affiliateUsageWording.en)).sort()
    )
  })

  it('only rewords upstream sentences that still exist, keeping their placeholders', () => {
    for (const [locale, wording] of Object.entries(affiliateUsageWording)) {
      const upstreamLeaves = flattenLeaves(upstream[locale])
      for (const [key, text] of Object.entries(flattenLeaves(wording))) {
        expect(key in upstreamLeaves, `${locale}: ${key}`).toBe(true)
        expect(placeholders(text), `${locale}: ${key}`).toEqual(placeholders(upstreamLeaves[key]))
      }
    }
  })

  it('swaps the upstream sentences in and out', () => {
    i18n.global.setLocaleMessage('en', upstreamEn as any)
    i18n.global.locale.value = 'en'
    const key = 'affiliate.tips.line2'
    const upstreamText = i18n.global.t(key, { rate: '10%' })

    setAffiliateUsageWording(true)
    const reworded = i18n.global.t(key, { rate: '10%' })
    expect(reworded).not.toBe(upstreamText)
    expect(reworded).toContain('10%')

    setAffiliateUsageWording(true)
    setAffiliateUsageWording(false)
    expect(i18n.global.t(key, { rate: '10%' })).toBe(upstreamText)
  })
})
