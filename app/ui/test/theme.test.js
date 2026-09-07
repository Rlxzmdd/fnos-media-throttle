import test from 'node:test'
import assert from 'node:assert/strict'
import { applyTheme, connectFNOSThemeSDK, parseThemeMode, readFNOSTheme } from '../src/theme.js'

const storage = values => ({
  get length () { return Object.keys(values).length },
  key: index => Object.keys(values)[index],
  getItem: key => values[key] ?? null,
  setItem: (key, value) => { values[key] = value }
})

const environment = (values = {}, dark = false, search = '') => {
  const localStorage = storage(values)
  const win = { localStorage, location: { search }, matchMedia: () => ({ matches: dark }) }
  const root = { dataset: {} }
  const doc = { documentElement: root, body: { getAttribute: () => null } }
  return { win, doc, root }
}

test('fnOS numeric and named theme values are recognized', () => {
  assert.equal(parseThemeMode(10), 'light')
  assert.equal(parseThemeMode(20), 'dark')
  assert.equal(parseThemeMode(30), 'system')
  assert.equal(parseThemeMode('"dark"'), 'dark')
})

test('system theme reads fnOS desktop configuration before browser fallback', () => {
  const { win, doc, root } = environment({
    'DesktopConfig-1000': JSON.stringify({ userPreference: { theme: 20 } })
  }, false)
  assert.equal(readFNOSTheme(win, doc), 'dark')
  assert.equal(applyTheme('system', win, doc), 'dark')
  assert.equal(root.dataset.theme, 'dark')
})

test('current fnOS os-theme-mode key takes precedence over its legacy key', () => {
  const { win, doc } = environment({
    'os-theme-mode': '10',
    'fnos-theme-mode': '20'
  }, true)
  assert.equal(readFNOSTheme(win, doc), 'light')
})

test('manual app theme overrides fnOS and browser themes', () => {
  const { win, doc, root } = environment({ 'fnos-theme-mode': 'dark' }, true)
  assert.equal(applyTheme('light', win, doc), 'light')
  assert.equal(root.dataset.theme, 'light')
})

test('official fnOS SDK supplies initial theme and subsequent host changes', async () => {
  const received = []
  let listener
  let removed
  const sdk = {
    isWeb: true,
    isStandaloneWeb: false,
    getPlatformConfig: async () => ({ theme: 'light' }),
    $on: async (event, callback) => {
      assert.equal(event, 'os/theme')
      listener = callback
    },
    $off: async (event, callback) => {
      removed = [event, callback]
    }
  }

  const disconnect = await connectFNOSThemeSDK(theme => received.push(theme), async () => sdk)
  listener('dark')
  assert.deepEqual(received, ['light', 'dark'])
  await disconnect()
  assert.deepEqual(removed, ['os/theme', listener])
})
