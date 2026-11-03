// Backend base URL, e.g. http://localhost:8090. Set NEXT_PUBLIC_BARS_API_URL
// to point at a non-default bars-backend instance.
export const API_BASE_URL =
  process.env.NEXT_PUBLIC_BARS_API_URL || 'http://localhost:8090'

export function wsURL(path: string, params: Record<string, string>): string {
  const base = API_BASE_URL.replace(/^http/, 'ws')
  const query = new URLSearchParams(params).toString()
  return `${base}${path}?${query}`
}
