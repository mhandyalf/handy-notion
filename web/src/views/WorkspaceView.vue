<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import BlockEditor from '../components/BlockEditor.vue'
import { useWorkspaceNotes } from '../composables/useWorkspaceNotes.js'
import { api } from '../lib/api.js'
import { auth } from '../lib/auth.js'
import '../workspace.css'

const route = useRoute()
const router = useRouter()
const sidebarOpen = ref(false)
const sidebar = ref(null)
const searchInput = ref(null)
const menuButton = ref(null)
const titleInput = ref(null)
const isMobile = ref(false)
const shortcutModifier = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl+'
let mobileQuery
let titleObserver
let titleWidth = 0
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
  groupedNotes, flush, loadNotes, toggleArchive, conflict } = workspace

function downloadDraft() {
  const url = URL.createObjectURL(new Blob([JSON.stringify(selected.value, null, 2)], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `note-${selected.value.id}-draft.json`
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function reloadServerNote() {
  if (window.confirm('Replace this local draft with the latest server version? Download your draft first if you want to keep it.')) {
    await workspace.reloadServerNote()
  }
}

async function createNote() {
  if (await workspace.createNote()) {
    sidebarOpen.value = false
    await nextTick()
    titleInput.value?.focus()
  }
}

function closeSidebar() {
  sidebarOpen.value = false
  menuButton.value?.focus()
}

function resizeTitle() {
  const input = titleInput.value
  if (!input) return
  input.style.height = 'auto'
  input.style.height = `${input.scrollHeight}px`
}

function updateMobile() {
  isMobile.value = mobileQuery.matches
  if (!isMobile.value) sidebarOpen.value = false
}

async function handleKeydown(event) {
  if (event.repeat || event.isComposing) return
  if (event.key === 'Escape' && sidebarOpen.value) {
    event.preventDefault()
    closeSidebar()
    return
  }
  if ((event.metaKey || event.ctrlKey) && !event.shiftKey) {
    if (event.key.toLowerCase() === 'k' && !event.altKey) {
      event.preventDefault()
      if (isMobile.value) sidebarOpen.value = true
      await nextTick()
      searchInput.value?.focus()
      searchInput.value?.select()
    } else if (event.code === 'KeyN' && event.altKey) {
      event.preventDefault()
      if (!busy.value) void createNote()
    }
  }
  if (event.key === 'Tab' && isMobile.value && sidebarOpen.value) {
    const controls = [...sidebar.value.querySelectorAll('a[href], button:not(:disabled), input')]
      .filter((element) => element.getClientRects().length)
    const first = controls[0]
    const last = controls.at(-1)
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last?.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first?.focus()
    }
  }
}

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
watch(() => [selected.value?.id, selected.value?.title], resizeTitle, { flush: 'post' })
watch(titleInput, (input) => {
  titleObserver?.disconnect()
  titleWidth = 0
  if (input) titleObserver?.observe(input)
  resizeTitle()
}, { flush: 'post' })
watch(sidebarOpen, (open) => {
  if (open && isMobile.value) {
    sidebar.value?.querySelector('button[aria-label="Close notes sidebar"]')?.focus()
  }
}, { flush: 'post' })
onMounted(() => {
  mobileQuery = window.matchMedia('(max-width: 760px)')
  updateMobile()
  mobileQuery.addEventListener('change', updateMobile)
  if (typeof ResizeObserver !== 'undefined') {
    titleObserver = new ResizeObserver(([entry]) => {
      if (entry.contentRect.width !== titleWidth) {
        titleWidth = entry.contentRect.width
        resizeTitle()
      }
    })
    if (titleInput.value) titleObserver.observe(titleInput.value)
  }
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', resizeTitle)
  window.addEventListener('beforeunload', beforeUnload)
  void workspace.initialize(route.params.id)
})
onBeforeUnmount(() => {
  clearTimeout(searchTimer)
  mobileQuery?.removeEventListener('change', updateMobile)
  titleObserver?.disconnect()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', resizeTitle)
  window.removeEventListener('beforeunload', beforeUnload)
  workspace.dispose()
})
</script>

<template>
  <main class="workspace" aria-label="Notes">
    <button v-if="sidebarOpen && isMobile" class="sidebar-backdrop" aria-label="Close notes sidebar" tabindex="-1" @click="closeSidebar"></button>
    <aside id="notes-sidebar" ref="sidebar" class="sidebar" :class="{ open: sidebarOpen }" aria-label="Notes sidebar" :inert="isMobile && !sidebarOpen">
      <div class="sidebar-top">
        <a class="brand compact" href="/notes" @click.prevent="selectNote(null)"><span class="brand-mark">h</span> handy notes</a>
        <button class="icon-button mobile-only" aria-label="Close notes sidebar" @click="closeSidebar">×</button>
      </div>
      <p class="sidebar-caption">Your notes, all in one place</p>
      <button class="new-note" :disabled="busy" aria-keyshortcuts="Meta+Alt+N Control+Alt+N" @click="createNote"><span aria-hidden="true">＋</span> New note <kbd>{{ shortcutModifier }}Alt+N</kbd></button>
      <label class="search-box"><span aria-hidden="true">⌕</span><input ref="searchInput" v-model="search" type="search" aria-label="Search notes" aria-keyshortcuts="Meta+K Control+K" placeholder="Search notes…" /><kbd aria-hidden="true">{{ shortcutModifier }}K</kbd></label>
      <nav class="note-nav" :aria-label="archived ? 'Archived notes' : 'Note list'" :aria-busy="loading">
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
        <div v-else-if="search.trim()" class="empty-nav" role="status"><strong>No matching notes</strong><p>Nothing found for “{{ search }}”{{ archived ? ' in the archive' : '' }}.</p><button class="clear-search" @click="search = ''; searchInput?.focus()">Clear search</button></div>
        <p v-else class="empty-nav">{{ archived ? 'No archived notes.' : 'No notes here yet. Create your first note above.' }}</p>
      </nav>
      <div class="sidebar-bottom">
        <button :class="{ active: archived }" @click="archived = !archived"><span>▱</span>{{ archived ? 'Back to notes' : 'Archive' }}</button>
        <button :disabled="busy" @click="logout"><span>↗</span>Log out</button>
      </div>
    </aside>

    <section class="editor-shell" aria-label="Note editor" :inert="isMobile && sidebarOpen">
      <header class="editor-toolbar" aria-label="Note controls">
        <button ref="menuButton" class="icon-button mobile-only" aria-label="Open notes sidebar" aria-controls="notes-sidebar" :aria-expanded="sidebarOpen" @click="sidebarOpen = true">☰</button>
        <div class="breadcrumbs"><span>{{ selected?.is_archived ? 'Archive' : 'Notes' }}</span><b aria-hidden="true">/</b><strong>{{ selected ? selected.title || 'Untitled' : 'No note selected' }}</strong></div>
        <div v-if="selected" class="toolbar-actions">
          <span class="save-state" role="status" aria-live="polite" aria-atomic="true"><i aria-hidden="true" :class="{ saving: saveState !== 'Saved' }"></i>{{ saveState }}</span>
          <button v-if="saveState === 'Save failed' || saveState === 'Recovered draft'" class="text-button" @click="flush">Retry save</button>
          <button class="icon-button" :class="{ starred: selected.is_favorite }" :aria-label="selected.is_favorite ? 'Remove from favorites' : 'Add to favorites'" :aria-pressed="selected.is_favorite" title="Favorite" @click="selected.is_favorite = !selected.is_favorite">☆</button>
          <button class="text-button" :disabled="busy" @click="toggleArchive">{{ selected.is_archived ? 'Restore' : 'Archive' }}</button>
          <button v-if="selected.is_archived" class="icon-button danger" aria-label="Delete note permanently" title="Delete permanently" :disabled="busy" @click="removeNote">⌫</button>
        </div>
      </header>

      <div v-if="error" class="app-error" role="alert">{{ error }} <button class="icon-button" aria-label="Dismiss error" @click="error = ''">×</button></div>
      <div v-if="conflict" class="app-error" role="alert">
        <div><strong>This note changed elsewhere.</strong> Automatic saving is paused. Your local draft is still here; download it before loading the server version. No changes will be overwritten automatically.</div>
        <button class="text-button" @click="downloadDraft">Download local draft</button>
        <button class="text-button" :disabled="busy" @click="reloadServerNote">Load server version</button>
      </div>
      <article v-if="selected" :key="selected.id" class="document">
        <button class="page-icon" aria-label="Change note icon" title="Change icon" @click="selected.icon = selected.icon === '📝' ? '💡' : selected.icon === '💡' ? '📌' : '📝'">{{ selected.icon }}</button>
        <textarea ref="titleInput" v-model="selected.title" class="title-input" aria-label="Note title" rows="1" placeholder="Untitled" @input="resizeTitle" />
        <div class="document-meta"><span>Last edited {{ new Date(selected.updated_at).toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' }) }}</span><span>•</span><span>{{ selected.content.length }} {{ selected.content.length === 1 ? 'block' : 'blocks' }}</span></div>
        <BlockEditor :key="selected.id" v-model:blocks="selected.content" />
      </article>
      <div v-else-if="!loading" class="empty-document"><span>✦</span><h2>A space for your next idea</h2><p>Create a new note and start with one small sentence.</p><button class="primary-button" :disabled="busy" @click="createNote">Create note</button></div>
    </section>
  </main>
</template>
