// Serves @hugeicons/core-free-icons as one static JSON file per icon export name
// (aliases included, so any stored name resolves with a single small fetch), plus a
// name catalog for the icon picker, so the app can fetch only the icons a page shows
// instead of bundling the whole catalog. Files are written to assets/hugeicons/<version>/ at build time
// (served by the Go service's /assets route) and served on demand by the dev server.
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const PACKAGE = '@hugeicons/core-free-icons'
const EXPORT_LINE = /^export \{ (.+) \} from '\.\/([A-Za-z0-9_]+)\.js';$/

function readPackage(root) {
  const dir = resolve(root, 'node_modules', PACKAGE)
  const { version } = JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'))
  return { dir, esmDir: join(dir, 'dist', 'esm'), version }
}

// Parses dist/esm/index.js into the icon file names and the export aliases that
// differ from them (e.g. DatabaseFreeIcons -> DatabaseIcon).
export function parseIconIndex(source) {
  const icons = []
  const aliases = {}
  for (const line of source.split('\n')) {
    const match = EXPORT_LINE.exec(line.trim())
    if (!match) continue
    const file = match[2]
    icons.push(file)
    for (const part of match[1].split(',')) {
      const alias = part.trim().replace(/^default as /, '')
      if (alias && alias !== file) aliases[alias] = file
    }
  }
  icons.sort()
  return { icons, aliases }
}

async function loadIconData(esmDir, file) {
  const mod = await import(pathToFileURL(join(esmDir, `${file}.js`)).href)
  return JSON.stringify(mod.default)
}

export default function hugeiconsPlugin() {
  let pkg
  let catalog
  let urlPrefix

  const getCatalog = () => {
    catalog ??= parseIconIndex(readFileSync(join(pkg.esmDir, 'index.js'), 'utf8'))
    return catalog
  }

  return {
    name: 'sickrock-hugeicons',

    config(userConfig) {
      pkg = readPackage(userConfig.root ?? process.cwd())
      const base = userConfig.base ?? '/'
      urlPrefix = `${base.replace(/\/$/, '')}/assets/hugeicons/${pkg.version}`
      return { define: { __HUGEICONS_BASE__: JSON.stringify(urlPrefix) } }
    },

    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const path = (req.url ?? '').split('?')[0]
        if (!path.startsWith(`${urlPrefix}/`)) return next()
        const name = path.slice(urlPrefix.length + 1).replace(/\.json$/, '')
        try {
          const { icons, aliases } = getCatalog()
          let body
          if (name === 'catalog') {
            body = JSON.stringify(getCatalog())
          } else if (icons.includes(name) || aliases[name]) {
            body = await loadIconData(pkg.esmDir, aliases[name] ?? name)
          } else {
            res.statusCode = 404
            return res.end()
          }
          res.setHeader('Content-Type', 'application/json')
          res.end(body)
        } catch (err) {
          next(err)
        }
      })
    },

    // Written straight to the output directory rather than emitted as bundle assets,
    // so the ~6k files don't flood Vite's build report.
    async writeBundle(options) {
      const { icons, aliases } = getCatalog()
      const dir = join(options.dir, 'assets', 'hugeicons', pkg.version)
      mkdirSync(dir, { recursive: true })
      writeFileSync(join(dir, 'catalog.json'), JSON.stringify({ icons, aliases }))
      const data = new Map(await Promise.all(icons.map(async (file) => [file, await loadIconData(pkg.esmDir, file)])))
      for (const [file, json] of data) writeFileSync(join(dir, `${file}.json`), json)
      for (const [alias, file] of Object.entries(aliases)) writeFileSync(join(dir, `${alias}.json`), data.get(file))
      this.info(`wrote ${icons.length} icons and ${Object.keys(aliases).length} aliases to ${dir}`)
    },
  }
}
