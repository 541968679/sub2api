import { describe, expect, it } from 'vitest'
import {
  captureOpsErrorDetailReturnTarget,
  nextOpsListVisibility,
  shouldResetOpsListFiltersOnOpen
} from '../opsErrorDetailReturn'

describe('ops error detail return to list', () => {
  it('returns to the source error list and keeps filters', () => {
    const target = captureOpsErrorDetailReturnTarget(true, false)
    expect(target).toBe('errorList')
    expect(shouldResetOpsListFiltersOnOpen(true)).toBe(false)
    expect(nextOpsListVisibility(target)).toEqual({
      errorList: true,
      requestList: false,
      errorDetail: false
    })
  })

  it('returns to the source request list and keeps filters', () => {
    const target = captureOpsErrorDetailReturnTarget(false, true)
    expect(target).toBe('requestList')
    expect(shouldResetOpsListFiltersOnOpen(true)).toBe(false)
    expect(nextOpsListVisibility(target)).toEqual({
      errorList: false,
      requestList: true,
      errorDetail: false
    })
  })

  it('resets filters when the list is opened manually', () => {
    expect(shouldResetOpsListFiltersOnOpen(false)).toBe(true)
  })
})
