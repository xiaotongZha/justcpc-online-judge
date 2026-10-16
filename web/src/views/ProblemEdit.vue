<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import {
  store,
  goBack,
  navigate,
  refresh,
  languageLabel,
  type EditorProblem,
  type PublicProblem,
} from '../store'
import Markdown from '../components/Markdown.vue'

const emptyForm = () => ({
  title: '',
  statement_markdown: '',
  score: '100',
  compare: { floating_point: false, epsilon: 0.000001 },
  resource_limits: {
    cpp: { time_limit_ms: 1000, memory_limit_mb: 256 },
    java: { time_limit_ms: 1000, memory_limit_mb: 256 },
    python3: { time_limit_ms: 2000, memory_limit_mb: 512 },
  },
})
const form = reactive(emptyForm())
const loading = ref(false)
const saving = ref(false)
const testFile = ref<File | null>(null)

const isEdit = computed(() => store.editTargetID > 0)

async function load() {
  if (!isEdit.value) {
    // 新建：复用同一页面，重置为空白表单，测试数据可在保存时一并上传。
    Object.assign(form, emptyForm())
    store.selectedEditor = null
    testFile.value = null
    return
  }
  loading.value = true
  try {
    const item = await api<EditorProblem>(`/problems/${store.editTargetID}/editor`)
    store.selectedEditor = item
    Object.assign(form, {
      title: item.title,
      statement_markdown: item.statement_markdown,
      score: item.score,
      compare: { ...item.compare },
      resource_limits: JSON.parse(JSON.stringify(item.resource_limits)),
    })
    testFile.value = null
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

watch(() => store.editTargetID, load, { immediate: true })

async function save() {
  if (!form.title.trim()) {
    ElMessage.warning('标题不能为空')
    return
  }
  saving.value = true
  try {
    // 新建与编辑共用同一提交逻辑，且允许不提供测试数据。
    const item = await api<EditorProblem>(
      isEdit.value ? `/problems/${store.editTargetID}` : '/problems',
      { method: isEdit.value ? 'PUT' : 'POST', body: JSON.stringify(form) },
    )
    store.selectedEditor = item
    store.editTargetID = item.id
    if (testFile.value) {
      const formData = new FormData()
      formData.append('file', testFile.value)
      await api(`/problems/${item.id}/testdata`, { method: 'POST', body: formData })
      testFile.value = null
    }
    ElMessage.success('题目已保存')
    await refresh()
    // 保存完成后跳转到该题目的详情页。
    store.selectedProblem = await api<PublicProblem>(`/problems/${item.id}`)
    navigate('problem-view')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function viewPage() {
  if (!isEdit.value) return
  try {
    store.loading = true
    store.selectedProblem = await api<PublicProblem>(`/problems/${store.editTargetID}`)
    navigate('problem-view')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    store.loading = false
  }
}
</script>

<template>
  <div>
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>{{ isEdit ? '编辑题目' : '新建题目' }}</h2>
        <p>完善题面、资源限制与测试数据</p>
      </div>
      <el-button text @click="goBack('problems')">返回</el-button>
      <el-button v-if="isEdit" @click="viewPage">查看题目</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </div>

    <div v-loading="loading" class="oj-card">
      <div class="oj-card-body">
        <el-form label-position="top">
          <section class="oj-form-section">
            <div class="oj-section-title"><h3>基本信息</h3></div>
            <div class="oj-form-grid">
              <el-form-item label="题目标题" required
                ><el-input v-model.trim="form.title" placeholder="输入题目标题"
              /></el-form-item>
              <el-form-item label="默认分值"
                ><el-input v-model="form.score" placeholder="如 100 / 12.5"
              /></el-form-item>
            </div>
            <div class="row">
              <el-form-item label="浮点误差比较" style="margin-right: 28px"
                ><el-switch v-model="form.compare.floating_point"
              /></el-form-item>
              <el-form-item v-if="form.compare.floating_point" label="允许误差 Epsilon"
                ><el-input-number
                  v-model="form.compare.epsilon"
                  :min="0"
                  :step="0.0001"
                  :precision="6"
              /></el-form-item>
            </div>
          </section>

          <section class="oj-form-section">
            <div class="oj-section-title">
              <h3>资源限制</h3>
              <span>按语言分别设置</span>
            </div>
            <div class="oj-resource-grid">
              <div
                v-for="(limit, language) in form.resource_limits"
                :key="language"
                class="oj-resource-item"
              >
                <strong>{{ languageLabel[language] ?? language }}</strong>
                <el-form-item label="时间限制（ms）"
                  ><el-input-number
                    v-model="limit.time_limit_ms"
                    :min="1"
                    :step="100"
                    style="width: 100%"
                /></el-form-item>
                <el-form-item label="内存限制（MB）"
                  ><el-input-number
                    v-model="limit.memory_limit_mb"
                    :min="1"
                    :step="32"
                    style="width: 100%"
                /></el-form-item>
              </div>
            </div>
          </section>

          <section class="oj-form-section">
            <div class="oj-section-title">
              <h3>题面</h3>
              <span>支持 Markdown 与数学公式</span>
            </div>
            <div class="oj-editor-split">
              <el-form-item label="Markdown 编辑"
                ><el-input
                  v-model="form.statement_markdown"
                  type="textarea"
                  class="oj-code-input"
                  :rows="22"
                  spellcheck="false"
                  placeholder="输入题目描述、输入输出格式及样例…"
              /></el-form-item>
              <div>
                <h4 class="muted">实时预览</h4>
                <div class="oj-preview"><Markdown :markdown="form.statement_markdown" /></div>
              </div>
            </div>
          </section>

          <section class="oj-form-section">
            <div class="oj-section-title">
              <h3>测试数据</h3>
              <span>可选，可在保存后补充</span>
            </div>
            <el-alert
              type="info"
              :closable="false"
              show-icon
              :title="
                isEdit && store.selectedEditor?.testdata_ready
                  ? `已有 ${store.selectedEditor.test_case_count} 个测试点，未选择新文件时保留现有数据`
                  : '尚未上传测试数据，不影响保存题目'
              "
            />
            <div class="row" style="margin-top: 16px">
              <el-upload
                :auto-upload="false"
                :limit="1"
                accept=".zip"
                :on-change="(file: any) => (testFile = file.raw)"
                :on-remove="() => (testFile = null)"
                ><el-button
                  ><el-icon><Upload /></el-icon>选择 ZIP</el-button
                ></el-upload
              >
              <span class="small muted">{{ testFile ? testFile.name : '未选择新文件' }}</span>
            </div>
            <p class="oj-help">
              ZIP 需包含从 1 开始连续配对的 <span class="mono">N.in</span> /
              <span class="mono">N.out</span>；保存时上传，替换原有测试数据。
            </p>
          </section>
        </el-form>
      </div>
    </div>
    <div class="oj-sticky-actions">
      <span class="action-note">{{
        isEdit ? '正在编辑已有题目' : '保存后可继续编辑并验证题目'
      }}</span>
      <el-button @click="goBack('problems')">取消</el-button
      ><el-button type="primary" :loading="saving" @click="save">保存题目</el-button>
    </div>
  </div>
</template>
