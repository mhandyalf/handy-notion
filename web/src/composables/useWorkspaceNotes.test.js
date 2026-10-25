import test from 'node:test'
import assert from 'node:assert/strict'
import { useWorkspaceNotes } from './useWorkspaceNotes.js'

const note = (id, title = id) => ({ id, title, content: [{ id: 'block', type: 'text', text: '' }],
  icon: '📝', is_favorite: false, is_archived: false, updated_at: '2026-01-01' })
function deferred() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  return { promise, resolve }
}
function setup(t, overrides = {}) {
  const calls = []
  const api = {
    listNotes: async () => ({ notes: [note('a'), note('b')] }),
    getNote: async (id) => note(id),
    updateNote: async (id, value) => { calls.push([id, value]); return { ...note(id), ...value } },
    createNote: async () => note('new'), deleteNote: async () => {}, ...overrides,
  }
  const state = useWorkspaceNotes(api, { delay: 60_000 })
  t.after(() => state.dispose())
  return { state, calls }
}

test('switching notes flushes the latest draft before changing selection', async (t) => {
  const { state, calls } = setup(t)
  await state.initialize('a')
  state.selected.value.title = 'unsaved'
  assert.equal(await state.selectNote(note('b')), true)
  assert.equal(calls[0][0], 'a')
  assert.equal(calls[0][1].title, 'unsaved')
  assert.equal(state.selected.value.id, 'b')
})

test('search refresh never replaces the active draft', async (t) => {
  const { state } = setup(t)
  await state.initialize('a')
  state.selected.value.title = 'draft'
  state.search.value = 'b'
  await state.loadNotes()
  assert.equal(state.selected.value.title, 'draft')
  assert.equal(state.dirty.value, true)
})

test('failed saves preserve draft, block switching, and can be retried', async (t) => {
  let fail = true
  const { state } = setup(t, { updateNote: async (id, value) => {
    if (fail) throw new Error('offline')
    return { ...note(id), ...value }
  } })
  await state.initialize('a')
  state.selected.value.title = 'keep me'
  assert.equal(await state.selectNote(note('b')), false)
  assert.equal(state.selected.value.title, 'keep me')
  assert.equal(state.saveState.value, 'Save failed')
  fail = false
  assert.equal(await state.flush(), true)
  assert.equal(state.dirty.value, false)
})

test('edits during an in-flight save are serialized and saved again', async (t) => {
  const pending = deferred()
  const writes = []
  const { state } = setup(t, { updateNote: async (id, value) => {
    writes.push(value.title)
    if (writes.length === 1) await pending.promise
    return { ...note(id), ...value }
  } })
  await state.initialize('a')
  state.selected.value.title = 'first'
  const saving = state.flush()
  state.selected.value.title = 'second'
  pending.resolve()
  await saving
  assert.deepEqual(writes, ['first', 'second'])
  assert.equal(state.dirty.value, false)
})

test('failed archive preserves the page and restores the archive flag', async (t) => {
  const { state } = setup(t, { updateNote: async () => { throw new Error('offline') } })
  await state.initialize('a')
  assert.equal(await state.toggleArchive(), false)
  assert.equal(state.selected.value.id, 'a')
  assert.equal(state.selected.value.is_archived, false)
  assert.equal(state.notes.value.length, 2)
})

test('out-of-order search responses cannot overwrite newer results', async (t) => {
  const first = deferred()
  let count = 0
  const { state } = setup(t, { listNotes: () => ++count === 1 ? first.promise : Promise.resolve({ notes: [note('new')] }) })
  const old = state.loadNotes()
  await state.loadNotes()
  first.resolve({ notes: [note('old')] })
  await old
  assert.equal(state.notes.value[0].id, 'new')
})

test('edits while fetching the destination are flushed before selection changes', async (t) => {
  const pending = deferred()
  const { state, calls } = setup(t, { getNote: (id) => id === 'b' ? pending.promise : Promise.resolve(note(id)) })
  await state.initialize('a')
  const switching = state.selectNote(note('b'))
  await Promise.resolve()
  state.selected.value.title = 'typed during loading'
  pending.resolve(note('b'))
  await switching
  assert.equal(calls.at(-1)[1].title, 'typed during loading')
  assert.equal(state.selected.value.id, 'b')
})

test('successful archive removes the page only after the server accepts it', async (t) => {
  const pending = deferred()
  const { state } = setup(t, { updateNote: async (id, value) => {
    await pending.promise
    return { ...note(id), ...value }
  } })
  await state.initialize('a')
  const archiving = state.toggleArchive()
  await Promise.resolve()
  assert.equal(state.notes.value.length, 2)
  pending.resolve()
  assert.equal(await archiving, true)
  assert.equal(state.selected.value, null)
  assert.deepEqual(state.notes.value.map((page) => page.id), ['b'])
})

test('failed deletion keeps the active page visible', async (t) => {
  const { state } = setup(t, { deleteNote: async () => { throw new Error('offline') } })
  await state.initialize('a')
  assert.equal(await state.removeNote(), false)
  assert.equal(state.selected.value.id, 'a')
  assert.equal(state.notes.value.length, 2)
})

test('unsaved draft survives reload in the same tab and is cleared after saving', async (t) => {
  const entries = new Map()
  const storage = {
    getItem: (key) => entries.get(key),
    setItem: (key, value) => entries.set(key, value),
    removeItem: (key) => entries.delete(key),
  }
  const api = { listNotes: async () => ({ notes: [note('a')] }), getNote: async () => note('a'),
    updateNote: async (id, value) => ({ ...note(id), ...value }) }
  const options = { storage, recoveryKey: 'user-one', delay: 60_000 }
  const first = useWorkspaceNotes(api, options)
  await first.initialize('a')
  first.selected.value.title = 'recovered text'
  first.dispose()
  const second = useWorkspaceNotes(api, options)
  t.after(() => second.dispose())
  await second.initialize('a')
  assert.equal(second.selected.value.title, 'recovered text')
  assert.equal(second.saveState.value, 'Recovered draft')
  assert.equal(second.dirty.value, true)
  assert.equal(await second.flush(), true)
  assert.equal(entries.size, 0)
})

test('storage failure does not stop server saving', async (t) => {
  const state = useWorkspaceNotes({ getNote: async () => note('a'),
    updateNote: async (id, value) => ({ ...note(id), ...value }) }, {
    storage: { getItem() { throw new Error('disabled') }, setItem() { throw new Error('full') } },
    recoveryKey: 'user', delay: 60_000,
  })
  t.after(() => state.dispose())
  await state.selectNote(note('a'))
  state.selected.value.title = 'still save'
  assert.equal(await state.flush(), true)
  assert.equal(state.dirty.value, false)
})

test('two tabs cannot overwrite a newer save and conflict pauses retries and navigation', async (t) => {
  let server = note('a')
  let writes = 0
  const updateNote = async (id, value) => {
    writes++
    if (value.expected_updated_at !== server.updated_at) {
      throw Object.assign(new Error('note has changed'), { status: 409 })
    }
    server = { ...server, ...value, updated_at: '2026-01-02T00:00:00Z' }
    return server
  }
  const first = setup(t, { updateNote }).state
  const second = setup(t, { updateNote }).state
  await first.initialize('a')
  await second.initialize('a')
  first.selected.value.title = 'server winner'
  assert.equal(await first.flush(), true)
  second.selected.value.title = 'keep local'
  assert.equal(await second.flush(), false)
  assert.equal(second.conflict.value, true)
  assert.equal(second.saveState.value, 'Conflict')
  second.selected.value.title = 'still editable'
  assert.equal(await second.selectNote(note('b')), false)
  assert.equal(writes, 2)
  assert.equal(server.title, 'server winner')
  assert.equal(second.selected.value.title, 'still editable')
})

test('serialized saves advance the base even when edits arrive during the request', async (t) => {
  const pending = deferred()
  const bases = []
  const { state } = setup(t, { updateNote: async (id, value) => {
    bases.push(value.expected_updated_at)
    if (bases.length === 1) await pending.promise
    return { ...note(id), ...value, updated_at: `version-${bases.length}` }
  } })
  await state.initialize('a')
  state.selected.value.title = 'first'
  const saving = state.flush()
  state.selected.value.title = 'second'
  pending.resolve()
  assert.equal(await saving, true)
  assert.deepEqual(bases, ['2026-01-01', 'version-1'])
  assert.equal(state.selected.value.updated_at, 'version-2')
})

test('recovery retains the original base rather than adopting a newer server version', async (t) => {
  const draft = { ...note('a'), title: 'old local draft' }
  const storage = { getItem: () => JSON.stringify({ a: draft }), setItem() {}, removeItem() {} }
  let base
  const state = useWorkspaceNotes({ getNote: async () => ({ ...note('a'), updated_at: 'new-server-version' }),
    updateNote: async (id, value) => { base = value.expected_updated_at; throw Object.assign(new Error('stale'), { status: 409 }) },
  }, { storage, recoveryKey: 'user', delay: 60_000 })
  t.after(() => state.dispose())
  await state.selectNote(note('a'))
  assert.equal(await state.flush(), false)
  assert.equal(base, draft.updated_at)
  assert.equal(state.selected.value.title, draft.title)
})

test('loading server version clears conflict only after success and does not discard intervening edits', async (t) => {
  const pending = deferred()
  let reads = 0
  const { state } = setup(t, { getNote: () => ++reads === 1 ? Promise.resolve(note('a')) : pending.promise,
    updateNote: async () => { throw Object.assign(new Error('conflict'), { status: 409 }) } })
  await state.initialize('a')
  state.selected.value.title = 'local'
  await state.flush()
  const loading = state.reloadServerNote()
  state.selected.value.title = 'new local edit'
  pending.resolve({ ...note('a'), title: 'server', updated_at: 'new' })
  assert.equal(await loading, false)
  assert.equal(state.selected.value.title, 'new local edit')
  assert.equal(state.conflict.value, true)
  assert.equal(await state.reloadServerNote(), true)
  assert.equal(state.selected.value.title, 'server')
  assert.equal(state.conflict.value, false)
  assert.equal(state.dirty.value, false)
})

test('delete sends its base and preserves the note when another tab changed it', async (t) => {
  let base
  const { state } = setup(t, { deleteNote: async (id, version) => {
    base = version
    throw Object.assign(new Error('conflict'), { status: 409 })
  } })
  await state.initialize('a')
  assert.equal(await state.removeNote(), false)
  assert.equal(base, '2026-01-01')
  assert.equal(state.conflict.value, true)
  assert.equal(state.selected.value.id, 'a')
})