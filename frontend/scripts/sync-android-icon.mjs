// Generate the Android launcher icons from assets/favicon.svg (the single
// icon source, see root CLAUDE.md). Follows the it-tools-apk-builder
// approach: render the favicon once to a 512px PNG and copy it into every
// density's legacy mipmap (ic_launcher.png / ic_launcher_round.png). NO
// adaptive-icon resources (mipmap-anydpi-v26, foreground/background drawables)
// — the platform scales, centers and masks legacy bitmaps itself, so the
// glyph can never be misplaced by hand-written transform math.
//
// Needs sharp from the frontend dependency tree; run from the repo root:
//   cd frontend && node scripts/sync-android-icon.mjs
// (or via the android workflow, which runs it after `npm ci`).

import { mkdir, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import sharp from 'sharp'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const SRC = path.join(ROOT, 'assets', 'favicon.svg')
const RES = path.join(ROOT, 'frontend', 'android', 'app', 'src', 'main', 'res')
const DENSITIES = ['mdpi', 'hdpi', 'xhdpi', 'xxhdpi', 'xxxhdpi']
const SIZE = 512

const png = await sharp(SRC).resize(SIZE, SIZE, { fit: 'contain' }).png().toBuffer()

for (const d of DENSITIES) {
  const dir = path.join(RES, `mipmap-${d}`)
  await mkdir(dir, { recursive: true })
  await writeFile(path.join(dir, 'ic_launcher.png'), png)
  await writeFile(path.join(dir, 'ic_launcher_round.png'), png)
  console.log(`wrote mipmap-${d}/ic_launcher.png + ic_launcher_round.png`)
}
