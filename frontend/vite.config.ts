import { defineConfig, loadEnv, Plugin, type PluginOption } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

/** Local Sub2API backend default (see Agents.md / backend/config.yaml). */
const DEFAULT_DEV_BACKEND = 'http://127.0.0.1:18081'

/** Rate-limit inject-public-settings warnings so a down backend does not flood the console. */
const PUBLIC_SETTINGS_WARN_INTERVAL_MS = 15_000
let lastPublicSettingsWarnAt = 0

/**
 * Expand a backend base URL with Windows-friendly loopback fallbacks.
 * Node may try localhost → ::1 first; if the Go process only has an IPv4
 * listener ready yet (or vice versa), fetch fails with afterConnectMultiple.
 */
function backendFetchCandidates(backendUrl: string): string[] {
  const candidates = [backendUrl]
  try {
    const u = new URL(backendUrl)
    const port = u.port ? `:${u.port}` : ''
    if (u.hostname === 'localhost' || u.hostname === '::1' || u.hostname === '[::1]') {
      candidates.push(`${u.protocol}//127.0.0.1${port}`)
    }
    if (u.hostname === 'localhost' || u.hostname === '127.0.0.1') {
      candidates.push(`${u.protocol}//[::1]${port}`)
    }
  } catch {
    // keep primary only
  }
  return [...new Set(candidates)]
}

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        let lastError: unknown
        for (const base of backendFetchCandidates(backendUrl)) {
          try {
            const response = await fetch(`${base}/api/v1/settings/public`, {
              signal: AbortSignal.timeout(2000)
            })
            if (response.ok) {
              const data = await response.json()
              if (data.code === 0 && data.data) {
                const script = `<script>window.__APP_CONFIG__=${JSON.stringify(data.data)};</script>`
                return html.replace('</head>', `${script}\n</head>`)
              }
            } else {
              lastError = new Error(`HTTP ${response.status} from ${base}`)
            }
          } catch (e) {
            lastError = e
          }
        }

        const now = Date.now()
        if (now - lastPublicSettingsWarnAt >= PUBLIC_SETTINGS_WARN_INTERVAL_MS) {
          lastPublicSettingsWarnAt = now
          const detail =
            lastError instanceof Error
              ? lastError.message
              : lastError
                ? String(lastError)
                : 'unknown error'
          console.warn(
            `[vite] 无法获取公开配置（${backendUrl}），将回退到浏览器 API 调用: ${detail}`
          )
          console.warn(
            '[vite] 请确认后端已在 18081 监听（scripts/dev-stack.ps1 status），且本机 PostgreSQL/Redis 就绪。'
          )
        }
        return html
      }
    }
  }
}

export default defineConfig(async ({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  // Prefer explicit env; never fall back to the historical Docker port 8080.
  const backendUrl = (env.VITE_DEV_PROXY_TARGET || DEFAULT_DEV_BACKEND).replace(/\/$/, '')
  const devPort = Number(env.VITE_DEV_PORT || 15174)

  // vite-plugin-checker (vue-tsc) can add hundreds of MB–1GB+ RSS on this codebase.
  // Opt in with VITE_DEV_CHECKER=1 / true / on, or `pnpm run dev:check`.
  // Prefer IDE typecheck or `pnpm run typecheck` for day-to-day local work.
  const enableChecker = ['1', 'true', 'on', 'yes'].includes(
    String(env.VITE_DEV_CHECKER || '').toLowerCase()
  )

  // Prefer loopback in local dev to avoid binding every NIC (LAN/WSL/VPN adapters).
  // Override with VITE_DEV_HOST=0.0.0.0 when you need phone/LAN access.
  const devHost = env.VITE_DEV_HOST || '127.0.0.1'

  const plugins: PluginOption[] = [vue(), injectPublicSettings(backendUrl)]
  if (enableChecker) {
    // Dynamic import keeps the default `pnpm dev` config graph free of checker/typescript.
    const { default: checker } = await import('vite-plugin-checker')
    plugins.splice(
      1,
      0,
      checker({
        // vue-tsc already typechecks TS + Vue SFCs; do not also run plain tsc
        // (double language servers ≈ double memory).
        typescript: false,
        vueTsc: true
      })
    )
  }

  return {
    plugins,
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
        // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
        'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
      }
    },
    define: {
      // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
      // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
      __INTLIFY_JIT_COMPILATION__: true
    },
    build: {
      outDir: '../backend/internal/web/dist',
      emptyOutDir: true,
      rollupOptions: {
        output: {
          /**
           * 手动分包配置
           * 分离第三方库并按功能合并应用代码，避免循环依赖
           */
          manualChunks(id: string) {
            if (id.includes('node_modules')) {
              // Vue 核心库
              if (
                id.includes('/vue/') ||
                id.includes('/vue-router/') ||
                id.includes('/pinia/') ||
                id.includes('/@vue/')
              ) {
                return 'vendor-vue'
              }

              // UI 工具库（较大，单独分离）
              if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
                return 'vendor-ui'
              }

              // 图表库
              if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
                return 'vendor-chart'
              }

              // 国际化
              if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
                return 'vendor-i18n'
              }

              // 其他小型第三方库合并
              return 'vendor-misc'
            }

            // 应用代码：按入口点自动分包，不手动干预
            // 这样可以避免循环依赖，同时保持合理的 chunk 数量
          }
        }
      }
    },
    server: {
      host: devHost,
      port: devPort,
      strictPort: true,
      // Avoid eager transform of random requests (devtools, crawlers, stale HMR)
      // which can pin large module graphs in memory during local dev.
      preTransformRequests: false,
      // Keep the watcher focused on app sources. Do NOT add parent-repo paths
      // like `../backend/**` here — relative `..` globs can make chokidar expand
      // the watch root to the monorepo and burn multi-GB RSS on Windows.
      watch: {
        ignored: ['**/node_modules/**', '**/.git/**', '**/dist/**', '**/coverage/**', '**/tmp/**']
      },
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/antigravity': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
