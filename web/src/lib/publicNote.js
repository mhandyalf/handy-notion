export async function fetchPublicNote(base, token, signal, fetcher = fetch) {
  // Never attach private credentials or trigger authenticated logout handling.
  const response = await fetcher(`${base}/api/public/notes/${encodeURIComponent(token)}`, {
    signal, credentials: 'omit', cache: 'no-store', referrerPolicy: 'no-referrer',
  })
  if (!response.ok) throw new Error(response.status === 404 ? 'This shared note is unavailable.' : 'Unable to load this note. Please try again.')
  return response.json()
}