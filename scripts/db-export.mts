#!/usr/bin/env node
/**
 * Dumps the gourl SQLite database into per-entity JSON files, one per table:
 *
 *   links.json        — full link rows incl. id and deleted (soft-deleted
 *                       links are kept, flagged deleted: true)
 *   tokens.json       — API tokens incl. deleted (revoked tokens are kept)
 *   daily-clicks.json — daily click history keyed by link id
 *   backups.json      — edit snapshots, b_id as the "b-1, b-2, …" string
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

// Full links row, with deleted normalized to a boolean. The shape mirrors the
// store schema (v3+), not the 7-field admin export — this script dumps the
// database, deleted rows included.
interface LinkRow {
  id: number
  code: string
  url: string
  title: string
  description: string
  expires_at: number
  click_count: number
  created_at: number
  updated_at: number
  deleted: boolean
}

interface TokenRow {
  id: number
  token: string
  note: string
  created_at: number
  deleted: boolean
}

interface DailyClickRow {
  link_id: number | null
  code: string
  date: string
  count: number
}

interface BackupRow {
  b_id: string
  link_id: number
  code: string
  url: string
  title: string
  description: string
  expires_at: number
  click_count: number
  created_at: number
  updated_at: number
  backed_at: number
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

const rawLinks = db.all(
  `SELECT id, code, url, title, description, expires_at, click_count, created_at, updated_at, deleted
   FROM links ORDER BY created_at DESC, rowid DESC`,
)
const links: LinkRow[] = rawLinks.map((r) => ({ ...r, deleted: (r.deleted as number) !== 0 }))
writeJson(path.join(outDir, 'links.json'), links)

const rawTokens = db.all(
  `SELECT id, token, note, created_at, deleted FROM api_tokens ORDER BY id DESC`,
)
const tokens: TokenRow[] = rawTokens.map((r) => ({ ...r, deleted: (r.deleted as number) !== 0 }))
writeJson(path.join(outDir, 'tokens.json'), tokens)

const dailies = db.all(
  `SELECT link_id, code, date, count FROM daily_clicks ORDER BY date DESC, code`,
) as DailyClickRow[]
writeJson(path.join(outDir, 'daily-clicks.json'), dailies)

const rawBackups = db.all(
  `SELECT b_id, link_id, code, url, title, description, expires_at, click_count, created_at, updated_at, backed_at
   FROM backups ORDER BY b_id`,
)
const backups: BackupRow[] = rawBackups.map((r) => ({ ...r, b_id: `b-${r.b_id}` }))
writeJson(path.join(outDir, 'backups.json'), backups)

console.log(
  `wrote ${links.length} links, ${tokens.length} tokens, ${dailies.length} daily-click rows, ${backups.length} backups to ${outDir}`,
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
  links.json        Full link rows incl. id and deleted (soft-deleted links
                    are kept, flagged deleted: true)
  tokens.json       API tokens incl. deleted (revoked tokens are kept)
  daily-clicks.json Daily click history keyed by link id (link_id, code, date, count)
  backups.json      Edit snapshots, b_id as the "b-1, b-2, …" string

SECURITY
  The database is opened read-only. tokens.json contains full API token
  values — treat the output directory like a credential store.

EXIT CODES
  0  success    1  missing db / bad arguments`)
}
