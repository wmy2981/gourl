#!/usr/bin/env node
/**
 * Dumps the gourl SQLite database into per-entity JSON files, one per table:
 *
 *   links.json        — 7-field rows, same shape and ordering as the admin
 *                       export (GET /api/v1/export.json)
 *   tokens.json       — API tokens (id, token, note, created_at)
 *   daily-clicks.json — daily click history (code, date, count)
 *
 * Zero dependencies: built on node:sqlite and TypeScript type stripping.
 * Requires Node >= 22.6.
 *
 * Usage:
 *   node --experimental-sqlite --experimental-strip-types scripts/db-export.mts <db-path> [out-dir]
 *
 * The token file contains full API token values — treat the output directory
 * like a credential store.
 */
import { DatabaseSync } from 'node:sqlite'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'

// Link row shape — mirrors internal/api/export.go's exportRow, so the links
// output is interchangeable with the admin export JSON.
interface LinkRow {
  code: string
  url: string
  title: string
  description: string
  expires_at: number
  click_count: number
  created_at: number
}

interface TokenRow {
  id: number
  token: string
  note: string
  created_at: number
}

interface DailyClickRow {
  code: string
  date: string
  count: number
}

const [dbPath, outDir = '.'] = process.argv.slice(2)

if (!dbPath) {
  console.error('usage: node --experimental-sqlite --experimental-strip-types scripts/db-export.mts <db-path> [out-dir]')
  process.exit(1)
}
if (!existsSync(dbPath)) {
  console.error(`db not found: ${dbPath}`)
  process.exit(1)
}
mkdirSync(outDir, { recursive: true })

// Open read-only so a typo can never mutate the live database.
const db = new DatabaseSync(dbPath)
db.exec('PRAGMA query_only = ON')

const links = db
  .prepare(
    `SELECT code, url, title, description, expires_at, click_count, created_at
     FROM links ORDER BY created_at DESC, rowid DESC`,
  )
  .all() as LinkRow[]
writeJson(path.join(outDir, 'links.json'), links)

const tokens = db
  .prepare(`SELECT id, token, note, created_at FROM api_tokens ORDER BY id DESC`)
  .all() as TokenRow[]
writeJson(path.join(outDir, 'tokens.json'), tokens)

const dailies = db
  .prepare(`SELECT code, date, count FROM daily_clicks ORDER BY date DESC, code`)
  .all() as DailyClickRow[]
writeJson(path.join(outDir, 'daily-clicks.json'), dailies)

console.log(
  `wrote ${links.length} links, ${tokens.length} tokens, ${dailies.length} daily-click rows to ${outDir}`,
)
if (tokens.length > 0) {
  console.warn('note: tokens.json contains full API token values — keep it private')
}

function writeJson(file: string, rows: unknown[]) {
  writeFileSync(file, JSON.stringify(rows, null, 2) + '\n')
}
