import { computed, ref, watch } from 'vue'

const clone = (value) => JSON.parse(JSON.stringify(value))
const payload = (note) => Object.fromEntries(
  ['title', 'content', 'icon', 'is_favorite', 'is_archived'].map((key) => [key, clone(note[key])]),
)

export function useWorkspaceNotes(api, { navigate = () => {}, delay = 650, storage, recoveryKey } = {}) {
  const notes = ref([])
  const selected = ref(null)
  const search = ref('')
  const archived = ref(false)
  const loading = ref(false)
  const busy = ref(false)
  const error = ref('')
  const saveState = ref('Saved')
  const dirty = ref(false)
  const conflict = ref(false)
  let hydrating = false
  let revision = 0
  let timer
  let saving
  let listRequest = 0
  let disposed = false

  function recoveryDrafts() {
    try { return JSON.parse(storage?.getItem(recoveryKey) || '{}') } catch { return {} }
  }

  function persistDraft(note, remove = false) {
    if (!storage || !recoveryKey) return
    try {
      const drafts = recoveryDrafts()
      if (remove) delete drafts[note.id]
      else drafts[note.id] = clone(note)
      if (Object.keys(drafts).length) storage.setItem(recoveryKey, JSON.stringify(drafts))
      else storage.removeItem(recoveryKey)
    } catch {
      error.value = 'Local draft recovery is unavailable. Keep this page open until saved.'
    }
  }

  const groupedNotes = computed(() => ({
    favorites: notes.value.filter((note) => note.is_favorite),
    recent: notes.value.filter((note) => !note.is_favorite),
  }))

  function applySelected(note) {
    hydrating = true
    selected.value = note ? clone(note) : null
    if (selected.value && (!Array.isArray(selected.value.content) || !selected.value.content.length)) {
      selected.value.content = [{ id: crypto.randomUUID(), type: 'text', text: '' }]
    }
    dirty.value = false
    conflict.value = false
    saveState.value = 'Saved'
    hydrating = false
    const recovered = note && recoveryDrafts()?.[note.id]
    if (recovered && recovered.id === note.id && Array.isArray(recovered.content)) {
      hydrating = true
      selected.value = { ...selected.value, ...payload(recovered) }
      // A recovered draft must retain its original base, not silently adopt the server's version.
      selected.value.updated_at = recovered.updated_at || null
      hydrating = false
      revision++
      dirty.value = true
      saveState.value = 'Recovered draft'
      error.value = 'An unsaved draft was recovered in this tab. Review it and choose Retry save to save it.'
    }
  }

  const stopWatch = watch(selected, () => {
    if (hydrating || !selected.value) return
    revision++
    dirty.value = true
    persistDraft(selected.value)
    clearTimeout(timer)
    saveState.value = conflict.value ? 'Conflict' : 'Waiting…'
    if (!conflict.value) timer = setTimeout(() => { void flush() }, delay)
  }, { deep: true, flush: 'sync' })

  async function drain() {
    if (conflict.value) return false
    if (dirty.value && !selected.value?.updated_at) {
      conflict.value = true
      saveState.value = 'Conflict'
      error.value = 'This draft has no base version. Download it before loading the server version.'
      return false
    }
    while (dirty.value && selected.value) {
      const id = selected.value.id
      const version = revision
      const snapshot = payload(selected.value)
      snapshot.expected_updated_at = selected.value.updated_at
      saveState.value = 'Saving…'
      try {
        const saved = await api.updateNote(id, snapshot)
        if (disposed) return true
        const index = notes.value.findIndex((note) => note.id === id)
        if (index >= 0) notes.value[index] = clone(saved)
        if (selected.value?.id === id) {
          hydrating = true
          selected.value.updated_at = saved.updated_at
          hydrating = false
          if (revision !== version) {
            persistDraft(selected.value)
            continue
          }
          dirty.value = false
          persistDraft({ id }, true)
          saveState.value = 'Saved'
        }
      } catch (err) {
        error.value = err.message
        conflict.value = err.status === 409 || err.status === 428
        saveState.value = conflict.value ? 'Conflict' : 'Save failed'
        return false
      }
    }
    return true
  }

  function flush() {
    clearTimeout(timer)
    if (!saving) saving = drain().finally(() => { saving = null })
    return saving
  }

  async function loadNotes() {
    const request = ++listRequest
    loading.value = true
    try {
      const data = await api.listNotes(archived.value, search.value)
      if (!disposed && request === listRequest) notes.value = data.notes || []
    } catch (err) {
      if (!disposed && request === listRequest) error.value = err.message
    } finally {
      if (!disposed && request === listRequest) loading.value = false
    }
  }

  async function transition(action) {
    if (busy.value) return false
    busy.value = true
    error.value = ''
    try {
      if (!await flush()) return false
      return await action()
    } catch (err) {
      error.value = err.message
      return false
    } finally { busy.value = false }
  }

  function selectNote(note, shouldNavigate = true) {
    return transition(async () => {
      const fullNote = note ? await api.getNote(note.id) : null
      // The user can keep typing while the destination is loading.
      if (!await flush() || disposed) return false
      applySelected(fullNote)
      if (shouldNavigate) await navigate(fullNote ? `/notes/${fullNote.id}` : '/notes')
      return true
    })
  }

  async function initialize(id) {
    await loadNotes()
    if (!disposed) await selectNote(id ? { id } : notes.value[0] || null, false)
  }

  function createNote() {
    return transition(async () => {
      const note = await api.createNote()
      if (!await flush() || disposed) return false
      notes.value.unshift(note)
      applySelected(note)
      await navigate(`/notes/${note.id}`)
      return true
    })
  }

  function toggleArchive() {
    return transition(async () => {
      if (!selected.value) return false
      const id = selected.value.id
      const previous = selected.value.is_archived
      selected.value.is_archived = !previous
      if (!await flush()) {
        hydrating = true
        selected.value.is_archived = previous
        hydrating = false
        persistDraft(selected.value)
        return false
      }
      notes.value = notes.value.filter((note) => note.id !== id)
      applySelected(null)
      await navigate('/notes')
      return true
    })
  }

  function removeNote() {
    return transition(async () => {
      if (!selected.value) return false
      const id = selected.value.id
      try {
        await api.deleteNote(id, selected.value.updated_at)
      } catch (err) {
        if (err.status === 409 || err.status === 428) {
          conflict.value = true
          saveState.value = 'Conflict'
          persistDraft(selected.value)
        }
        throw err
      }
      clearTimeout(timer)
      persistDraft({ id }, true)
      notes.value = notes.value.filter((note) => note.id !== id)
      applySelected(null)
      await navigate('/notes')
      return true
    })
  }

  function dispose() {
    disposed = true
    clearTimeout(timer)
    stopWatch()
  }

  async function reloadServerNote() {
    if (busy.value || saving || !selected.value) return false
    busy.value = true
    clearTimeout(timer)
    try {
      const version = revision
      const fresh = await api.getNote(selected.value.id)
      if (revision !== version) {
        error.value = 'The local draft changed while loading. Download it and try loading again.'
        return false
      }
      persistDraft({ id: fresh.id }, true)
      applySelected(fresh)
      error.value = ''
      return true
    } catch (err) {
      error.value = err.message
      return false
    } finally { busy.value = false }
  }

  return { notes, selected, search, archived, loading, busy, error, saveState, dirty, conflict,
    groupedNotes, flush, loadNotes, initialize, selectNote, createNote, toggleArchive, removeNote, reloadServerNote, dispose }
}