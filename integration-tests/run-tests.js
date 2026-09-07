// Starts the SickRock service with -configdir and runs mocha.

import { createServer } from 'node:net'
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import waitOn from 'wait-on'

const __dirname = dirname(fileURLToPath(import.meta.url))
const repoRoot = join(__dirname, '..')
const testsDir = join(__dirname, 'tests')
const serviceBin = join(repoRoot, 'service', 'tmp', 'sickrock')
const frontendDist = join(repoRoot, 'frontend', 'dist')
const testDb = join(repoRoot, 'tmp', 'sickrock.db')

async function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = createServer()
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      const port = typeof address === 'object' && address ? address.port : 0
      server.close((err) => (err ? reject(err) : resolve(port)))
    })
    server.on('error', reject)
  })
}

function resetTestDatabase() {
  mkdirSync(join(repoRoot, 'tmp'), { recursive: true })
  for (const suffix of ['', '-wal', '-shm']) {
    try {
      rmSync(testDb + suffix)
    } catch {
      // ignore missing files
    }
  }
}

function integrationEnv(testPort) {
  const env = {
    ...process.env,
    PORT: String(testPort),
    JWT_SECRET: 'integration-test-secret',
    SICKROCK_FRONTEND_DIR: frontendDist,
  }

  for (const key of ['DB_HOST', 'DB_PORT', 'DB_USER', 'DB_PASS', 'DB_NAME']) {
    delete env[key]
  }

  return env
}

async function main() {
  if (!existsSync(serviceBin)) {
    throw new Error(`SickRock binary not found at ${serviceBin}. Run "make service-build" from the repository root first.`)
  }

  if (!existsSync(join(frontendDist, 'index.html'))) {
    throw new Error(`Frontend build not found at ${frontendDist}. Run "make frontend-build" from the repository root first.`)
  }

  resetTestDatabase()

  const testPort = process.env.SICKROCK_TEST_PORT
    ? Number(process.env.SICKROCK_TEST_PORT)
    : await getFreePort()
  const baseUrl = process.env.SICKROCK_BASE_URL || `http://127.0.0.1:${testPort}`
  const env = integrationEnv(testPort)

  const child = spawn(serviceBin, ['-configdir', testsDir], {
    cwd: join(repoRoot, 'service'),
    env,
    stdio: ['ignore', 'inherit', 'inherit'],
  })

  try {
    await waitOn({
      resources: [`http-get://127.0.0.1:${testPort}/login`],
      timeout: 30000,
      validateStatus: (status) => status >= 200 && status < 500,
    })

    const mocha = spawn(
      'npx',
      ['mocha', '--timeout', '30000', 'tests/**/*.spec.js'],
      {
        cwd: __dirname,
        env: {
          ...env,
          SICKROCK_BASE_URL: baseUrl,
        },
        stdio: 'inherit',
      },
    )

    const code = await new Promise((resolve) => mocha.on('close', resolve))
    child.kill('SIGTERM')
    process.exit(code ?? 1)
  } catch (err) {
    child.kill('SIGTERM')
    throw err
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
