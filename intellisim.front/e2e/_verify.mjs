import { chromium } from 'playwright'
import fs from 'fs'

const OUT = process.env.E2E_SHOT_DIR || 'test-results/_uiscan'
fs.mkdirSync(OUT, { recursive: true })

const consoleErrors = []
const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1280, height: 800 } })
page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()) })

const log = (...a) => console.log(...a)
async function shot(name) { await page.screenshot({ path: `${OUT}/v-${name}.png` }); log('shot:', name) }
async function bubbleTexts() {
  return await page.locator('.flex.flex-col.h-full .space-y-3 > div').allInnerTexts().catch(() => [])
}

// ---- 1. Default + provider dropdown ----
await page.goto('http://localhost:3000/', { waitUntil: 'networkidle' })
await page.waitForTimeout(2500) // allow provider retry to settle
const providerOpts = await page.locator('aside select option').allInnerTexts().catch(() => [])
log('provider options:', JSON.stringify(providerOpts))
await shot('01-default')

// ---- 2. Collapse / expand sidebar; check toggle stays inside the 64px rail ----
const asideBox = async () => await page.locator('aside').boundingBox()
await page.locator('aside button').first().click()
await page.waitForTimeout(400)
await shot('02-collapsed')
{
  const aside = await asideBox()
  const btn = await page.locator('aside button').first().boundingBox()
  const inside = btn && aside && btn.x >= aside.x - 1 && (btn.x + btn.width) <= (aside.x + aside.width + 1)
  log('collapsed rail width:', Math.round(aside.width), '| toggle inside rail:', inside, '| btn.x:', Math.round(btn.x), 'btn.right:', Math.round(btn.x + btn.width))
}
await page.locator('aside button').first().click() // expand back
await page.waitForTimeout(400)

// ---- 3. General chat: send + expect user msg AND assistant reply to persist ----
await page.goto('http://localhost:3000/chat/new', { waitUntil: 'networkidle' })
await page.waitForTimeout(500)
await page.locator('.p-inputtext').first().fill('Reply with exactly: PONG')
await page.locator('button:has(.pi-send)').first().click()
await page.waitForFunction(() => {
  const els = document.querySelectorAll('.space-y-3 > div')
  return Array.from(els).some(e => /pong/i.test(e.textContent || ''))
}, { timeout: 60000 }).catch(() => log('general chat: no PONG within timeout'))
await page.waitForTimeout(800)
{
  const txt = (await bubbleTexts()).join(' || ')
  log('general chat bubbles:', txt.slice(0, 200))
  log('  -> user message present:', /Reply with exactly: PONG/i.test(txt))
  log('  -> assistant reply present:', /pong/i.test(txt))
}
await shot('03-general-chat')

// ---- 4. Model chat: generate model, expect friendly text + populated preview ----
await page.goto('http://localhost:3000/models/new', { waitUntil: 'networkidle' })
await page.waitForTimeout(500)
await page.locator('.p-inputtext').first().fill('Model predator-prey with 3 wolves and 15 sheep on a 200x200 grid')
await page.locator('button:has(.pi-send)').first().click()
log('model chat: waiting for generation (LLM)...')
await page.waitForFunction(() => document.body.textContent?.includes('Agents ('), { timeout: 120000 })
  .catch(() => log('model chat: preview did not populate within timeout'))
await page.waitForTimeout(1000)
{
  const bubbles = (await bubbleTexts()).join(' || ')
  const rawJson = /```|"is_spatial"|"agents":\s*\[/.test(bubbles)
  log('model chat bubble (first 160):', bubbles.slice(0, 160))
  log('  -> raw JSON leaked into chat:', rawJson)
  const previewText = await page.locator('.w-\\[400px\\]').innerText().catch(() => '')
  log('  -> preview shows Agents:', /Agents \(\d+\)/.test(previewText))
  log('  -> preview shows rules:', /\d+ rules/.test(previewText))
  const createBtn = page.getByRole('button', { name: /Create Simulation/i })
  log('  -> Create Simulation visible:', await createBtn.isVisible().catch(() => false),
      '| enabled:', await createBtn.isEnabled().catch(() => false))
}
await shot('04-model-chat')

// ---- 5. Create Simulation -> should navigate to /simulations/{simId} ----
const urlBefore = page.url()
await page.getByRole('button', { name: /Create Simulation/i }).click().catch((e) => log('create sim click err', e.message))
await page.waitForURL(/\/simulations\/[a-f0-9]{24}/, { timeout: 120000 })
  .catch(() => log('did not navigate to a simulation route'))
await page.waitForTimeout(1500)
log('url before:', urlBefore.replace('http://localhost:3000',''), '-> after:', page.url().replace('http://localhost:3000',''))
await shot('05-simulation')

fs.writeFileSync(`${OUT}/verify-report.json`, JSON.stringify({ providerOpts, consoleErrors }, null, 2))
log('\n=== CONSOLE ERRORS ===\n' + (consoleErrors.join('\n') || '(none)'))
await browser.close()
