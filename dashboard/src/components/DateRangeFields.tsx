import type { useDateRange } from '../hooks/useDateRange'

export function DateRangeFields({ range }: { range: ReturnType<typeof useDateRange> }) {
  return (
    <fieldset className="date-range">
      <legend>Date range</legend>
      <div className="field">
        <label htmlFor="range-from">From</label>
        <input id="range-from" type="date" value={range.from} onChange={(e) => range.setFrom(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="range-to">To</label>
        <input id="range-to" type="date" value={range.to} onChange={(e) => range.setTo(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="range-tz">Timezone</label>
        <input
          id="range-tz"
          type="text"
          value={range.timezone}
          onChange={(e) => range.setTimezone(e.target.value)}
          aria-describedby="range-tz-hint"
        />
        <span id="range-tz-hint" className="hint">
          IANA name, e.g. America/New_York
        </span>
      </div>
    </fieldset>
  )
}
