const tokenKey = 'handy_access_token'

export const auth = {
  token: () => localStorage.getItem(tokenKey),
  setToken: (token) => localStorage.setItem(tokenKey, token),
  logout: () => localStorage.removeItem(tokenKey),
  isAuthenticated() {
    const token = this.token()
    if (!token) return false
    try {
      const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
      if (payload.exp && payload.exp * 1000 <= Date.now()) {
        this.logout()
        return false
      }
      return true
    } catch {
      this.logout()
      return false
    }
  },
}

