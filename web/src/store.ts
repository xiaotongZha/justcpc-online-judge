import { reactive } from 'vue'
import { api } from './api'

export type Account = { id: number; username: string; role: 'admin' | 'user' }
export type AdminAccount = Account & { password: string }
export type Limits = Record<string, { time_limit_ms: number; memory_limit_mb: number }>
export type Compare = { floating_point: boolean; epsilon: number }

export type ProblemSummary = {
  id: number
  title: string
  score: string
  accepting_submissions: boolean
  can_edit: boolean
}
export type PublicProblem = ProblemSummary & {
  resource_limits: Limits
  statement_markdown: string
}
export type EditorProblem = {
  id: number
  title: string
  statement_markdown: string
  score: string
  resource_limits: Limits
  compare: Compare
  testdata_ready: boolean
  test_case_count: number
  created_at: string
  updated_at: string
}
export type CaseResult = {
  id: string
  verdict: string
  time_ms: number
  memory: number
  stderr?: string
  error?: string
  output_path?: string
}
export type Submission = {
  id: number
  problem_id: number
  is_contestant: boolean
  problem_title: string
  submitter: string
  language: string
  status: string
  score: string
  submitted_at: string
  judged_at?: string
  source?: string
  can_download_testdata: boolean
  judge_result_json?: { compile_log?: string; cases?: CaseResult[] }
}
export type Contest = {
  id: number
  name: string
  description: string
  statement_pdf_path?: string
  problem_count: number
  contestant_count: number
  submission_count: number
  created_at: string
}
export type ContestProblem = {
  problem_id: number
  label: string
  title: string
  score: string
  order_index: number
  resource_limits: Limits
}
export type Contestant = { id: number; student_id: string; name: string }
export type ContestDetail = Contest & { problems: ContestProblem[]; contestants: Contestant[] }
export type QueueStatus = {
  id: number
  submission_id: number
  state: 'queued' | 'running' | 'failed'
  attempt: number
  error?: string
}

export const knownTabs = new Set([
  'users',
  'problems',
  'problem-view',
  'problem-edit',
  'submissions',
  'submission-detail',
  'contests',
  'contest-create',
  'contest-detail',
  'admin',
])

export const statusType: Record<string, string> = {
  AC: 'success',
  WA: 'danger',
  CE: 'warning',
  TLE: 'danger',
  MLE: 'danger',
  RE: 'danger',
  OLE: 'danger',
  SYS: 'danger',
  SE: 'danger',
  waiting: 'info',
}

export const languageLabel: Record<string, string> = {
  cpp: 'C++',
  java: 'Java',
  python3: 'Python 3',
}

export const store = reactive({
  account: null as Account | null,
  tab: 'problems',
  loading: false,
  routeID: 0,
  routeKey: '',
  routeLoading: false,
  routeError: '',
  problems: [] as ProblemSummary[],
  problemList: { keyword: '', page: 1, pageSize: 20 },
  contestList: { page: 1, pageSize: 20 },
  submissions: [] as Submission[],
  contests: [] as Contest[],
  accounts: [] as AdminAccount[],
  selectedProblem: null as PublicProblem | null,
  selectedEditor: null as EditorProblem | null,
  editTargetID: 0 as number,
  selectedSubmission: null as Submission | null,
  selectedContest: null as Contest | null,
})

export async function refresh() {
  if (!store.account) return
  const [problems, contests] = await Promise.all([
    api<ProblemSummary[] | null>('/problems'),
    api<Contest[] | null>('/contests'),
  ])
  store.problems = problems ?? []
  store.contests = contests ?? []
  if (store.account.role === 'admin') {
    store.accounts = (await api<AdminAccount[] | null>('/accounts')) ?? []
  }
}

export function resetSession() {
  routeRequest++
  store.routeLoading = false
  store.routeError = ''
  store.routeID = 0
  store.account = null
  store.tab = 'problems'
  store.problems = []
  Object.assign(store.problemList, { keyword: '', page: 1, pageSize: 20 })
  Object.assign(store.contestList, { page: 1, pageSize: 20 })
  store.submissions = []
  store.contests = []
  store.accounts = []
  store.selectedProblem = null
  store.selectedEditor = null
  store.selectedSubmission = null
  store.selectedContest = null
  store.editTargetID = 0
}

type Route = { tab: string; id: number }
const detailTabs = new Set(['problem-view', 'problem-edit', 'submission-detail', 'contest-detail'])

export function routeHash(tab: string, id = 0): string {
  if (tab === 'problem-edit') return id ? `#/problems/${id}/edit` : '#/problems/new'
  if (tab === 'problem-view') return `#/problems/${id}`
  if (tab === 'submission-detail') return `#/submissions/${id}`
  if (tab === 'contest-detail') return `#/contests/${id}`
  return `#/${tab}`
}

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#\/?/, '')
  if (path === 'problems/new') return { tab: 'problem-edit', id: 0 }
  const match = /^(problems|submissions|contests)\/([1-9]\d*)(\/edit)?$/.exec(path)
  if (match && Number.isSafeInteger(Number(match[2]))) {
    const tab =
      match[1] === 'problems'
        ? match[3]
          ? 'problem-edit'
          : 'problem-view'
        : match[1] === 'submissions'
          ? 'submission-detail'
          : 'contest-detail'
    if (!match[3] || match[1] === 'problems') return { tab, id: Number(match[2]) }
  }
  if (knownTabs.has(path) && !detailTabs.has(path)) return { tab: path, id: 0 }
  // 旧版详情链接没有 ID，返回所属列表，避免出现空白页。
  if (path.startsWith('submission')) return { tab: 'submissions', id: 0 }
  if (path.startsWith('contest')) return { tab: 'contests', id: 0 }
  return { tab: 'problems', id: 0 }
}

let routeRequest = 0
export async function loadRoute(hash = location.hash) {
  const request = ++routeRequest
  const route = parseRoute(hash)
  store.routeLoading = true
  store.routeError = ''
  store.tab = route.tab
  store.routeID = route.id
  store.routeKey = routeHash(route.tab, route.id)
  if (location.hash !== store.routeKey) history.replaceState(history.state, '', store.routeKey)
  store.editTargetID = route.tab.startsWith('problem') ? route.id : 0
  store.selectedProblem = null
  store.selectedEditor = null
  store.selectedSubmission = null
  store.selectedContest = null
  try {
    if (route.id) {
      const path = route.tab.startsWith('problem')
        ? `/problems/${route.id}${route.tab === 'problem-edit' ? '/editor' : ''}`
        : route.tab === 'submission-detail'
          ? `/submissions/${route.id}`
          : `/contests/${route.id}`
      const item = await api<PublicProblem | EditorProblem | Submission | ContestDetail>(path)
      if (request !== routeRequest) return
      if (route.tab === 'problem-view') store.selectedProblem = item as PublicProblem
      if (route.tab === 'problem-edit') store.selectedEditor = item as EditorProblem
      if (route.tab === 'submission-detail') store.selectedSubmission = item as Submission
      if (route.tab === 'contest-detail') store.selectedContest = item as ContestDetail
    }
  } catch (error) {
    if (request === routeRequest)
      store.routeError = error instanceof Error ? error.message : String(error)
  } finally {
    if (request === routeRequest) store.routeLoading = false
  }
}

export function navigate(next: string, id?: number) {
  const targetID =
    id ??
    (next === 'problem-view'
      ? store.selectedProblem?.id
      : next === 'problem-edit'
        ? store.editTargetID
        : next === 'submission-detail'
          ? store.selectedSubmission?.id
          : next === 'contest-detail'
            ? store.selectedContest?.id
            : 0) ??
    0
  const hash = routeHash(next, targetID)
  if (location.hash === hash) return
  history.replaceState({ ...history.state, scrollY: window.scrollY }, '')
  const depth = history.state?.justcpc ? Number(history.state.depth) || 0 : 0
  history.pushState({ justcpc: true, depth: depth + 1 }, '', hash)
  void loadRoute(hash)
  window.scrollTo({ top: 0, behavior: 'instant' })
}

export function goBack(fallback = 'problems') {
  if (history.state?.justcpc && history.state.depth > 0) {
    history.back()
    return
  }
  navigate(fallback)
}

export function restoreHistory() {
  void loadRoute().then(() =>
    requestAnimationFrame(() =>
      window.scrollTo({ top: history.state?.scrollY ?? 0, behavior: 'instant' }),
    ),
  )
}
