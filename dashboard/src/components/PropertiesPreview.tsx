const INLINE_COUNT = 3

function formatValue(v: unknown): string {
  if (v === null) return 'null'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

// Comparing rows one at a time behind a <details> per row doesn't scale.
// Shows the first few key=value pairs inline so most rows are scannable
// without opening anything; the rest stay one click away as raw JSON.
export function PropertiesPreview({ properties }: { properties: Record<string, unknown> | null | undefined }) {
  const entries = Object.entries(properties ?? {})
  if (entries.length === 0) return <span className="hint">—</span>

  const inline = entries.slice(0, INLINE_COUNT)
  const rest = entries.length - inline.length

  return (
    <span className="properties-preview">
      {inline.map(([k, v]) => `${k}=${formatValue(v)}`).join(', ')}
      {rest > 0 && (
        <details>
          <summary>+{rest} more</summary>
          <pre>{JSON.stringify(properties, null, 2)}</pre>
        </details>
      )}
    </span>
  )
}
