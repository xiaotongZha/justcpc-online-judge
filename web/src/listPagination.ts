import { computed, watch, type ComputedRef } from 'vue'

// 列表状态留在工作台内，查看详情后返回仍保留页码。
export function useListPagination<T>(
  items: ComputedRef<T[]>,
  state: { page: number; pageSize: number },
) {
  watch(
    () => state.pageSize,
    () => {
      state.page = 1
    },
  )
  watch(
    () => items.value.length,
    (total) => {
      state.page = Math.max(1, Math.min(state.page, Math.ceil(total / state.pageSize) || 1))
    },
    { immediate: true },
  )
  return computed(() =>
    items.value.slice((state.page - 1) * state.pageSize, state.page * state.pageSize),
  )
}
