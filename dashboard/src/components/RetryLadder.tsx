import type { PuzzleHouseBlock } from '../api/houseTypes'

interface RetryLadderProps {
  blocks: PuzzleHouseBlock[]
  waveIndex: number | null
  selectedBlockId: number | null
  onSelectBlock: (blockId: number) => void
  onSelectAttempt?: (attemptId: string) => void
}

function formatPercent(count: number, total: number): string {
  if (total <= 0) return '0%'
  return `${Math.round((count / total) * 100)}%`
}

export function RetryLadderTable({ blocks, waveIndex, selectedBlockId, onSelectBlock, onSelectAttempt }: RetryLadderProps) {
  const visibleBlocks = waveIndex != null ? blocks.filter((b) => b.wave_index === waveIndex) : blocks
  const nonGroundBlocks = visibleBlocks.filter((b) => !b.is_ground)

  return (
    <div className="retry-ladder-wrapper">
      <table className="retry-ladder-table" aria-label="Retry ladder by detail">
        <thead>
          <tr>
            <th scope="col">Detail</th>
            <th scope="col">Wave</th>
            <th scope="col">1st try</th>
            <th scope="col">2nd try</th>
            <th scope="col">3rd try</th>
            <th scope="col">4th+ try</th>
            <th scope="col">Never placed</th>
            <th scope="col">Chains</th>
            <th scope="col">Median tries</th>
            <th scope="col">Verdict</th>
            <th scope="col">Example replays</th>
          </tr>
        </thead>
        <tbody>
          {nonGroundBlocks.length === 0 ? (
            <tr>
              <td colSpan={11} className="muted" style={{ textAlign: 'center', padding: '1rem' }}>
                No details in this wave.
              </td>
            </tr>
          ) : (
            nonGroundBlocks.map((b) => {
              const ladder = b.retry_ladder
              const total = ladder?.sample_count ?? 0
              const isSelected = selectedBlockId === b.block_id
              const reliable = ladder?.reliable ?? false
              const hasSuccess = b.successful_placements > 0 && (ladder?.median_tries ?? 0) > 0

              let verdictClass = 'ladder-badge ladder-badge--grey'
              let verdictText = 'Unplayed'
              if (total > 0 && !reliable) {
                verdictClass = 'ladder-badge ladder-badge--amber'
                verdictText = 'Not enough plays'
              } else if (total > 0 && reliable) {
                const neverRate = (ladder?.never_succeeded ?? 0) / total
                if (neverRate >= 0.25 || (ladder?.median_tries ?? 0) >= 3) {
                  verdictClass = 'ladder-badge ladder-badge--red'
                  verdictText = 'High friction'
                } else if ((ladder?.median_tries ?? 0) === 1) {
                  verdictClass = 'ladder-badge ladder-badge--green'
                  verdictText = 'Smooth (1st try)'
                } else {
                  verdictClass = 'ladder-badge ladder-badge--blue'
                  verdictText = 'Moderate retries'
                }
              }

              return (
                <tr
                  key={b.block_id}
                  className={`retry-ladder-row ${isSelected ? 'retry-ladder-row--selected' : ''}`}
                  onClick={() => onSelectBlock(b.block_id)}
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      onSelectBlock(b.block_id)
                    }
                  }}
                  aria-selected={isSelected}
                >
                  <th scope="row" style={{ textAlign: 'left', fontWeight: 'normal' }}>
                    <strong>Detail {b.block_id}</strong>
                    <span className="muted block-subtext">{b.visual_key}</span>
                  </th>
                  <td>Wave {b.wave_index + 1}</td>
                  <td>{total > 0 && ladder ? `${ladder.success_1st_try} (${formatPercent(ladder.success_1st_try, total)})` : '—'}</td>
                  <td>{total > 0 && ladder ? `${ladder.success_2nd_try} (${formatPercent(ladder.success_2nd_try, total)})` : '—'}</td>
                  <td>{total > 0 && ladder ? `${ladder.success_3rd_try} (${formatPercent(ladder.success_3rd_try, total)})` : '—'}</td>
                  <td>{total > 0 && ladder ? `${ladder.success_4th_plus_try} (${formatPercent(ladder.success_4th_plus_try, total)})` : '—'}</td>
                  <td>{total > 0 && ladder ? `${ladder.never_succeeded} (${formatPercent(ladder.never_succeeded, total)})` : '—'}</td>
                  <td>{total}</td>
                  <td>
                    {!hasSuccess ? (
                      <span className="muted">—</span>
                    ) : reliable ? (
                      <span>
                        <strong>{ladder?.median_tries ?? 0}</strong>
                        {ladder && (ladder.p75_tries > 0 || ladder.p90_tries > 0) && (
                          <span className="muted percentile-subtext" title={`p75: ${ladder.p75_tries}, p90: ${ladder.p90_tries}`}>
                            {' '}(p75: {ladder.p75_tries}, p90: {ladder.p90_tries})
                          </span>
                        )}
                      </span>
                    ) : (
                      <span className="muted" title="Fewer than 5 samples">
                        {ladder?.median_tries ? `~${ladder.median_tries}` : '—'}
                      </span>
                    )}
                  </td>
                  <td>
                    <span className={verdictClass}>{verdictText}</span>
                  </td>
                  <td
                    onClick={(e) => e.stopPropagation()}
                    onKeyDown={(e) => e.stopPropagation()}
                  >
                    {ladder?.example_attempt_ids && ladder.example_attempt_ids.length > 0 ? (
                      <div className="ladder-replay-links">
                        {ladder.example_attempt_ids.slice(0, 3).map((attId) => (
                          <button
                            key={attId}
                            type="button"
                            className="link-button example-replay-btn"
                            title={`Watch replay for attempt ${attId}`}
                            aria-label={`Watch replay for attempt ${attId.slice(0, 8)}`}
                            onClick={() => {
                              onSelectBlock(b.block_id)
                              onSelectAttempt?.(attId)
                            }}
                          >
                            #{attId.slice(0, 8)}
                          </button>
                        ))}
                      </div>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </div>
  )
}
