// Same rule as api/rsvp.js. Quotes and backslashes are excluded so the address
// can go straight into an Airtable formula.
const EMAIL_PATTERN = /^[^\s@"'\\]+@[^\s@"'\\]+\.[^\s@"'\\]+$/

const scene = document.getElementById('scene')
const form = document.getElementById('rsvp')
const emailInput = document.getElementById('email')
const nameInput = document.getElementById('name')
const trapInput = document.getElementById('website')
const submitButton = form.querySelector('.submit')
const slot = document.querySelector('.slot')
const error = document.getElementById('error')
const status = document.getElementById('status')

let sending = false

form.addEventListener('submit', async (event) => {
  event.preventDefault()
  if (sending || scene.classList.contains('printed')) return

  // Bad input and in-flight requests deliberately show nothing.
  const email = emailInput.value.trim()
  const name = nameInput.value.trim()
  if (!EMAIL_PATTERN.test(email) || !name) return

  sending = true
  try {
    const response = await fetch('/api/rsvp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, name, website: trapInput.value }),
    })
    if (!response.ok) throw new Error(`RSVP failed with ${response.status}`)
  } catch (err) {
    console.error(err)
    error.hidden = false
    return
  } finally {
    sending = false
  }

  error.hidden = true
  emailInput.readOnly = true
  nameInput.readOnly = true
  submitButton.disabled = true
  scene.classList.add('printed')
  status.textContent = 'rsvp received'

  const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches
  slot.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'nearest' })
})
