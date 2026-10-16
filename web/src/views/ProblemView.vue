<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'
import { store, goBack, navigate, refresh, type Submission } from '../store'
import Markdown from '../components/Markdown.vue'
import LanguageTag from '../components/LanguageTag.vue'

const sourceForm = ref({ language: 'cpp', source: '' })
const submitting = ref(false)
const codeSection = ref<HTMLElement | null>(null)
function scrollToCode() {
  codeSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const problem = computed(() => store.selectedProblem)
const isAdmin = computed(() => store.account?.role === 'admin')

async function removeProblem() {
  if (!problem.value) return
  try {
    await ElMessageBox.confirm(
      `确定删除题目「${problem.value.title}」？该题的测试数据、提交记录及比赛引用将一并删除，且不可恢复。`,
      '删除题目',
      {
        type: 'warning',
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        confirmButtonClass: 'el-button--danger',
      },
    )
  } catch {
    return
  }
  try {
    await api(`/problems/${problem.value.id}`, { method: 'DELETE' })
    ElMessage.success('题目已删除')
    await refresh()
    goBack('problems')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  }
}

async function submit() {
  if (!problem.value || !problem.value.accepting_submissions) return
  if (!sourceForm.value.source.trim()) {
    ElMessage.warning('请先填写源码')
    return
  }
  submitting.value = true
  try {
    const item = await api<Submission>(`/problems/${problem.value.id}/submissions`, {
      method: 'POST',
      body: JSON.stringify(sourceForm.value),
    })
    ElMessage.success(`提交 #${item.id} 已入队`)
    store.selectedSubmission = item
    navigate('submission-detail')
    await refresh()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    submitting.value = false
  }
}

function goEdit() {
  if (!problem.value) return
  store.editTargetID = problem.value.id
  navigate('problem-edit')
}
</script>

<template>
  <div v-if="problem" class="oj-document">
    <div class="oj-problem-actions">
      <el-button text @click="goBack('problems')"
        ><el-icon><ArrowLeft /></el-icon>返回题目库</el-button
      >
      <span class="spacer" />
      <el-button v-if="problem.can_edit" @click="goEdit"
        ><el-icon><Edit /></el-icon>编辑题目</el-button
      >
      <el-button v-if="problem.can_edit || isAdmin" text type="danger" @click="removeProblem"
        >删除题目</el-button
      >
      <el-button type="primary" @click="scrollToCode"
        ><el-icon><EditPen /></el-icon>提交代码</el-button
      >
    </div>

    <article class="oj-card">
      <div class="oj-card-body oj-problem-title">
        <span class="oj-table-id">题目 #{{ problem.id }}</span>
        <h1>{{ problem.title }}</h1>
        <div class="row">
          <span class="small muted"
            >总分 <strong class="mono">{{ problem.score }}</strong></span
          >
          <span
            class="oj-data-state"
            :class="problem.accepting_submissions ? 'ready' : 'incomplete'"
            >{{ problem.accepting_submissions ? '可提交' : '暂无测试数据' }}</span
          >
        </div>
      </div>
      <div class="oj-limits" aria-label="各语言资源限制">
        <div v-for="(limit, language) in problem.resource_limits" :key="language" class="oj-limit">
          <LanguageTag :language="language" /><span class="mono">{{ limit.time_limit_ms }} ms</span
          ><span class="mono">{{ limit.memory_limit_mb }} MB</span>
        </div>
      </div>
      <div class="oj-statement-body"><Markdown :markdown="problem.statement_markdown" /></div>
    </article>

    <section ref="codeSection" class="oj-card" style="scroll-margin-top: 84px">
      <div class="oj-card-header">
        <h3>提交代码</h3>
        <span class="small muted">标准输入输出</span>
      </div>
      <div class="oj-card-body">
        <el-alert
          v-if="!problem.accepting_submissions"
          type="warning"
          :closable="false"
          show-icon
          title="该题尚未上传测试数据，暂不能提交。"
          style="margin-bottom: 20px"
        />
        <el-form label-position="top">
          <el-form-item label="语言" style="max-width: 200px">
            <el-select v-model="sourceForm.language">
              <el-option label="C++" value="cpp" /><el-option label="Java" value="java" /><el-option
                label="Python 3"
                value="python3"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="源码">
            <el-input
              v-model="sourceForm.source"
              type="textarea"
              class="oj-source"
              :rows="18"
              spellcheck="false"
              placeholder="在此粘贴代码…"
            />
          </el-form-item>
          <div class="row">
            <el-button
              type="primary"
              :disabled="!problem.accepting_submissions"
              :loading="submitting"
              @click="submit"
              >提交评测</el-button
            >
            <span class="small muted">提交后自动跳转至评测结果</span>
          </div>
        </el-form>
      </div>
    </section>
  </div>
</template>
