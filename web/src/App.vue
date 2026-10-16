<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from './api'
import { store, knownTabs, refresh, navigate, restoreHistory, resetSession } from './store'
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
    { key: 'problems', label: '题目库', icon: 'Document', group: 'workspace' },
    { key: 'contests', label: '比赛管理', icon: 'Trophy', group: 'workspace' },
    { key: 'submissions', label: '提交记录', icon: 'Tickets', group: 'workspace' },
  ]
  if (store.account?.role === 'admin')
    tabs.push({ key: 'admin', label: '判题队列', icon: 'Monitor', group: 'system' })
  tabs.push({
    key: 'users',
    label: store.account?.role === 'admin' ? '账户管理' : '我的账户',
    icon: 'User',
    group: 'system',
  })
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

const activeNav = computed(() => navTabs.value.find((tab) => isActive(tab.key)))
const detailLabels: Record<string, string> = {
  'problem-view': '题目详情',
  'submission-detail': '提交详情',
  'contest-create': '新建比赛',
  'contest-detail': '比赛详情',
}
const detailLabel = computed(() =>
  store.tab === 'problem-edit'
    ? store.editTargetID
      ? '编辑题目'
      : '新建题目'
    : detailLabels[store.tab],
)

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
        <span class="logo">J</span
        ><span>JustCPC<span class="brand-caption">试题组工作台</span></span>
      </div>
      <h1>JustCPC Online Judge</h1>
      <p class="hint">命题、自测与赛后批量测评工作台</p>
      <el-form label-position="top" @submit.prevent="login">
        <el-form-item label="用户名">
          <el-input
            v-model="loginForm.username"
            placeholder="用户名"
            size="large"
            autocomplete="username"
          >
            <template #prefix
              ><el-icon><User /></el-icon
            ></template>
          </el-input>
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            size="large"
            show-password
            autocomplete="current-password"
          >
            <template #prefix
              ><el-icon><Lock /></el-icon
            ></template>
          </el-input>
        </el-form-item>
        <el-button
          type="primary"
          size="large"
          style="width: 100%"
          :loading="loggingIn"
          native-type="submit"
        >
          登录
        </el-button>
      </el-form>
    </div>
  </div>

  <div v-else class="shell">
    <aside class="oj-sidebar">
      <div class="brand">
        <span class="logo">J</span>
        <span>JustCPC<span class="brand-caption">试题组工作台</span></span>
      </div>
      <template
        v-for="group in [
          { key: 'workspace', label: '工作区' },
          { key: 'system', label: '系统' },
        ]"
        :key="group.key"
      >
        <div class="oj-nav-label">{{ group.label }}</div>
        <nav class="oj-nav" :aria-label="group.label">
          <button
            v-for="t in navTabs.filter((tab) => tab.group === group.key)"
            :key="t.key"
            :class="{ active: isActive(t.key) }"
            :aria-current="isActive(t.key) ? 'page' : undefined"
            @click="navigate(t.key)"
          >
            <el-icon><component :is="t.icon" /></el-icon><span>{{ t.label }}</span>
          </button>
        </nav>
      </template>
      <div class="oj-sidebar-account">
        <el-avatar :size="34">{{ store.account.username.slice(0, 1).toUpperCase() }}</el-avatar>
        <div class="oj-account-copy">
          <strong>{{ store.account.username }}</strong
          ><span>{{ store.account.role === 'admin' ? '管理员' : '普通用户' }}</span>
        </div>
        <el-tooltip content="退出登录" placement="top"
          ><el-button text circle aria-label="退出登录" @click="logout"
            ><el-icon><SwitchButton /></el-icon></el-button
        ></el-tooltip>
      </div>
    </aside>

    <div class="oj-workspace">
      <header class="oj-header">
        <nav class="oj-breadcrumb" aria-label="当前位置">
          <span>{{ activeNav?.group === 'system' ? '系统' : '工作区' }}</span
          ><el-icon><ArrowRight /></el-icon>
          <button v-if="detailLabel" @click="navigate(activeNav?.key ?? 'problems')">
            {{ activeNav?.label }}
          </button>
          <strong v-else>{{ activeNav?.label }}</strong>
          <template v-if="detailLabel"
            ><el-icon><ArrowRight /></el-icon><strong>{{ detailLabel }}</strong></template
          >
        </nav>
        <span class="oj-header-caption">JustCPC Online Judge</span>
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
  </div>
</template>
