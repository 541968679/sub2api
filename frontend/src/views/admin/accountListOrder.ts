/**
 * Move one account to another index within the current page list.
 * Used for page-local drag reorder (admin list_order).
 */
export function moveAccountInPageList<T extends { id: number }>(
  list: T[],
  fromId: number,
  toId: number
): T[] {
  if (fromId === toId || list.length < 2) {
    return list
  }
  const fromIndex = list.findIndex((item) => item.id === fromId)
  const toIndex = list.findIndex((item) => item.id === toId)
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
    return list
  }
  const next = list.slice()
  const [item] = next.splice(fromIndex, 1)
  next.splice(toIndex, 0, item)
  return next
}
