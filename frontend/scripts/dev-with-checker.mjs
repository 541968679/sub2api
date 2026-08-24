/**
 * Start Vite with in-browser typechecking (vue-tsc via vite-plugin-checker).
 * Default `pnpm dev` leaves checker off to keep local RSS much lower.
 */
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const viteBin = join(root, 'node_modules', 'vite', 'bin', 'vite.js')

const child = spawn(process.execPath, [viteBin], {
  cwd: root,
  stdio: 'inherit',
  env: {
    ...process.env,
    VITE_DEV_CHECKER: '1'
  }
})

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal)
    return
  }
  process.exit(code ?? 0)
})
