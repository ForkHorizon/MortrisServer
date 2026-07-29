// Aggregates blend untrusted-clock events into their day buckets with no
// visible signal — this makes that visible next to any time-bucketed view.
export function TimeQualityNote({ pct }: { pct: number }) {
  if (pct <= 0) return null
  return (
    <p className="hint" role="status">
      {pct.toFixed(1)}% of events in this range have untrusted timing (time_quality: untrusted).
    </p>
  )
}
