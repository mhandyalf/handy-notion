<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BlockEditor from '../components/BlockEditor.vue'
import { api } from '../lib/api.js'
import { auth } from '../lib/auth.js'

const route = useRoute()
const router = useRouter()
const notes = ref([])
const selected = ref(null)
const search = ref('')
const archived = ref(false)
const loading = ref(true)
const error = ref('')
const saveState = ref('Tersimpan')
const sidebarOpen = ref(false)
let saveTimer
let searchTimer
let hydrating = true
let selectionRequest = 0

const groupedNotes = computed(() => ({
  favorites: notes.value.filter((note) => note.is_favorite),
  recent: notes.value.filter((note) => !note.is_favorite),
}))

async function loadNotes(preferredId = route.params.id) {
  loading.value = true
  error.value = ''
  try {
    const data = await api.listNotes(archived.value, search.value)
    notes.value = data.notes || []
    const match = notes.value.find((note) => note.id === preferredId)
    applySelected(match || notes.value[0] || null, false)
  } catch (err) { error.value = err.message }
  finally { loading.value = false }
}

function applySelected(note, navigate = true) {
  hydrating = true
  selected.value = note ? structuredClone(note) : null
  if (selected.value && (!Array.isArray(selected.value.content) || !selected.value.content.length)) {
    selected.value.content = [{ id: crypto.randomUUID(), type: 'text', text: '' }]
  }
  if (navigate && note) router.replace(`/notes/${note.id}`)
  sidebarOpen.value = false
  setTimeout(() => { hydrating = false }, 0)
}

async function selectNote(note, navigate = true) {
  if (!note) {
    applySelected(null, navigate)
    return
  }

  clearTimeout(saveTimer)
  const requestID = ++selectionRequest
  saveState.value = 'Memuat…'
  try {
    const fullNote = await api.getNote(note.id)
    if (requestID !== selectionRequest) return
    applySelected(fullNote, navigate)
    saveState.value = 'Tersimpan'
  } catch (err) {
    if (requestID === selectionRequest) error.value = err.message
  }
}

async function createNote() {
  try {
    const note = await api.createNote()
    notes.value.unshift(note)
    applySelected(note)
  } catch (err) { error.value = err.message }
}

function scheduleSave() {
  if (hydrating || !selected.value) return
  saveState.value = 'Menunggu…'
  clearTimeout(saveTimer)
  saveTimer = setTimeout(saveNote, 650)
}

async function saveNote() {
  if (!selected.value) return
  saveState.value = 'Menyimpan…'
  try {
    const saved = await api.updateNote(selected.value.id, {
      title: selected.value.title,
      content: selected.value.content,
      icon: selected.value.icon,
      is_favorite: selected.value.is_favorite,
      is_archived: selected.value.is_archived,
    })
    const index = notes.value.findIndex((note) => note.id === saved.id)
    if (index >= 0) notes.value[index] = structuredClone(saved)
    saveState.value = 'Tersimpan'
  } catch (err) { saveState.value = 'Gagal tersimpan'; error.value = err.message }
}

async function toggleArchive() {
  if (!selected.value) return
  selected.value.is_archived = !selected.value.is_archived
  await saveNote()
  notes.value = notes.value.filter((note) => note.id !== selected.value.id)
  applySelected(notes.value[0] || null)
}

async function removeNote() {
  if (!selected.value || !window.confirm(`Hapus permanen “${selected.value.title || 'Untitled'}”?`)) return
  try {
    await api.deleteNote(selected.value.id)
    notes.value = notes.value.filter((note) => note.id !== selected.value.id)
    applySelected(notes.value[0] || null)
  } catch (err) { error.value = err.message }
}

function logout() { auth.logout(); router.push('/login') }

watch(selected, scheduleSave, { deep: true })
watch(search, () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => loadNotes(), 300) })
watch(archived, () => loadNotes())
onMounted(loadNotes)
onBeforeUnmount(() => { clearTimeout(saveTimer); clearTimeout(searchTimer) })
</script>

<template>
  <main class="workspace">
    <aside class="sidebar" :class="{ open: sidebarOpen }">
      <div class="sidebar-top">
        <a class="brand compact" href="/notes"><span class="brand-mark">h</span> handy notes</a>
        <button class="icon-button mobile-only" @click="sidebarOpen = false">×</button>
      </div>
      <button class="new-note" @click="createNote"><span>＋</span> Catatan baru <kbd>⌘ N</kbd></button>
      <label class="search-box"><span>⌕</span><input v-model="search" placeholder="Cari catatan…" /></label>
      <nav class="note-nav">
        <template v-if="!loading && notes.length">
          <section v-if="groupedNotes.favorites.length">
            <p class="nav-label">Favorit</p>
            <button v-for="note in groupedNotes.favorites" :key="note.id" :class="{ active: selected?.id === note.id }" @click="selectNote(note)"><span>{{ note.icon }}</span><span class="note-link-copy"><strong>{{ note.title || 'Untitled' }}</strong><small>{{ new Date(note.updated_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' }) }}</small></span></button>
          </section>
          <section>
            <p class="nav-label">{{ archived ? 'Arsip' : 'Terbaru' }}</p>
            <button v-for="note in groupedNotes.recent" :key="note.id" :class="{ active: selected?.id === note.id }" @click="selectNote(note)"><span>{{ note.icon }}</span><span class="note-link-copy"><strong>{{ note.title || 'Untitled' }}</strong><small>{{ new Date(note.updated_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' }) }}</small></span></button>
          </section>
        </template>
        <p v-else-if="loading" class="empty-nav">Memuat catatan…</p>
        <p v-else class="empty-nav">Belum ada catatan di sini.</p>
      </nav>
      <div class="sidebar-bottom">
        <button :class="{ active: archived }" @click="archived = !archived"><span>▱</span>{{ archived ? 'Kembali ke catatan' : 'Arsip' }}</button>
        <button @click="logout"><span>↗</span>Keluar</button>
      </div>
    </aside>

    <section class="editor-shell">
      <header class="editor-toolbar">
        <button class="icon-button mobile-only" @click="sidebarOpen = true">☰</button>
        <div class="breadcrumbs"><span>Catatan</span><b>/</b><strong>{{ selected?.title || 'Kosong' }}</strong></div>
        <div v-if="selected" class="toolbar-actions">
          <span class="save-state"><i :class="{ saving: saveState !== 'Tersimpan' }"></i>{{ saveState }}</span>
          <button class="icon-button" :class="{ starred: selected.is_favorite }" title="Favorit" @click="selected.is_favorite = !selected.is_favorite">☆</button>
          <button class="text-button" @click="toggleArchive">{{ selected.is_archived ? 'Pulihkan' : 'Arsipkan' }}</button>
          <button v-if="selected.is_archived" class="icon-button danger" title="Hapus permanen" @click="removeNote">⌫</button>
        </div>
      </header>

      <p v-if="error" class="app-error" @click="error = ''">{{ error }} <span>×</span></p>
      <article v-if="selected" :key="selected.id" class="document">
        <button class="page-icon" title="Ganti ikon" @click="selected.icon = selected.icon === '📝' ? '💡' : selected.icon === '💡' ? '📌' : '📝'">{{ selected.icon }}</button>
        <textarea v-model="selected.title" class="title-input" rows="1" placeholder="Untitled" @input="$event.target.style.height = 'auto'; $event.target.style.height = `${$event.target.scrollHeight}px`" />
        <div class="document-meta"><span>Terakhir diubah {{ new Date(selected.updated_at).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) }}</span><span>•</span><span>{{ selected.content.length }} blok</span></div>
        <BlockEditor :key="selected.id" v-model:blocks="selected.content" />
      </article>
      <div v-else-if="!loading" class="empty-document"><span>✦</span><h2>Ruang untuk ide berikutnya</h2><p>Buat catatan baru dan mulai dari satu kalimat kecil.</p><button class="primary-button" @click="createNote">Buat catatan</button></div>
    </section>
  </main>
</template>
