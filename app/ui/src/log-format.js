// Display old records consistently without rewriting users' historical logs.
export const formatLogMessage = message => String(message ?? '')
  .replace(/(-?\d+(?:\.\d+)?)\s*bytes\/s\b/gi, (_, bytes) => `${Number(bytes) / 1000} kb/s`)
  .replace(/\bKB\/s\b/g, 'kb/s')
