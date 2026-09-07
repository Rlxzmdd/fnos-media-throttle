import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'
import ChainLimits from '../src/components/ChainLimits.js'
import { blankChain } from '../src/forms.js'
import { formatLogMessage } from '../src/log-format.js'

test('chain summaries show ordered metrics and distinguish uncontrolled directions', async () => {
  const chain = blankChain()
  chain.viewerCount = 2
  chain.rule.download.enabled = false
  const html = await renderToString(createSSRApp(ChainLimits, { chain }))
  assert.equal((html.match(/<h4>/g) || []).length, 2)
  assert.ok(html.includes('6000 kb/s'))
  assert.ok(html.includes('未接管'))
  assert.deepEqual([...html.matchAll(/<dt[^>]*>(.*?)<\/dt>/g)].map(match => match[1]), ['最大', '递减', '最小', '当前', '最大', '递减', '最小', '当前'])
  chain.enabled = false
  const disabled = await renderToString(createSSRApp(ChainLimits, { chain }))
  assert.equal((disabled.match(/未启用/g) || []).length, 2)
  chain.enabled = true
  chain.lastError = 'sync failed'
  assert.ok((await renderToString(createSSRApp(ChainLimits, { chain }))).includes('待同步'))
})

test('historical speeds are normalized for display without changing other text', () => {
  assert.equal(formatLogMessage('上传=100000 bytes/s，下载=1536 bytes/s'), '上传=100 kb/s，下载=1.536 kb/s')
  assert.equal(formatLogMessage('上传=1000 KB/s，下载=未修改'), '上传=1000 kb/s，下载=未修改')
  assert.equal(formatLogMessage('错误码 131072'), '错误码 131072')
})

test('labels are concise and closed rule sections have no heading bottom margin', async () => {
  const source = await readFile(new URL('../src/main.js', import.meta.url), 'utf8')
  assert.ok(source.includes('统计本地'))
  assert.ok(source.includes('管理所有影视库'))
  assert.ok(source.includes('管理所有下载器'))
  assert.ok(source.includes('当前 ↑{{item.currentUploadSpeedKB}} kb/s ↓{{item.currentDownloadSpeedKB}} kb/s'))
  for (const old of ['当前上传速度', '最大上传速度', '当前下载速度', '最大下载速度', '计入本地播放']) assert.ok(!source.includes(old))
  const css = await readFile(new URL('../src/style.css', import.meta.url), 'utf8')
  assert.ok(!/\.rule-heading\{[^}]*margin-bottom/.test(css))
  assert.ok(/\.rule-section>\.rule-grid\{margin-top:12px\}/.test(css))
})
