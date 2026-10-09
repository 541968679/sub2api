import { describe, expect, it } from 'vitest'
import { isGatewayLocalTimeoutLog, isGatewayLocalTimeoutText } from '../gatewayLocalTimeout'

describe('gatewayLocalTimeout', () => {
  it('recognizes our own wait and idle limits', () => {
    expect(isGatewayLocalTimeoutText('openai_header_wait_timeout waited_ms=90001')).toBe(true)
    expect(isGatewayLocalTimeoutText('openai_first_useful_frame_timeout waited_ms=31000')).toBe(true)
    expect(isGatewayLocalTimeoutText('stream data interval timeout')).toBe(true)
    expect(isGatewayLocalTimeoutText('image stream data interval timeout')).toBe(true)
    expect(isGatewayLocalTimeoutText('upstream stream idle for 1m0s')).toBe(true)
  })

  it('does not treat a real provider failure as a gateway timeout', () => {
    expect(isGatewayLocalTimeoutText('Upstream service temporarily unavailable')).toBe(false)
    expect(isGatewayLocalTimeoutText('provider returned 503 overloaded')).toBe(false)
  })

  it('reads the marker from the ops upstream field when the client text is generic', () => {
    expect(
      isGatewayLocalTimeoutLog({
        message: 'Upstream service temporarily unavailable',
        upstream_error_message: 'openai_header_wait_timeout waited_ms=90001'
      })
    ).toBe(true)
  })
})
