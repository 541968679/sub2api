export type CsvCell = string | number | boolean | null | undefined

const escapeCsvCell = (value: CsvCell): string => {
  let text = value == null ? '' : String(value)
  if (/^[=+\-@]/.test(text)) text = `'${text}`
  if (/[",\r\n]/.test(text)) text = `"${text.replace(/"/g, '""')}"`
  return text
}

export const buildUserUsageCsvBytes = (headers: string[], rows: CsvCell[][]): Uint8Array => {
  const content = [
    headers.map(escapeCsvCell).join(','),
    ...rows.map((row) => row.map(escapeCsvCell).join(',')),
  ].join('\r\n')
  return new TextEncoder().encode(`\uFEFF${content}`)
}
