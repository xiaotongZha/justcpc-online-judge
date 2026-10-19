import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { pdfPreviewAssets } from './pdfPreviewAssets'
export default defineConfig({
  plugins: [vue(), pdfPreviewAssets()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue'],
          element: ['element-plus', '@element-plus/icons-vue'],
          markdown: ['marked', 'marked-katex-extension', 'dompurify'],
        },
      },
    },
  },
})
