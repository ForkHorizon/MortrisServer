export function StatGrid({ children }: { children: React.ReactNode }) {
  return <dl className="stat-grid">{children}</dl>
}

export function StatTile({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="stat-tile">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  )
}
