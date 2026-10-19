<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from 'pdfjs-dist'
import workerURL from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

const props = defineProps<{ file: File | null }>()
const canvas = ref<HTMLCanvasElement | null>(null)
const viewport = ref<HTMLDivElement | null>(null)
const page = ref(1)
const pages = ref(0)
const loading = ref(false)
const rendering = ref(false)
const error = ref('')
const fileURL = ref('')
let task: PDFDocumentLoadingTask | null = null
let document: PDFDocumentProxy | null = null
let drawing: RenderTask | null = null
let observer: ResizeObserver | null = null
let version = 0
let drawVersion = 0

function release() {
  version++
  drawVersion++
  drawing?.cancel()
  drawing = null
  void task?.destroy().catch(() => {})
  task = null
  document = null
  if (fileURL.value) URL.revokeObjectURL(fileURL.value)
  fileURL.value = ''
}

async function drawPage() {
  const pdf = document,
    target = canvas.value,
    container = viewport.value
  if (!pdf || !target || !container) return
  const current = ++drawVersion
  const previous = drawing
  previous?.cancel()
  rendering.value = true
  try {
    await previous?.promise.catch(() => {})
    const sheet = await pdf.getPage(page.value)
    if (current !== drawVersion) return
    const base = sheet.getViewport({ scale: 1 })
    const scale = Math.max(
      0.1,
      Math.min(
        (container.clientWidth - 32) / base.width,
        (container.clientHeight - 32) / base.height,
      ),
    )
    const view = sheet.getViewport({ scale })
    const density = Math.min(window.devicePixelRatio || 1, 2)
    target.width = Math.floor(view.width * density)
    target.height = Math.floor(view.height * density)
    target.style.width = `${view.width}px`
    target.style.height = `${view.height}px`
    drawing = sheet.render({
      canvas: target,
      viewport: view,
      transform: [density, 0, 0, density, 0, 0],
    })
    await drawing.promise
  } catch (e) {
    if (
      current === drawVersion &&
      !(e instanceof Error && e.name === 'RenderingCancelledException')
    )
      error.value = '此页无法预览，可打开原文件核对。'
  } finally {
    if (current === drawVersion) rendering.value = false
  }
}

watch(
  () => props.file,
  async (file) => {
    release()
    const current = version
    page.value = 1
    pages.value = 0
    error.value = ''
    loading.value = Boolean(file)
    if (!file) return
    fileURL.value = URL.createObjectURL(file)
    try {
      const pdfjs = await import('pdfjs-dist')
      const data = await file.arrayBuffer()
      if (current !== version) return
      pdfjs.GlobalWorkerOptions.workerSrc = workerURL
      const base = `${import.meta.env.BASE_URL}pdfjs/`
      task = pdfjs.getDocument({
        data,
        cMapUrl: `${base}cmaps/`,
        cMapPacked: true,
        iccUrl: `${base}iccs/`,
        standardFontDataUrl: `${base}standard_fonts/`,
        wasmUrl: `${base}wasm/`,
      })
      const loaded = await task.promise
      if (current !== version) return
      document = loaded
      pages.value = loaded.numPages
      loading.value = false
      await nextTick()
      await drawPage()
    } catch (e) {
      if (current !== version) return
      error.value =
        e instanceof Error && e.name === 'PasswordException'
          ? '这份 PDF 需要密码，请打开原文件核对。'
          : '无法预览这份 PDF，请检查文件或打开原文件核对。'
    } finally {
      if (current === version) loading.value = false
    }
  },
  { immediate: true },
)
watch(page, () => {
  error.value = ''
  void drawPage()
})
onMounted(() => {
  observer = new ResizeObserver(() => {
    void drawPage()
  })
  if (viewport.value) observer.observe(viewport.value)
})
onUnmounted(() => {
  observer?.disconnect()
  release()
})
</script>

<template>
  <section class="pdf-preview" aria-label="题面 PDF 预览">
    <div class="preview-toolbar">
      <strong>题面预览</strong>
      <a v-if="fileURL" :href="fileURL" target="_blank" rel="noopener">打开原文件</a>
    </div>
    <div ref="viewport" class="pdf-viewport" :aria-busy="loading || rendering">
      <canvas
        ref="canvas"
        v-show="pages > 0 && !error && !loading"
        :aria-label="`PDF 第 ${page} 页`"
      />
      <div
        v-if="!file || loading || error"
        class="preview-message"
        :role="error ? 'alert' : 'status'"
      >
        <el-icon><Document /></el-icon>
        <strong>{{
          !file ? '上传后在这里预览题面' : loading ? '正在加载 PDF…' : '暂时无法预览'
        }}</strong>
        <span>{{
          error || (!file ? '检查页码、题号与排版，确认考生收到的正式文件。' : '文件仅在本地预览')
        }}</span>
      </div>
    </div>
    <div class="preview-pages">
      <el-button
        size="small"
        :disabled="!pages || page <= 1"
        aria-label="PDF 上一页"
        @click="page--"
        >上一页</el-button
      >
      <span>{{ pages ? `第 ${page} / ${pages} 页` : '尚未选择 PDF' }}</span>
      <el-button
        size="small"
        :disabled="!pages || page >= pages"
        aria-label="PDF 下一页"
        @click="page++"
        >下一页</el-button
      >
    </div>
  </section>
</template>

<style scoped>
.pdf-preview {
  border: 1px solid var(--oj-border);
  border-radius: 10px;
  overflow: hidden;
  min-width: 0;
}
.preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  background: #fff;
  border-bottom: 1px solid var(--oj-border);
  font-size: 13px;
}
.preview-toolbar a {
  color: var(--oj-primary);
  font-size: 12px;
  text-decoration: none;
}
.pdf-viewport {
  height: 460px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: #edf0f5;
}
.pdf-viewport canvas {
  display: block;
  box-shadow: 0 2px 10px #2230481a;
  background: #fff;
}
.preview-message {
  display: flex;
  align-items: center;
  text-align: center;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
  color: var(--oj-text-secondary);
  font-size: 13px;
}
.preview-message .el-icon {
  font-size: 36px;
  color: #8a98af;
}
.preview-message strong {
  color: var(--oj-text);
  font-weight: 500;
}
.preview-message span {
  max-width: 300px;
  line-height: 1.7;
}
.preview-pages {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  padding: 12px;
  background: #fff;
  border-top: 1px solid var(--oj-border);
  font-size: 12px;
  color: var(--oj-text-secondary);
}
@media (max-width: 600px) {
  .pdf-viewport {
    height: 400px;
  }
}
</style>
