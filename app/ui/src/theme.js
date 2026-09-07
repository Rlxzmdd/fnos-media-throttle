const APP_THEME_KEY = 'media-throttle-theme'
const FNOS_CURRENT_THEME_KEY = 'os-theme-mode'
const FNOS_THEME_KEY = 'fnos-theme-mode'
const FNOS_THEME_KEYS = [FNOS_CURRENT_THEME_KEY, FNOS_THEME_KEY]

export const parseThemeMode = value => {
  if (value == null) return null
  if (typeof value === 'number') return ({ 10: 'light', 20: 'dark', 30: 'system' })[value] || null
  if (typeof value === 'object') return parseThemeMode(value?.userPreference?.theme ?? value?.themeMode ?? value?.theme)
  const raw = String(value).trim()
  const normalized = raw.replace(/^['"]|['"]$/g, '').toLowerCase()
  if (['light', 'dark', 'system'].includes(normalized)) return normalized
  if (['10', '20', '30'].includes(normalized)) return parseThemeMode(Number(normalized))
  try { return parseThemeMode(JSON.parse(raw)) } catch { return null }
}

const storageValue = (storage, key) => {
  try { return storage?.getItem(key) } catch { return null }
}

export const readFNOSTheme = (win = window, doc = document) => {
  const params = new URLSearchParams(win.location?.search || '')
  for (const key of [...FNOS_THEME_KEYS, 'theme-mode', 'theme']) {
    const mode = parseThemeMode(params.get(key))
    if (mode) return mode
  }

  for (const key of FNOS_THEME_KEYS) {
    const direct = parseThemeMode(storageValue(win.localStorage, key))
    if (direct) return direct
  }

  try {
    for (let index = 0; index < win.localStorage.length; index++) {
      const key = win.localStorage.key(index)
      if (!/^DesktopConfig-\d+$/.test(key || '')) continue
      const mode = parseThemeMode(storageValue(win.localStorage, key))
      if (mode) return mode
    }
  } catch {}

  return parseThemeMode(doc.body?.getAttribute('theme-mode')) || 'system'
}

const preferredTheme = win => win.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'

export const applyTheme = (preference, win = window, doc = document) => {
  const hostTheme = preference === 'system' ? readFNOSTheme(win, doc) : preference
  const effective = hostTheme === 'system' ? preferredTheme(win) : hostTheme
  doc.documentElement.dataset.theme = effective
  return effective
}

export const connectFNOSThemeSDK = async (onTheme, createSDK) => {
  const factory = createSDK || (async () => {
    const { TrimApp } = await import('@trimjs/web-app')
    return new TrimApp()
  })
  const sdk = await factory()
  const config = await sdk.getPlatformConfig()
  const initialTheme = parseThemeMode(config?.theme)
  if (initialTheme && initialTheme !== 'system') onTheme(initialTheme)

  let subscribed = false
  const handleTheme = value => {
    const theme = parseThemeMode(value)
    if (theme && theme !== 'system') onTheme(theme)
  }
  if (sdk.isWeb === true && sdk.isStandaloneWeb === false) {
    await sdk.$on('os/theme', handleTheme)
    subscribed = true
  }
  return async () => {
    if (subscribed) {
      try { await sdk.$off('os/theme', handleTheme) } catch {}
    }
  }
}

export const followFNOSTheme = (getPreference, win = window, doc = document) => {
  let hostTheme = null
  let stopped = false
  let disconnectSDK = async () => {}
  const refresh = () => applyTheme(getPreference() === 'system' && hostTheme ? hostTheme : getPreference(), win, doc)
  const media = win.matchMedia?.('(prefers-color-scheme: dark)')
  const onStorage = event => {
    if (FNOS_THEME_KEYS.includes(event.key) || /^DesktopConfig-\d+$/.test(event.key || '')) refresh()
  }
  win.addEventListener('storage', onStorage)
  media?.addEventListener?.('change', refresh)
  const observer = typeof MutationObserver === 'undefined' ? null : new MutationObserver(refresh)
  if (doc.body) observer?.observe(doc.body, { attributes: true, attributeFilter: ['theme-mode'] })
  // fnOS may update storage in the embedding window without producing a storage
  // event in the application frame, so polling is required as a compatibility path.
  const timer = win.setInterval(refresh, 500)
  refresh()
  connectFNOSThemeSDK(theme => {
    if (stopped) return
    hostTheme = theme
    refresh()
  }).then(disconnect => {
    if (stopped) disconnect()
    else disconnectSDK = disconnect
  }).catch(() => {
    // Older fnOS releases and standalone browser access do not expose the
    // official host bridge. Keep the compatibility readers active there.
  })
  return () => {
    stopped = true
    disconnectSDK()
    win.clearInterval(timer)
    win.removeEventListener('storage', onStorage)
    media?.removeEventListener?.('change', refresh)
    observer?.disconnect()
  }
}

export const loadAppTheme = storage => storageValue(storage, APP_THEME_KEY) || 'system'
export const saveAppTheme = (storage, theme) => storage.setItem(APP_THEME_KEY, theme)
