export const api = async (path, options = {}) => {
  const response = await fetch(`/api/v1${path}`, { ...options, headers: { 'Content-Type': 'application/json', ...(options.headers || {}) } })
  if (response.status === 204) return null
  const body = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(body.error || `请求失败：${response.status}`)
  return body
}
