<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from './api'
import { store, knownTabs, refresh, navigate, goBack, restoreHistory, resetSession } from './store'
import Users from './views/Users.vue'
import Problems from './views/Problems.vue'
import ProblemView from './views/ProblemView.vue'
import ProblemEdit from './views/ProblemEdit.vue'
import Submissions from './views/Submissions.vue'
import SubmissionDetail from './views/SubmissionDetail.vue'
import Contests from './views/Contests.vue'
import ContestCreate from './views/ContestCreate.vue'
import ContestDetail from './views/ContestDetail.vue'
import Admin from './views/Admin.vue'

const loginForm = ref({ username: 'admin', password: '' })
const loggingIn = ref(false)

const navTabs = computed(() => {
  const tabs = [
    { key: 'users', label: '用户' },
    { key: 'problems', label: '题目' },
    { key: 'submissions', label: '提交' },
    { key: 'contests', label: '比赛' },
  ]
  if (store.account?.role === 'admin') tabs.push({ key: 'admin', label: '管理员' })
  return tabs
})

const isActive = (key: string) =>
  key === 'problems'
    ? store.tab.startsWith('problem')
    : key === 'submissions'
      ? store.tab.startsWith('submission')
      : key === 'contests'
        ? store.tab.startsWith('contest')
        : store.tab === key

const canGoBack = computed(() => store.navigation.length > 0)

async function login() {
  loggingIn.value = true
  try {
    const account = await api<{ id: number; username: string; role: 'admin' | 'user' }>('/login', {
      method: 'POST',
      body: JSON.stringify(loginForm.value),
    })
    resetSession()
    store.account = account
    history.replaceState({ justcpc: true, tab: 'problems' }, '', '#problems')
    loginForm.value.password = ''
    await refresh()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
  } finally {
    loggingIn.value = false
  }
}

async function logout() {
  try {
    await api('/logout', { method: 'POST' })
  } catch {
    /* ignore */
  }
  resetSession()
  history.replaceState({ justcpc: true, tab: 'problems' }, '', '#problems')
}

onMounted(async () => {
  const hash = location.hash.slice(1)
  if (knownTabs.has(hash)) store.tab = hash
  history.replaceState({ justcpc: true, tab: store.tab }, '', `#${store.tab}`)
  window.addEventListener('popstate', (event: PopStateEvent) => {
    const next = event.state?.justcpc && typeof event.state.tab === 'string' ? event.state.tab : ''
    restoreHistory(next)
  })
  try {
    store.account = await api<{ id: number; username: string; role: 'admin' | 'user' }>('/me')
    await refresh()
  } catch {
    /* not logged in */
  }
})
</script>

<template>
  <div v-if="!store.account" class="login">
    <div class="login-card">
      <div class="brand">
        <span class="logo">J</span>
      </div>
      <h1>JustCPC Online Judge</h1>
      <p class="hint">命题、自测与赛后批量测评工作台</p>
      <el-form @submit.prevent="login">
        <el-form-item>
          <el-input v-model="loginForm.username" placeholder="用户名" size="large" autocomplete="username">
            <template #prefix><el-icon><User /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            size="large"
            show-password
            autocomplete="current-password"
            @keyup.enter="login"
          >
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loggingIn" native-type="submit">
          登录
        </el-button>
      </el-form>
    </div>
  </div>

  <div v-else class="shell">
    <header class="oj-header">
      <div class="brand">
        <span class="logo">J</span>
        <span>JustCPC OJ</span>
      </div>
      <nav class="oj-nav">
        <button
          v-for="t in navTabs"
          :key="t.key"
          :class="{ active: isActive(t.key) }"
          @click="navigate(t.key)"
        >
          {{ t.label }}
        </button>
      </nav>
      <div class="user">
        <el-icon><UserFilled /></el-icon>
        <span>{{ store.account.username }}</span>
        <el-button text size="small" style="color: var(--oj-header-fg)" @click="logout">退出</el-button>
      </div>
    </header>

    <main class="oj-page">
      <Users v-if="store.tab === 'users'" />
      <Problems v-else-if="store.tab === 'problems'" />
      <ProblemView v-else-if="store.tab === 'problem-view'" />
      <ProblemEdit v-else-if="store.tab === 'problem-edit'" />
      <Submissions v-else-if="store.tab === 'submissions'" />
      <SubmissionDetail v-else-if="store.tab === 'submission-detail'" />
      <Contests v-else-if="store.tab === 'contests'" />
      <ContestCreate v-else-if="store.tab === 'contest-create'" />
      <ContestDetail v-else-if="store.tab === 'contest-detail'" />
      <Admin v-else-if="store.tab === 'admin'" />
    </main>
  </div>
</template>
