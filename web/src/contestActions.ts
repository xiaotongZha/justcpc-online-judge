import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from './api'
import { navigate, store, type Contest } from './store'

export const deletingContestID = ref<number | null>(null)

export async function removeContest(contest: Contest): Promise<boolean> {
  if (store.account?.role !== 'admin' || deletingContestID.value !== null) return false
  deletingContestID.value = contest.id
  try {
    try {
      await ElMessageBox.confirm(
        `确定删除比赛“${contest.name}”？本场考生、提交源码和评测记录将一并删除，无法恢复。题库中的原题会保留。`,
        '删除比赛',
        {
          type: 'warning',
          confirmButtonText: '删除比赛',
          cancelButtonText: '取消',
          confirmButtonClass: 'el-button--danger',
        },
      )
    } catch {
      return false
    }
    await api<void>(`/contests/${contest.id}`, { method: 'DELETE' })
    store.contests = store.contests.filter((item) => item.id !== contest.id)
    if (store.selectedContest?.id === contest.id) {
      store.selectedContest = null
      if (store.tab === 'contest-detail') navigate('contests')
    }
    ElMessage.success('比赛已删除')
    return true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error))
    return false
  } finally {
    deletingContestID.value = null
  }
}
