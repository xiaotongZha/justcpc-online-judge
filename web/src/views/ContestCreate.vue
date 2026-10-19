<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Top, Bottom, Delete } from '@element-plus/icons-vue'
import { api } from '../api'
import { store, navigate, refresh } from '../store'
import type { Contest, ProblemSummary } from '../store'
import PdfPreview from '../components/PdfPreview.vue'

const form = reactive({ name: '', description: '' })
const statementFile = ref<File | null>(null)
const rosterFile = ref<File | null>(null)
const chosen = ref<{ problem_id: number; title: string; score: string }[]>([])
const keyword = ref('')
const saving = ref(false)
type RosterEntry = { student_id: string; name: string }
type RosterPreview = { total: number; entries: RosterEntry[] }
const rosterPreview = ref<RosterPreview | null>(null)
const rosterLoading = ref(false)
const rosterError = ref('')
const rosterQuery = ref('')
const rosterPage = ref(1)
const rosterPageSize = 10
let rosterRequest = 0
let rosterAbort: AbortController | null = null
const filteredRoster = computed(() => {
  const query = rosterQuery.value.trim().toLowerCase()
  return (rosterPreview.value?.entries || []).filter(
    (entry) => !query || `${entry.student_id} ${entry.name}`.toLowerCase().includes(query),
  )
})
const rosterRows = computed(() =>
  filteredRoster.value.slice(
    (rosterPage.value - 1) * rosterPageSize,
    rosterPage.value * rosterPageSize,
  ),
)
watch(rosterQuery, () => {
  rosterPage.value = 1
})

const available = computed(() => {
  const key = keyword.value.trim().toLowerCase()
  return store.problems.filter(
    (p) => !key || p.title.toLowerCase().includes(key) || String(p.id).includes(key),
  )
})
const totalScore = computed(() => {
  let sum = 0
  for (const item of chosen.value) {
    const value = Number(item.score)
    if (!Number.isNaN(value)) sum += value
  }
  return String(Math.round(sum * 100) / 100)
})

const letter = (index: number) => String.fromCharCode(65 + index)
const isChosen = (id: number) => chosen.value.some((c) => c.problem_id === id)
const rowClass = ({ row }: { row: ProblemSummary }) =>
  row.accepting_submissions ? 'clickable-row' : ''

function onStatement(file: any) {
  if (!file?.raw) return
  if (!/\.pdf$/i.test(file.raw.name)) return ElMessage.warning('请选择 PDF 文件')
  statementFile.value = file.raw
}
function clearRoster() {
  rosterRequest++
  rosterAbort?.abort()
  rosterAbort = null
  rosterFile.value = null
  rosterPreview.value = null
  rosterLoading.value = false
  rosterError.value = ''
  rosterQuery.value = ''
  rosterPage.value = 1
}
async function onRoster(file: any) {
  if (!file?.raw) return
  clearRoster()
  rosterFile.value = file.raw
  rosterLoading.value = true
  const request = rosterRequest
  const controller = new AbortController()
  rosterAbort = controller
  const data = new FormData()
  data.append('roster', file.raw)
  try {
    const preview = await api<RosterPreview>('/contest-roster-preview', {
      method: 'POST',
      body: data,
      signal: controller.signal,
    })
    if (request === rosterRequest) rosterPreview.value = preview
  } catch (error) {
    if (request === rosterRequest) {
      rosterError.value = error instanceof Error ? error.message : String(error)
    }
  } finally {
    if (request === rosterRequest) {
      rosterLoading.value = false
      rosterAbort = null
    }
  }
}
onUnmounted(clearRoster)

function addProblem(item: ProblemSummary) {
  if (isChosen(item.id)) return
  if (!item.accepting_submissions) {
    ElMessage.warning('该题尚未上传测试数据，无法加入比赛')
    return
  }
  chosen.value.push({ problem_id: item.id, title: item.title, score: item.score })
}
function removeProblem(index: number) {
  chosen.value.splice(index, 1)
}
function move(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= chosen.value.length) return
  const temp = chosen.value[index]
  chosen.value[index] = chosen.value[target]
  chosen.value[target] = temp
}

async function submit() {
  if (saving.value) return
  if (!form.name.trim()) return ElMessage.warning('请填写比赛名称')
  if (!statementFile.value) return ElMessage.warning('请上传比赛题面 PDF')
  if (!rosterFile.value) return ElMessage.warning('请上传名单文件（xlsx 或 csv）')
  if (rosterLoading.value) return ElMessage.warning('名单正在解析，请稍候')
  if (!rosterPreview.value || rosterError.value)
    return ElMessage.warning('请先上传并通过校验的选手名单')
  if (chosen.value.length === 0) return ElMessage.warning('请至少选择一道题目')
  for (const item of chosen.value) {
    if (Number.isNaN(Number(item.score)) || item.score.trim() === '') {
      return ElMessage.warning(`题目「${item.title}」的分值无效`)
    }
  }
  saving.value = true
  const data = new FormData()
  data.append(
    'payload',
    JSON.stringify({
      name: form.name,
      description: form.description,
      problems: chosen.value.map((c) => ({ problem_id: c.problem_id, score: c.score })),
    }),
  )
  data.append('statement', statementFile.value)
  data.append('roster', rosterFile.value)
  try {
    const contest = await api<Contest>('/contests', { method: 'POST', body: data })
    ElMessage.success('比赛已创建')
    navigate('contest-detail', contest.id)
    await refresh().catch(() => ElMessage.warning('比赛已创建，列表更新失败，请稍后刷新'))
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="create">
    <div class="oj-toolbar">
      <div class="oj-heading-copy">
        <h2>新建比赛</h2>
        <p class="muted small">从既有题目组卷，生成可用于机房考试的考试包。</p>
      </div>
      <el-button text type="primary" @click="navigate('contests')">
        <el-icon><Back /></el-icon>&nbsp;返回比赛列表
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-card-header">
        <span class="step">1</span>
        <h3>基本信息</h3>
      </div>
      <div class="oj-card-body">
        <el-form label-position="top">
          <div class="grid-2">
            <el-form-item label="比赛名称" required>
              <el-input v-model.trim="form.name" placeholder="如：第 19 届 JustCPC" size="large" />
            </el-form-item>
            <el-form-item label="说明">
              <el-input v-model="form.description" placeholder="可选" size="large" />
            </el-form-item>
          </div>
        </el-form>
      </div>
    </div>

    <section class="oj-card" aria-labelledby="statement-card-title">
      <div class="oj-card-header">
        <span class="step">2</span>
        <h3 id="statement-card-title">题面 PDF</h3>
        <el-tag size="small" type="info">必填</el-tag>
      </div>
      <div class="oj-card-body upload-preview-layout">
        <div class="upload-column">
          <h4>上传正式题面</h4>
          <p class="upload-description">
            考试系统不支持逐题题面预览。请上传一份完整、正式的试题
            PDF，供考生下载阅读，并方便赛后留档。
          </p>
          <el-upload
            drag
            :auto-upload="false"
            :disabled="saving"
            :show-file-list="false"
            accept=".pdf"
            :on-change="onStatement"
            class="uploader"
            aria-label="上传题面 PDF"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽 PDF 到此处，或<em>点击选择</em></div>
          </el-upload>
          <div v-if="statementFile" class="selected-file">
            <el-icon><Document /></el-icon
            ><span
              >{{ statementFile.name
              }}<small>{{ (statementFile.size / 1024 / 1024).toFixed(2) }} MB</small></span
            >
            <el-button text size="small" :disabled="saving" @click="statementFile = null"
              >移除</el-button
            >
          </div>
          <p class="upload-footnote">右侧预览仅供试题组核对题号、页码与排版。</p>
        </div>
        <PdfPreview :file="statementFile" />
      </div>
    </section>

    <section class="oj-card" aria-labelledby="roster-card-title">
      <div class="oj-card-header">
        <span class="step">3</span>
        <h3 id="roster-card-title">选手名单</h3>
        <el-tag size="small" type="info">必填</el-tag>
      </div>
      <div class="oj-card-body upload-preview-layout">
        <div class="upload-column">
          <h4>上传名单</h4>
          <p class="upload-description">
            支持
            <strong>CSV 或 XLSX</strong
            >。<strong>必须严格遵守格式模板</strong>，请下载模板填写后再上传。
          </p>
          <div class="templates">
            <a href="/api/contest-roster-example.csv" download="roster-example.csv"
              ><el-icon><Download /></el-icon>CSV 格式模板</a
            ><a href="/api/contest-roster-example.xlsx" download="roster-example.xlsx"
              ><el-icon><Download /></el-icon>XLSX 格式模板</a
            >
          </div>
          <el-upload
            drag
            :auto-upload="false"
            :disabled="saving"
            :show-file-list="false"
            accept=".xlsx,.csv"
            :on-change="onRoster"
            class="uploader"
            aria-label="上传选手名单"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽 CSV / XLSX 到此处，或<em>点击选择</em></div>
          </el-upload>
          <div v-if="rosterFile" class="selected-file">
            <el-icon><Document /></el-icon><span>{{ rosterFile.name }}</span
            ><el-button text size="small" :disabled="saving" @click="clearRoster">移除</el-button>
          </div>
          <p class="upload-footnote">
            表头为
            student_id、name、password，三列均必填，每位选手都要设置密码。学号和密码使用文本格式，保留前导零。
          </p>
          <pre class="roster-example">
student_id,name,password
20260001,张三,pw0001
20260002,李四,pw0002</pre
          >
        </div>
        <section class="roster-preview" aria-label="选手名单预览">
          <div v-if="!rosterFile" class="roster-empty">
            <el-icon><UserFilled /></el-icon>
            <h3>上传后在这里核对名单</h3>
            <p>上传后显示学号、姓名和人数，便于核对。</p>
          </div>
          <p v-else-if="rosterLoading" class="muted" role="status">正在解析并校验选手名单…</p>
          <el-alert
            v-else-if="rosterError"
            :title="rosterError"
            type="error"
            :closable="false"
            show-icon
          />
          <template v-else-if="rosterPreview">
            <div class="roster-preview-header">
              <div>
                <h3>
                  选手名单预览
                  <el-tag size="small" type="success">共 {{ rosterPreview.total }} 人</el-tag>
                </h3>
                <p class="small muted">
                  请核对学号、姓名与人数。所有选手均已填写密码，预览中不展示密码。
                </p>
              </div>
              <el-input
                v-model="rosterQuery"
                placeholder="查找学号或姓名"
                aria-label="查找预览选手"
                clearable
                class="roster-search"
              />
            </div>
            <el-table
              :data="rosterRows"
              size="small"
              stripe
              max-height="320"
              empty-text="没有匹配的选手"
            >
              <el-table-column label="序号" width="65">
                <template #default="{ $index }">{{
                  (rosterPage - 1) * rosterPageSize + $index + 1
                }}</template>
              </el-table-column>
              <el-table-column prop="student_id" label="学号" min-width="150" />
              <el-table-column prop="name" label="姓名" min-width="120" />
              <el-table-column label="初始密码" width="100">
                <template #default>已填写</template>
              </el-table-column>
            </el-table>
            <el-pagination
              v-model:current-page="rosterPage"
              :page-size="rosterPageSize"
              :total="filteredRoster.length"
              layout="total, prev, pager, next"
              small
              class="roster-pagination"
            />
          </template>
        </section>
      </div>
    </section>

    <div class="oj-card">
      <div class="oj-card-header">
        <span class="step">4</span>
        <h3>组卷：选择题目并设置分值</h3>
        <span class="spacer" />
        <el-tag type="info" size="small" effect="light"
          >已选 {{ chosen.length }} 题 · 总分 {{ totalScore }}</el-tag
        >
      </div>
      <div class="oj-card-body picker">
        <div class="picker-panel">
          <div class="panel-title">
            <span>可选题目</span>
            <el-input
              v-model="keyword"
              size="small"
              placeholder="搜索标题或 ID"
              clearable
              style="width: 170px"
            >
              <template #prefix
                ><el-icon><Search /></el-icon
              ></template>
            </el-input>
          </div>
          <el-table
            :data="available"
            size="small"
            height="340"
            stripe
            :row-class-name="rowClass"
            @row-click="addProblem"
          >
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="title" label="标题" min-width="140" show-overflow-tooltip />
            <el-table-column label="数据" width="76">
              <template #default="{ row }">
                <el-tag
                  :type="row.accepting_submissions ? 'success' : 'info'"
                  size="small"
                  effect="light"
                >
                  {{ row.accepting_submissions ? '有' : '无' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="" width="86">
              <template #default="{ row }">
                <el-button
                  size="small"
                  text
                  type="primary"
                  :disabled="isChosen(row.id) || !row.accepting_submissions"
                  @click.stop="addProblem(row)"
                >
                  {{ isChosen(row.id) ? '已选' : '加入' }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="picker-panel">
          <div class="panel-title">
            <span>已选题目（顺序即 A、B、C…）</span>
            <el-button v-if="chosen.length" size="small" text type="danger" @click="chosen = []"
              >清空</el-button
            >
          </div>
          <div v-if="chosen.length" class="chosen-list">
            <div v-for="(item, index) in chosen" :key="item.problem_id" class="chosen-item">
              <span class="label-badge">{{ letter(index) }}</span>
              <div class="chosen-title">
                <div class="ellipsis">{{ item.title }}</div>
                <div class="small muted">题目 ID {{ item.problem_id }}</div>
              </div>
              <el-input v-model="item.score" size="small" class="score-input">
                <template #suffix>分</template>
              </el-input>
              <el-button-group>
                <el-button
                  size="small"
                  :icon="Top"
                  aria-label="上移题目"
                  title="上移题目"
                  :disabled="index === 0"
                  @click="move(index, -1)"
                />
                <el-button
                  size="small"
                  :icon="Bottom"
                  aria-label="下移题目"
                  title="下移题目"
                  :disabled="index === chosen.length - 1"
                  @click="move(index, 1)"
                />
                <el-button
                  size="small"
                  :icon="Delete"
                  aria-label="移除题目"
                  title="移除题目"
                  @click="removeProblem(index)"
                />
              </el-button-group>
            </div>
          </div>
          <el-empty v-else description="还没有选择题目" :image-size="90" />
        </div>
      </div>
    </div>

    <div class="oj-sticky-actions">
      <span class="action-note">共 {{ chosen.length }} 题 · 总分 {{ totalScore }}</span>
      <el-button @click="navigate('contests')">取消</el-button>
      <el-button
        type="primary"
        :loading="saving"
        :disabled="rosterLoading || !rosterPreview || !!rosterError"
        @click="submit"
        >创建比赛</el-button
      >
    </div>
  </div>
</template>

<style scoped>
.grid-2,
.upload-preview-layout,
.picker {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}
.upload-preview-layout {
  align-items: start;
}
.upload-column {
  min-width: 0;
}
.upload-column h4 {
  font-size: 15px;
  margin: 0 0 10px;
}
.upload-description {
  color: var(--oj-text-secondary);
  font-size: 13px;
  line-height: 1.9;
  margin: 0 0 18px;
}
.upload-description strong {
  color: var(--oj-text);
  font-weight: 600;
}
.upload-footnote {
  font-size: 12px;
  line-height: 1.8;
  color: var(--oj-text-secondary);
  margin: 14px 0 0;
  overflow-wrap: anywhere;
}
.selected-file {
  display: flex;
  align-items: center;
  gap: 10px;
  border: 1px solid var(--oj-border);
  border-radius: 7px;
  padding: 10px 12px;
  margin-top: 12px;
  font-size: 12px;
}
.selected-file > .el-icon {
  color: var(--oj-primary);
  font-size: 20px;
  flex: none;
}
.selected-file > span {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.selected-file small {
  display: block;
  color: var(--oj-text-secondary);
  margin-top: 3px;
}
.roster-empty {
  min-height: 340px;
  display: flex;
  justify-content: center;
  align-items: center;
  flex-direction: column;
  gap: 12px;
  text-align: center;
  color: var(--oj-text-secondary);
  padding: 30px;
}
.roster-empty > .el-icon {
  font-size: 36px;
  color: #8a98af;
}
.roster-empty h3 {
  font-size: 14px;
  color: var(--oj-text);
  font-weight: 500;
  margin: 0;
}
.roster-empty p {
  font-size: 12px;
  line-height: 1.8;
  margin: 0;
}
.roster-example {
  padding: 12px;
  background: var(--oj-bg);
  border: 1px solid var(--oj-border);
  border-radius: 6px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.7;
}

.roster-preview {
  padding: 18px 22px 22px;
  border: 1px solid var(--oj-border);
  border-radius: 10px;
  min-width: 0;
}
.roster-preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 14px;
}
.roster-preview-header h3 {
  margin: 0 0 6px;
}
.roster-search {
  width: min(100%, 240px);
}
.roster-pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
.spacer {
  flex: 1;
}
.step {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--el-color-primary);
  color: #fff;
  display: inline-grid;
  place-items: center;
  font-size: 12px;
  font-weight: 700;
}
.grid-2 :deep(.el-form-item) {
  margin-bottom: 8px;
}
.uploader {
  width: 100%;
}
.uploader :deep(.el-upload-dragger) {
  padding: 18px 10px;
}
.templates {
  margin: 0 0 18px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.templates a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border: 1px solid var(--oj-border);
  border-radius: 6px;
  color: var(--oj-primary);
  font-size: 12px;
  text-decoration: none;
}
.picker-panel {
  min-width: 0;
}
.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
  font-size: 13px;
  color: var(--oj-text-secondary);
}
.chosen-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 340px;
  overflow: auto;
}
.chosen-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--oj-border);
  border-radius: 8px;
  background: #fff;
  transition:
    border-color 0.15s,
    box-shadow 0.15s;
}
.chosen-item:hover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: var(--oj-shadow);
}
.label-badge {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  background: var(--el-color-primary);
  color: #fff;
  display: grid;
  place-items: center;
  font-weight: 700;
  font-size: 12px;
  flex: none;
}
.chosen-title {
  flex: 1;
  min-width: 0;
}
.ellipsis {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.score-input {
  width: 104px;
  flex: none;
}
:deep(.clickable-row) {
  cursor: pointer;
}
@media (max-width: 1100px) {
  .upload-preview-layout,
  .grid-2,
  .picker {
    grid-template-columns: 1fr;
  }
}
</style>
