// Keep the cooldown when switching between submission detail pages.
const downloads = new Map<string, number>()
const interval = 5000

export function downloadTestdataFile(
  accountID: number,
  problemID: number,
  caseID: string,
  kind: 'input' | 'output',
): boolean {
  const now = Date.now()
  for (const [key, until] of downloads) {
    if (until <= now) downloads.delete(key)
  }
  const key = `${accountID}/${problemID}/${caseID}/${kind}`
  if (downloads.has(key)) return false
  downloads.set(key, now + interval)
  const link = document.createElement('a')
  link.href = `/api/problems/${problemID}/testdata/${encodeURIComponent(caseID)}/${kind}`
  link.download = `${caseID}.${kind === 'input' ? 'in' : 'out'}`
  document.body.appendChild(link)
  link.click()
  link.remove()
  return true
}
