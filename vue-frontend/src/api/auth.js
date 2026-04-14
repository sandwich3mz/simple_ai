import client from './client'

export function sendCaptcha(email) {
  return client.post('/user/captcha', { email })
}

export function login(username, password) {
  return client.post('/user/login', { username, password })
}

export function register(email, password, captcha) {
  return client.post('/user/register', { email, password, captcha })
}
