import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { compileTemplate } from 'vue/compiler-sfc'
import { createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'
import SettingsPanel from '../src/components/SettingsPanel.js'
import { blankChain, blankDownloader } from '../src/forms.js'
import { api } from '../src/api.js'

test('application template compiles after component extraction', async () => {
  const source = await readFile(new URL('../src/main.js', import.meta.url), 'utf8')
  const template = source.match(/template:\s*`([\s\S]*)`\s*\n\}\)/)?.[1]
  assert.ok(template, 'application template not found')
  const result = compileTemplate({ source: template, filename: 'main.js', id: 'application' })
  assert.deepEqual(result.errors, [])
})

test('settings component renders all release modes and selected state', async () => {
  for (const mode of ['original', 'unlimited', 'maximum']) {
    const html = await renderToString(createSSRApp(SettingsPanel, {
      settings: { debug: true, releaseMode: mode }, theme: 'dark', busy: false
    }))
    for (const label of ['恢复原值', '解除限速', '设为最高限速', 'Debug 模式']) assert.ok(html.includes(label))
    assert.equal((html.match(/aria-pressed="true"/g) || []).length, 2)
    assert.ok(html.includes('checked'))
    assert.ok(html.includes('配置备份'))
    assert.ok(html.includes('导出配置'))
    assert.match(html, /class="backup-button secondary"[^>]*>导出配置/)
    assert.match(html, /class="backup-button primary"[^>]*>导入配置/)
  }
})

test('settings shows the packaged application version', async () => {
  const html = await renderToString(createSSRApp(SettingsPanel, {
    settings: { debug: false, releaseMode: 'original', appVersion: '0.9.17' }, theme: 'system', busy: false
  }))
  assert.ok(html.includes('v0.9.17'))
})

test('backup actions use equal fnOS button geometry', async () => {
  const css = await readFile(new URL('../src/style.css', import.meta.url), 'utf8')
  assert.match(css, /\.backup-actions\{[^}]*grid-template-columns:repeat\(2,96px\)/)
  assert.match(css, /\.backup-actions \.backup-button\{[^}]*width:96px[^}]*height:36px[^}]*border:0/)
  assert.match(css, /\.backup-actions \.backup-button\.secondary\{[^}]*color:var\(--blue\)[^}]*box-shadow:inset 0 0 0 1px var\(--blue\)/)
})

test('fresh forms do not share rule objects and default to fnOS', () => {
  const first = blankChain(), second = blankChain()
  first.rule.upload.baseKB = 42
  assert.notEqual(first.rule.upload.baseKB, second.rule.upload.baseKB)
  globalThis.localStorage = {}
  try { assert.equal(blankDownloader().kind, 'fnos') } finally { delete globalThis.localStorage }
})

test('API keeps JSON header when callers add headers and handles errors', async () => {
  const originalFetch = globalThis.fetch
  try {
    globalThis.fetch = async (url, options) => {
      assert.equal(url, '/api/v1/settings')
      assert.equal(options.headers['Content-Type'], 'application/json')
      assert.equal(options.headers['X-Test'], 'yes')
      return new Response(JSON.stringify({ releaseMode: 'maximum' }))
    }
    assert.deepEqual(await api('/settings', { headers: { 'X-Test': 'yes' } }), { releaseMode: 'maximum' })
    globalThis.fetch = async () => new Response(JSON.stringify({ error: 'invalid mode' }), { status: 400 })
    await assert.rejects(api('/settings'), /invalid mode/)
  } finally { globalThis.fetch = originalFetch }
})
