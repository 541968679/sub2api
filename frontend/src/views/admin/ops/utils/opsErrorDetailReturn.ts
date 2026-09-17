export type OpsDetailReturnTarget = 'errorList' | 'requestList' | null

export function captureOpsErrorDetailReturnTarget(
  fromErrorList: boolean,
  fromRequestList: boolean
): OpsDetailReturnTarget {
  if (fromRequestList) return 'requestList'
  if (fromErrorList) return 'errorList'
  return null
}

export function shouldResetOpsListFiltersOnOpen(resumeState: boolean): boolean {
  return !resumeState
}

export function nextOpsListVisibility(target: OpsDetailReturnTarget): {
  errorList: boolean
  requestList: boolean
  errorDetail: boolean
} {
  if (target === 'requestList') {
    return { errorList: false, requestList: true, errorDetail: false }
  }
  if (target === 'errorList') {
    return { errorList: true, requestList: false, errorDetail: false }
  }
  return { errorList: false, requestList: false, errorDetail: false }
}
