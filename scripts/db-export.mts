#!/usr/bin/env node
/**
 * Dumps the gourl SQLite database into per-entity JSON files, one per table:
 *
 *   links.json        — 7-field rows, same shape and ordering as the admin
 *                       export (GET /api/v1/export.json)
 *   tokens.json       — API tokens (id, token, note, created_at)
 *   daily-clicks.json — daily click history (code, date, count)
 *
 * Zero dependencies. Runs on both runtimes — node uses the built-in
 * node:sqlite (Node >= 22.10, type stripping from 22.6), bun its own
 * bun:sqlite (no flags needed at all):
 *
 *   node --experimental-sqlite --experimental-strip-types scripts/db-export.mts <db-path> [out-dir]
 *   bun run scripts/db-export.mts <db-path> [out-dir]
 *
 * The token file contains full API token values — treat the output directory
 * like a credential store.
 */
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

type Row = Record<string, unknown>

// The minimal surface the two runtime backends share: synchronous exec and
// "all rows of a query" (bun's query().all() vs node's prepare().all()).
interface SqliteBackend {
  exec(sql: string): void
  all(sql: string): Row[]
}

async function openDb(dbPath: string): Promise<SqliteBackend> {
  if (typeof Bun !== 'undefined') {
    const { Database } = await import('bun:sqlite')
    const db = new Database(dbPath, { readonly: true })
    return {
      exec: (sql) => {
        db.exec(sql)
      },
      all: (sql) => db.query(sql).all() as Row[],
    }
  }
  const { DatabaseSync } = await import('node:sqlite')
  const db = new DatabaseSync(dbPath, { readOnly: true })
  return {
    exec: (sql) => {
      db.exec(sql)
    },
    all: (sql) => db.prepare(sql).all() as Row[],
  }
}

const args = process.argv.slice(2)
if (args.includes('--help') || args.includes('-h')) {
  printHelp()
  process.exit(0)
}

const [dbPath, outDir = '.'] = args

if (!dbPath) {
  console.error('missing <db-path> — run with --help for usage')
  process.exit(1)
}
if (!existsSync(dbPath)) {
  console.error(`db not found: ${dbPath}`)
  process.exit(1)
}
mkdirSync(outDir, { recursive: true })

// Open read-only (constructor option on both runtimes) so a typo can never
// mutate the live database.
const db = await openDb(dbPath)

const links = db.all(
  `SELECT code, url, title, description, expires_at, click_count, created_at
   FROM links ORDER BY created_at DESC, rowid DESC`,
) as LinkRow[]
writeJson(path.join(outDir, 'links.json'), links)

const tokens = db.all(
  `SELECT id, token, note, created_at FROM api_tokens ORDER BY id DESC`,
) as TokenRow[]
writeJson(path.join(outDir, 'tokens.json'), tokens)

const dailies = db.all(
  `SELECT code, date, count FROM daily_clicks ORDER BY date DESC, code`,
) as DailyClickRow[]
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

function printHelp() {
  console.log(`gourl db-export — dump the gourl SQLite database into per-entity JSON files

USAGE
  node --experimental-sqlite --experimental-strip-types scripts/db-export.mts <db-path> [out-dir]
  bun run scripts/db-export.mts <db-path> [out-dir]

  node requires >= 22.10 (type stripping from 22.6); bun needs no flags.

ARGUMENTS
  <db-path>  Path to the gourl SQLite database (required)
  [out-dir]  Output directory, created if missing (default: current directory)

OUTPUT — one file per entity, each a JSON array:
  links.json        7-field link rows, same shape and ordering as the admin
                    export (GET /api/v1/export.json)
  tokens.json       API tokens (id, token, note, created_at)
  daily-clicks.json Daily click history (code, date, count)

SECURITY
  The database is opened read-only. tokens.json contains full API token
  values — treat the output directory like a credential store.

EXIT CODES
  0  success    1  missing db / bad arguments`)
}

