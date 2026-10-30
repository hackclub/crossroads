const TABLE_URL = 'https://api.airtable.com/v0/appuxTrbdZ2tjCo0X/tblNVL1eN65Pqb3TI' // YSWS - Crossroads / Event RSVP

// Same rule as script.js. Quotes and backslashes are excluded so the address
// can go straight into an Airtable formula.
const EMAIL_PATTERN = /^[^\s@"'\\]+@[^\s@"'\\]+\.[^\s@"'\\]+$/

export async function POST(request) {
  let body
  try {
    body = await request.json()
  } catch {
    return new Response(null, { status: 400 })
  }

  // Bots fill in the hidden "website" field; pretend it worked so they move on.
  if (body.website) return new Response(null, { status: 204 })

  const email = String(body.email ?? '').trim().toLowerCase()
  const name = String(body.name ?? '').trim()
  if (!EMAIL_PATTERN.test(email) || email.length > 254 || !name || name.length > 200) {
    return new Response(null, { status: 400 })
  }

  try {
    // Airtable can't upsert on an email-type field, so find an existing RSVP first.
    const query = new URLSearchParams({ filterByFormula: `LOWER({Email}) = '${email}'`, maxRecords: '1' })
    const { records } = await airtable(`?${query}`)

    if (records.length > 0) {
      await airtable(`/${records[0].id}`, { method: 'PATCH', body: { fields: { Name: name } } })
    } else {
      await airtable('', { method: 'POST', body: { fields: { Email: email, Name: name } } })
    }
  } catch (err) {
    console.error(err)
    return new Response(null, { status: 502 })
  }

  return new Response(null, { status: 204 })
}

async function airtable(path, { method = 'GET', body } = {}) {
  const response = await fetch(TABLE_URL + path, {
    method,
    headers: {
      Authorization: `Bearer ${process.env.AIRTABLE_TOKEN}`,
      'Content-Type': 'application/json',
    },
    body: body && JSON.stringify(body),
  })
  if (!response.ok) {
    throw new Error(`Airtable ${method} ${response.status}: ${await response.text()}`)
  }
  return response.json()
}
