import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { createRequire } from 'node:module'
import type { Plugin } from 'vite'

// PDF 字体、字形映射和解码器随前端打包，部署时无需连接外部 CDN。
export function pdfPreviewAssets(): Plugin {
  const root = dirname(createRequire(import.meta.url).resolve('pdfjs-dist/package.json'))
  const files = new Map<string, string>()
  for (const folder of ['cmaps', 'standard_fonts', 'wasm', 'iccs']) {
    for (const entry of readdirSync(join(root, folder), { withFileTypes: true })) {
      if (entry.isFile()) files.set(`pdfjs/${folder}/${entry.name}`, join(root, folder, entry.name))
    }
  }
  files.set('pdfjs/LICENSE', join(root, 'LICENSE'))
  return {
    name: 'local-pdf-preview-assets',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const path = files.get((req.url || '').split('?')[0].replace(/^\//, ''))
        if (!path) return next()
        res.setHeader(
          'Content-Type',
          path.endsWith('.wasm') ? 'application/wasm' : 'application/octet-stream',
        )
        res.end(readFileSync(path))
      })
    },
    generateBundle() {
      for (const [fileName, path] of files)
        this.emitFile({ type: 'asset', fileName, source: readFileSync(path) })
    },
  }
}
