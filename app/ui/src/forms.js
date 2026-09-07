const currentFNOSUserID = () => {
  for (const key of Object.keys(localStorage)) {
    if (!/user.*id|uid/i.test(key)) continue
    const value = localStorage.getItem(key) || ''
    const matched = value.match(/"(?:uid|userId)"\s*:\s*"?(\d+)/i) || value.match(/^\d+$/)
    if (matched) return Number(matched[1] || matched[0])
  }
  return 1000
}

export const blankDirection = () => ({ enabled: true, baseKB: 10000, stepKB: 2000, minKB: 0 })
export const blankChain = () => ({ id: 0, name: '', enabled: true, libraryIds: [], downloaderIds: [], rule: { upload: blankDirection(), download: blankDirection() } })
export const blankLibrary = () => ({ id: 0, name: '', kind: 'fnos', baseUrl: '/usr/local/apps/@appdata/trim.media/database/trimmedia.db', apiKey: '', pollSeconds: 5, activitySeconds: 60, countLocalClients: true, enabled: true })
export const blankDownloader = () => ({ id: 0, name: '', kind: 'fnos', baseUrl: 'ws://127.0.0.1:5666/websocket?type=main', username: '', password: '', userId: currentFNOSUserID(), enabled: true, pollSeconds: 5 })
