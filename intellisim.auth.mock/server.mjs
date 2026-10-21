import { readFileSync } from 'node:fs'
import express from 'express'
import cors from 'cors'
import { OAuth2Server } from 'oauth2-mock-server'

const PORT = Number(process.env.PORT || 9090)
const TTL = Number(process.env.MOCK_TOKEN_TTL || 3600)

if (!Number.isInteger(PORT) || PORT < 1 || PORT > 65535) {
  console.error(`Invalid PORT="${process.env.PORT}" — must be an integer 1-65535`)
  process.exit(1)
}
if (!Number.isInteger(TTL) || TTL < 1) {
  console.error(`Invalid MOCK_TOKEN_TTL="${process.env.MOCK_TOKEN_TTL}" — must be a positive integer (seconds)`)
  process.exit(1)
}

const ACTIVE = process.env.MOCK_ACTIVE_USER || 'dev-user'

const users = JSON.parse(readFileSync(new URL('./users.json', import.meta.url), 'utf-8'))
const user = users[ACTIVE]
if (!user) {
  console.error(`MOCK_ACTIVE_USER="${ACTIVE}" not found in users.json (have: ${Object.keys(users).join(', ')})`)
  process.exit(1)
}

const server = new OAuth2Server()
await server.issuer.keys.generate('RS256')
// Issuer must match what core/front are configured with (AUTH_SERVER / VITE_AUTH_AUTHORITY).
server.issuer.url = `http://localhost:${PORT}`

// Stamp the active user's identity claims into every signed token (access + id).
server.service.on('beforeTokenSigning', (token) => {
  token.payload.sub = user.sub
  token.payload.preferred_username = user.preferred_username
  token.payload.name = user.name
  token.payload.email = user.email
  token.payload.permissions = user.permissions
  token.payload.exp = Math.floor(Date.now() / 1000) + TTL
})

// Keep the token RESPONSE's expires_in in sync with the JWT exp claim —
// oidc-client-ts schedules proactive silent renew from expires_in, not from
// decoding the access token.
server.service.on('beforeResponse', (tokenEndpointResponse) => {
  if (tokenEndpointResponse.body && typeof tokenEndpointResponse.body === 'object') {
    tokenEndpointResponse.body.expires_in = TTL
  }
})

server.service.on('beforeUserinfo', (userInfoResponse) => {
  userInfoResponse.body = {
    sub: user.sub,
    preferred_username: user.preferred_username,
    name: user.name,
    email: user.email,
    permissions: user.permissions,
  }
})

const app = express()
app.use(cors())
app.use(server.service.requestHandler)
app.listen(PORT, () => {
  console.log(`[auth-mock] issuer:      http://localhost:${PORT}`)
  console.log(`[auth-mock] active user: ${ACTIVE} (${user.preferred_username})`)
  console.log(`[auth-mock] token ttl:   ${TTL}s`)
})
