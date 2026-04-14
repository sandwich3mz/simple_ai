import axios from 'axios'

const SUCCESS_CODE = 1000
const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1'

let authToken = localStorage.getItem('simple_ai_token') || ''

export function setAuthToken(token) {
  authToken = token || ''
}

function buildApiError(statusCode, statusMsg) {
  const error = new Error(statusMsg || 'Request failed')
  error.statusCode = statusCode
  return error
}

export function normalizeError(error) {
  if (error?.response?.data?.status_msg) {
    return error.response.data.status_msg
  }
  if (error?.message) {
    return error.message
  }
  return 'Request failed'
}

const client = axios.create({
  baseURL: API_BASE,
  timeout: 30000,
})

client.interceptors.request.use((config) => {
  if (authToken) {
    config.headers.Authorization = `Bearer ${authToken}`
  }
  return config
})

client.interceptors.response.use(
  (response) => {
    const data = response?.data
    if (typeof data?.status_code === 'number' && data.status_code !== SUCCESS_CODE) {
      throw buildApiError(data.status_code, data.status_msg)
    }
    return data
  },
  (error) => Promise.reject(error),
)

export default client
