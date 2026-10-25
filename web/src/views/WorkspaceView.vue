<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import BlockEditor from '../components/BlockEditor.vue'
import { useWorkspaceNotes } from '../composables/useWorkspaceNotes.js'
import { api } from '../lib/api.js'
import { auth } from '../lib/auth.js'

const route = useRoute()
const router = useRouter()
const sidebarOpen = ref(false)
let searchTimer
let recoveryStorage
try { recoveryStorage = window.sessionStorage } catch { /* Browser storage may be disabled. */ }
const userID = auth.userID()
const workspace = useWorkspaceNotes(api, {
  navigate: (path) => router.replace(path),
  storage: recoveryStorage,
  recoveryKey: userID ? `handy-notion:drafts:${userID}` : null,
})
const { notes, selected, search, archived, loading, busy, error, saveState, dirty,
  groupedNotes, flush, loadNotes, createNote, toggleArchive } = workspace

async function selectNote(note) {
  if (await workspace.selectNote(note)) sidebarOpen.value = false
}

async function removeNote() {
  if (selected.value && window.confirm(`Delete “${selected.value.title || 'Untitled'}”?`)) {
    await workspace.removeNote()
  }
}

async function logout() {
  if (busy.value) return
  if (!await flush()) return
  auth.logout()
  await router.push('/login')
}

function beforeUnload(event) {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

onBeforeRouteLeave(() => busy.value ? false : flush())
onBeforeRouteUpdate(async (to) => {
  if (to.params.id === selected.value?.id || (!to.params.id && !selected.value)) return true
  return workspace.selectNote(to.params.id ? { id: to.params.id } : null, false)
})
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(loadNotes, 300)
})
watch(archived, () => { void loadNotes() })
onMounted(() => {
  window.addEventListener('beforeunload', beforeUnload)
  void workspace.initialize(route.params.id)
})
onBeforeUnmount(() => {
  clearTimeout(searchTimer)
  window.removeEventListener('beforeunload', beforeUnload)
  workspace.dispose()
})
</script>

<template>
  <main class="workspace">
    <aside class="sidebar" :class="{ open: sidebarOpen }">
      <div class="sidebar-top">
        <a class="brand compact" href="/notes" @click.prevent="selectNote(null)"><span class="brand-mark">h</span> handy notes</a>
        <button class="icon-button mobile-only" @click="sidebarOpen = false">×</button>
      </div>
      <button class="new-note" :disabled="busy" @click="createNote"><span>＋</span> New note <kbd>⌘ N</kbd></button>
      <label class="search-box"><span>⌕</span><input v-model="search" placeholder="Search notes…" /></label>
      <nav class="note-nav">
        <template v-if="!loading && notes.length">
          <section v-if="groupedNotes.favorites.length">
            <p class="nav-label">Favorites</p>
            <button v-for="note in groupedNotes.favorites" :key="note.id" :class="{ active: selected?.id === note.id }" :disabled="busy" @click="selectNote(note)"><span>{{ note.icon }}</span><span class="note-link-copy"><strong>{{ note.title || 'Untitled' }}</strong><small>{{ new Date(note.updated_at).toLocaleDateString('en-US', { day: 'numeric', month: 'short' }) }}</small></span></button>
          </section>
          <section>
            <p class="nav-label">{{ archived ? 'Archive' : 'Recent' }}</p>
            <button v-for="note in groupedNotes.recent" :key="note.id" :class="{ active: selected?.id === note.id }" :disabled="busy" @click="selectNote(note)"><span>{{ note.icon }}</span><span class="note-link-copy"><strong>{{ note.title || 'Untitled' }}</strong><small>{{ new Date(note.updated_at).toLocaleDateString('en-US', { day: 'numeric', month: 'short' }) }}</small></span></button>
          </section>
        </template>
        <p v-else-if="loading" class="empty-nav">Loading notes…</p>
        <p v-else class="empty-nav">No notes here yet.</p>
      </nav>
      <div class="sidebar-bottom">
        <button :class="{ active: archived }" @click="archived = !archived"><span>▱</span>{{ archived ? 'Back to notes' : 'Archive' }}</button>
        <button :disabled="busy" @click="logout"><span>↗</span>Log out</button>
      </div>
    </aside>

    <section class="editor-shell">
      <header class="editor-toolbar">
        <button class="icon-button mobile-only" @click="sidebarOpen = true">☰</button>
        <div class="breadcrumbs"><span>Notes</span><b>/</b><strong>{{ selected?.title || 'Empty' }}</strong></div>
        <div v-if="selected" class="toolbar-actions">
          <span class="save-state"><i :class="{ saving: saveState !== 'Saved' }"></i>{{ saveState }}</span>
          <button v-if="saveState === 'Save failed' || saveState === 'Recovered draft'" class="text-button" @click="flush">Retry save</button>
          <button class="icon-button" :class="{ starred: selected.is_favorite }" title="Favorite" @click="selected.is_favorite = !selected.is_favorite">☆</button>
          <button class="text-button" :disabled="busy" @click="toggleArchive">{{ selected.is_archived ? 'Restore' : 'Archive' }}</button>
          <button v-if="selected.is_archived" class="icon-button danger" title="Delete permanently" :disabled="busy" @click="removeNote">⌫</button>
        </div>
      </header>

      <p v-if="error" class="app-error" @click="error = ''">{{ error }} <span>×</span></p>
      <article v-if="selected" :key="selected.id" class="document">
        <button class="page-icon" title="Change icon" @click="selected.icon = selected.icon === '📝' ? '💡' : selected.icon === '💡' ? '📌' : '📝'">{{ selected.icon }}</button>
        <textarea v-model="selected.title" class="title-input" rows="1" placeholder="Untitled" @input="$event.target.style.height = 'auto'; $event.target.style.height = `${$event.target.scrollHeight}px`" />
        <div class="document-meta"><span>Last edited {{ new Date(selected.updated_at).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' }) }}</span><span>•</span><span>{{ selected.content.length }} {{ selected.content.length === 1 ? 'block' : 'blocks' }}</span></div>
        <BlockEditor :key="selected.id" v-model:blocks="selected.content" />
      </article>
      <div v-else-if="!loading" class="empty-document"><span>✦</span><h2>A space for your next idea</h2><p>Create a new note and start with one small sentence.</p><button class="primary-button" :disabled="busy" @click="createNote">Create note</button></div>
    </section>
  </main>
</template>
