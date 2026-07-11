import type { UsageLog } from '@/types'

type ImageOutputTokenRow = Pick<UsageLog, 'output_tokens' | 'image_output_tokens'>

export const hasImageOutputTokens = (row: ImageOutputTokenRow | null | undefined): boolean =>
  (row?.image_output_tokens ?? 0) > 0

export const textOutputTokens = (row: ImageOutputTokenRow | null | undefined): number =>
  Math.max(0, (row?.output_tokens ?? 0) - (row?.image_output_tokens ?? 0))
