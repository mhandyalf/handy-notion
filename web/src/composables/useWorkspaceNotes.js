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
    saveState.value = 'Saved'
    hydrating = false
    const recovered = note && recoveryDrafts()?.[note.id]
    if (recovered && recovered.id === note.id && Array.isArray(recovered.content)) {
      hydrating = true
      selected.value = { ...selected.value, ...payload(recovered) }
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
    saveState.value = 'Waiting…'
    clearTimeout(timer)
    timer = setTimeout(() => { void flush() }, delay)
  }, { deep: true, flush: 'sync' })

  async function drain() {
    while (dirty.value && selected.value) {
      const id = selected.value.id
      const version = revision
      const snapshot = payload(selected.value)
      saveState.value = 'Saving…'
      try {
        const saved = await api.updateNote(id, snapshot)
        if (disposed) return true
        const index = notes.value.findIndex((note) => note.id === id)
        if (index >= 0) notes.value[index] = clone(saved)
        if (selected.value?.id === id && revision === version) {
          hydrating = true
          selected.value.updated_at = saved.updated_at
          hydrating = false
          dirty.value = false
          persistDraft({ id }, true)
          saveState.value = 'Saved'
        }
      } catch (err) {
        error.value = err.message
        saveState.value = 'Save failed'
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
      await api.deleteNote(id)
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

  return { notes, selected, search, archived, loading, busy, error, saveState, dirty,
    groupedNotes, flush, loadNotes, initialize, selectNote, createNote, toggleArchive, removeNote, dispose }
}