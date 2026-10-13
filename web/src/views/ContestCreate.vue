<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Top, Bottom, Delete } from '@element-plus/icons-vue'
import { api } from '../api'
import { store, navigate, refresh } from '../store'
import type { Contest, ProblemSummary } from '../store'

const form = reactive({ name: '', description: '' })
const statementFile = ref<File | null>(null)
const rosterFile = ref<File | null>(null)
const chosen = ref<{ problem_id: number; title: string; score: string }[]>([])
const keyword = ref('')
const saving = ref(false)

const available = computed(() => {
  const key = keyword.value.trim().toLowerCase()
  return store.problems.filter((p) => !key || p.title.toLowerCase().includes(key) || String(p.id).includes(key))
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
const rowClass = ({ row }: { row: ProblemSummary }) => (row.accepting_submissions ? 'clickable-row' : '')

function onStatement(file: any) {
  if (file?.raw) statementFile.value = file.raw
}
function onRoster(file: any) {
  if (file?.raw) rosterFile.value = file.raw
}

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
  if (!form.name.trim()) return ElMessage.warning('请填写比赛名称')
  if (!statementFile.value) return ElMessage.warning('请上传比赛题面 PDF')
  if (!rosterFile.value) return ElMessage.warning('请上传名单文件（xlsx 或 csv）')
  if (chosen.value.length === 0) return ElMessage.warning('请至少选择一道题目')
  for (const item of chosen.value) {
    if (Number.isNaN(Number(item.score)) || item.score.trim() === '') {
      return ElMessage.warning(`题目「${item.title}」的分值无效`)
    }
  }
  saving.value = true
  const data = new FormData()
  data.append('payload', JSON.stringify({
    name: form.name,
    description: form.description,
    problems: chosen.value.map((c) => ({ problem_id: c.problem_id, score: c.score })),
  }))
  data.append('statement', statementFile.value)
  data.append('roster', rosterFile.value)
  try {
    await api<Contest>('/contests', { method: 'POST', body: data })
    ElMessage.success('比赛已创建')
    await refresh()
    navigate('contests')
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
    <div class="create-head">
      <div>
        <h2>新建比赛</h2>
        <p class="muted small">从既有题目组卷，生成可用于机房考试的考试包。</p>
      </div>
      <span class="spacer" />
      <el-button text type="primary" @click="navigate('contests')">
        <el-icon><Back /></el-icon>&nbsp;返回比赛列表
      </el-button>
    </div>

    <div class="oj-card">
      <div class="oj-card-header"><span class="step">1</span><h3>基本信息</h3></div>
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

    <div class="oj-card">
      <div class="oj-card-header"><span class="step">2</span><h3>比赛题面与选手名单</h3></div>
      <div class="oj-card-body grid-2">
        <div>
          <div class="field-label">比赛题面 PDF <em>*</em></div>
          <el-upload drag :auto-upload="false" :show-file-list="false" accept=".pdf" :on-change="onStatement" class="uploader">
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽 PDF 到此处，或<em>点击选择</em></div>
          </el-upload>
          <div v-if="statementFile" class="file-state">
            <el-icon><CircleCheckFilled /></el-icon>{{ statementFile.name }}
          </div>
        </div>
        <div>
          <div class="field-label">选手名单（xlsx / csv：student_id,name[,password]） <em>*</em></div>
          <el-upload drag :auto-upload="false" :show-file-list="false" accept=".xlsx,.csv,.txt" :on-change="onRoster" class="uploader">
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽名单到此处，或<em>点击选择</em></div>
          </el-upload>
          <div v-if="rosterFile" class="file-state">
            <el-icon><CircleCheckFilled /></el-icon>{{ rosterFile.name }}
          </div>
          <div class="templates">
            <span class="muted small">示例模板（含表头与两行示例数据）：</span>
            <a href="/api/contest-roster-example.csv" download="roster-example.csv"><el-icon><Download /></el-icon>CSV</a>
            <a href="/api/contest-roster-example.xlsx" download="roster-example.xlsx"><el-icon><Download /></el-icon>XLSX</a>
          </div>
        </div>
      </div>
    </div>

    <div class="oj-card">
      <div class="oj-card-header">
        <span class="step">3</span><h3>组卷：选择题目并设置分值</h3>
        <span class="spacer" />
        <el-tag type="info" size="small" effect="light">已选 {{ chosen.length }} 题 · 总分 {{ totalScore }}</el-tag>
      </div>
      <div class="oj-card-body picker">
        <div class="picker-panel">
          <div class="panel-title">
            <span>可选题目</span>
            <el-input v-model="keyword" size="small" placeholder="搜索标题或 ID" clearable style="width: 170px">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
          </div>
          <el-table :data="available" size="small" height="340" stripe :row-class-name="rowClass" @row-click="addProblem">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="title" label="标题" min-width="140" show-overflow-tooltip />
            <el-table-column label="数据" width="76">
              <template #default="{ row }">
                <el-tag :type="row.accepting_submissions ? 'success' : 'info'" size="small" effect="light">
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
            <el-button v-if="chosen.length" size="small" text type="danger" @click="chosen = []">清空</el-button>
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
                <el-button size="small" :icon="Top" :disabled="index === 0" @click="move(index, -1)" />
                <el-button size="small" :icon="Bottom" :disabled="index === chosen.length - 1" @click="move(index, 1)" />
                <el-button size="small" :icon="Delete" @click="removeProblem(index)" />
              </el-button-group>
            </div>
          </div>
          <el-empty v-else description="还没有选择题目" :image-size="90" />
        </div>
      </div>
    </div>

    <div class="create-foot">
      <span class="muted small">共 {{ chosen.length }} 题 · 总分 {{ totalScore }}</span>
      <span class="spacer" />
      <el-button @click="navigate('contests')">取消</el-button>
      <el-button type="primary" size="large" :loading="saving" @click="submit">创建比赛</el-button>
    </div>
  </div>
</template>

<style scoped>
.create-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}
.create-head h2 {
  margin: 0;
  font-size: 20px;
}
.create-head p {
  margin: 4px 0 0;
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
.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}
.grid-2 :deep(.el-form-item) {
  margin-bottom: 8px;
}
.field-label {
  font-size: 13px;
  color: var(--oj-text-secondary);
  margin-bottom: 6px;
}
.field-label em {
  color: var(--el-color-danger);
  font-style: normal;
}
.uploader {
  width: 100%;
}
.uploader :deep(.el-upload-dragger) {
  padding: 18px 10px;
}
.file-state {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  color: var(--el-color-success);
  font-size: 13px;
  word-break: break-all;
}
.templates {
  margin-top: 10px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.templates a {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}
.picker {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
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
  transition: border-color 0.15s, box-shadow 0.15s;
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
.create-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 4px;
  padding-top: 12px;
}
:deep(.clickable-row) {
  cursor: pointer;
}
@media (max-width: 900px) {
  .grid-2,
  .picker {
    grid-template-columns: 1fr;
  }
}
</style>
