import axios from 'axios'

const client = axios.create({
  // Empty baseURL = relative to current host.
  // Vite dev server proxies /api → Go backend, so this works from
  // any device on the LAN (phone, tablet) without hardcoding an IP.
  baseURL: '',
  timeout: 60_000,
})

export default client