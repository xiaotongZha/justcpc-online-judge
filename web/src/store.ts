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
  problems: [] as ProblemSummary[],
  submissions: [] as Submission[],
  contests: [] as Contest[],
  accounts: [] as AdminAccount[],
  selectedProblem: null as PublicProblem | null,
  selectedEditor: null as EditorProblem | null,
  editTargetID: 0 as number,
  selectedSubmission: null as Submission | null,
  selectedContest: null as Contest | null,
  navigation: [] as string[],
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
  store.account = null
  store.navigation = []
  store.tab = 'problems'
  store.problems = []
  store.submissions = []
  store.contests = []
  store.accounts = []
  store.selectedProblem = null
  store.selectedEditor = null
  store.selectedSubmission = null
  store.selectedContest = null
  store.editTargetID = 0
}

export function navigate(next: string) {
  if (store.tab === next) return
  history.replaceState({ ...history.state, scrollY: window.scrollY }, '')
  store.navigation.push(store.tab)
  store.tab = next
  history.pushState({ justcpc: true, tab: next }, '', `#${next}`)
  window.scrollTo({ top: 0, behavior: 'instant' })
}

export function goBack(fallback = 'problems') {
  if (store.navigation.length > 0) {
    history.back()
    return
  }
  if (store.tab !== fallback) navigate(fallback)
}

export function restoreHistory(next: string) {
  if (!knownTabs.has(next)) return
  if (store.navigation.at(-1) === next) {
    store.navigation.pop()
  } else {
    store.navigation.push(store.tab)
  }
  store.tab = next
  requestAnimationFrame(() =>
    window.scrollTo({ top: history.state?.scrollY ?? 0, behavior: 'instant' }),
  )
}
