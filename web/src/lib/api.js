import { auth } from './auth.js'

const notesBase = import.meta.env.VITE_API_URL || 'http://localhost:8082'
const authBase = import.meta.env.VITE_AUTH_URL || 'http://localhost:8081'

async function request(base, path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  const token = auth.token()
  if (token) headers.Authorization = `Bearer ${token}`
  const response = await fetch(`${base}${path}`, { ...options, headers })
  if (response.status === 401 && base === notesBase) {
    auth.logout()
    window.location.assign('/login')
    throw new Error('Sesi kamu sudah berakhir')
  }
  if (response.status === 204) return null
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || 'Terjadi kesalahan. Coba lagi ya.')
  return data
}

export const api = {
  login: (payload) => request(authBase, '/api/login', { method: 'POST', body: JSON.stringify(payload) }),
  register: (payload) => request(authBase, '/api/register', { method: 'POST', body: JSON.stringify(payload) }),
  listNotes: (archived = false, query = '') => request(notesBase, `/api/notes?archived=${archived}&q=${encodeURIComponent(query)}`),
  getNote: (id) => request(notesBase, `/api/notes/${id}`),
  createNote: () => request(notesBase, '/api/notes', { method: 'POST', body: JSON.stringify({ content: [{ id: crypto.randomUUID(), type: 'text', text: '' }] }) }),
  updateNote: (id, payload) => request(notesBase, `/api/notes/${id}`, { method: 'PUT', body: JSON.stringify(payload) }),
  deleteNote: (id) => request(notesBase, `/api/notes/${id}`, { method: 'DELETE' }),
}
