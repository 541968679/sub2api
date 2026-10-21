/**
 * Common component types
 */

export interface Column {
  key: string
  label: string
  sortable?: boolean
  class?: string
  formatter?: (value: any, row: any) => string
  /** Preferred column width in pixels (desktop table). */
  width?: number
  /** Minimum width in pixels when resizing. */
  minWidth?: number
  /**
   * Whether this column can be resized when the table has resizableColumns enabled.
   * Defaults to true for data columns; select/actions typically set false.
   */
  resizable?: boolean
  /**
   * Keep header label casing as provided (skip DataTable's default uppercase).
   * Useful when labels intentionally differ only by case (e.g. token vs TOKEN).
   */
  preserveHeaderCase?: boolean
  /**
   * Skip this column on the phone card list. The desktop table still renders it.
   * Use when the card already shows the value in a summary row.
   */
  hideOnMobile?: boolean
}
