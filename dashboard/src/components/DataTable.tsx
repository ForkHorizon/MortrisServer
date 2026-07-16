import type { ReactNode } from 'react'

export interface Column<T> {
  key: string
  label: string
  render?: (row: T) => ReactNode
}

// A plain semantic <table> — screen readers and keyboard navigation get
// this for free with no ARIA needed. Every chart screen pairs one of
// these with its chart so the same data is always available as text
// (plan section 10.2: "textual values in addition to color/charts").
export function DataTable<T>({
  columns,
  rows,
  caption,
  getRowKey,
}: {
  columns: Column<T>[]
  rows: T[]
  caption: string
  getRowKey: (row: T, index: number) => string | number
}) {
  // Defensive against `null` — Go's encoding/json emits null, not [], for
  // a nil slice, and internal/analytics doesn't guarantee every list
  // field is initialized before it's empty (see also the backend fix:
  // internal/policyadmin and internal/analytics now always return []).
  if (!rows || rows.length === 0) {
    return <p>No data for this range.</p>
  }
  return (
    <table>
      <caption>{caption}</caption>
      <thead>
        <tr>
          {columns.map((c) => (
            <th scope="col" key={c.key}>
              {c.label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={getRowKey(row, i)}>
            {columns.map((c) => (
              <td key={c.key}>{c.render ? c.render(row) : String((row as Record<string, unknown>)[c.key] ?? '')}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
