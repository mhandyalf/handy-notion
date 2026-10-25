import test from 'node:test'
import assert from 'node:assert/strict'
import { fetchPublicNote } from './publicNote.js'

test('public reads omit credentials, caching and referrers and encode the token', async () => {
  const signal = new AbortController().signal
  const note = { title: '<script>alert(1)</script>', content: [] }
  const result = await fetchPublicNote('https://notes.example', 'a/b?', signal, async (url, options) => {
    assert.equal(url, 'https://notes.example/api/public/notes/a%2Fb%3F')
    assert.deepEqual(options, { signal, credentials: 'omit', cache: 'no-store', referrerPolicy: 'no-referrer' })
    return { ok: true, json: async () => note }
  })
  assert.deepEqual(result, note)
})

test('public read failures use generic messages, not server internals', async () => {
  for (const status of [404, 401, 500]) {
    await assert.rejects(fetchPublicNote('', 'token', undefined, async () => ({ ok: false, status })), {
      message: status === 404 ? 'This shared note is unavailable.' : 'Unable to load this note. Please try again.',
    })
  }
})

test('public reads propagate abort and network failures', async () => {
  const error = new Error('aborted')
  error.name = 'AbortError'
  await assert.rejects(fetchPublicNote('', 'token', undefined, async () => { throw error }), { name: 'AbortError' })
})