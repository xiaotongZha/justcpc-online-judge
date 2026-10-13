<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'
import { store, goBack, navigate, refresh, type Submission } from '../store'
import Markdown from '../components/Markdown.vue'
import LanguageTag from '../components/LanguageTag.vue'

const sourceForm = ref({ language: 'cpp', source: '' })
const submitting = ref(false)

const problem = computed(() => store.selectedProblem)
const isAdmin = computed(() => store.account?.role === 'admin')

async function removeProblem() {
  if (!problem.value) return
  try {
    await ElMessageBox.confirm(
      `确定删除题目「${problem.value.title}」？该题的测试数据、提交记录及比赛引用将一并删除，且不可恢复。`,
      '删除题目',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消', confirmButtonClass: 'el-button--danger' },
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
  <div v-if="problem">
    <div class="oj-toolbar">
      <h2>{{ problem.title }}</h2>
      <el-button text type="primary" @click="goBack('problems')">返回</el-button>
      <el-button v-if="problem.can_edit" text type="primary" @click="goEdit">编辑题目</el-button>
      <el-button v-if="problem.can_edit || isAdmin" text type="danger" @click="removeProblem">删除题目</el-button>
    </div>

    <div class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-body">
        <div class="row" style="gap: 14px">
          <el-tag size="small" type="warning" effect="plain">总分 {{ problem.score }}</el-tag>
          <el-tag size="small" :type="problem.accepting_submissions ? 'success' : 'info'">
            {{ problem.accepting_submissions ? '可提交' : '暂无测试数据' }}
          </el-tag>
        </div>

        <el-table :data="Object.entries(problem.resource_limits)" size="small" border style="max-width: 420px; margin-top: 12px">
          <el-table-column label="语言" min-width="110">
            <template #default="{ row }">
              <LanguageTag :language="row[0]" />
            </template>
          </el-table-column>
          <el-table-column label="时限" min-width="110">
            <template #default="{ row }">
              <span class="mono">{{ row[1].time_limit_ms }} ms</span>
            </template>
          </el-table-column>
          <el-table-column label="内存" min-width="110">
            <template #default="{ row }">
              <span class="mono">{{ row[1].memory_limit_mb }} MB</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <div class="oj-card" style="margin-bottom: 16px">
      <div class="oj-card-header"><h3>题目描述</h3></div>
      <div class="oj-card-body"><Markdown :markdown="problem.statement_markdown" /></div>
    </div>

    <div class="oj-card">
      <div class="oj-card-header"><h3>提交代码</h3></div>
      <div class="oj-card-body">
        <el-alert
          v-if="!problem.accepting_submissions"
          type="warning"
          :closable="false"
          title="该题尚未上传测试数据，暂不能提交。"
          style="margin-bottom: 12px"
        />
        <el-form label-position="top">
          <el-form-item label="语言" style="max-width: 200px">
            <el-select v-model="sourceForm.language">
              <el-option label="C++" value="cpp" />
              <el-option label="Java" value="java" />
              <el-option label="Python 3" value="python3" />
            </el-select>
          </el-form-item>
          <el-form-item label="源码">
            <el-input
              v-model="sourceForm.source"
              type="textarea"
              class="oj-source"
              :rows="18"
              spellcheck="false"
              placeholder="在此粘贴代码..."
            />
          </el-form-item>
          <el-button
            type="primary"
            :disabled="!problem.accepting_submissions"
            :loading="submitting"
            @click="submit"
          >
            提交
          </el-button>
        </el-form>
        <p class="muted small" style="margin-bottom: 0">
          提交后可在「提交」页查看判题结果。测试数据与标程不会公开。
        </p>
      </div>
    </div>
  </div>
</template>
