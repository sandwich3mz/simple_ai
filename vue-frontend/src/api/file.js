import client from './client'

export function uploadRagFile(file) {
  const formData = new FormData()
  formData.append('file', file)
  return client.post('/file/upload', formData)
}
