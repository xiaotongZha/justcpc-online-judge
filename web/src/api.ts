export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {}
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }
  let response: Response
  try {
    response = await fetch('/api' + path, {
      credentials: 'same-origin',
      ...options,
      headers: { ...headers, ...(options.headers || {}) },
    })
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') throw error
    throw new Error('网络请求失败，请检查连接后重试')
  }
  if (!response.ok) {
    const data = (await response.json().catch(() => ({}))) as { error?: string }
    if (response.status === 404) throw new Error('记录不存在或已删除')
    if (response.status === 403) throw new Error('没有权限访问或修改该内容')
    throw new Error(data.error || `请求失败：${response.status}`)
  }
  return response.status === 204 ? (undefined as T) : (response.json() as Promise<T>)
}
