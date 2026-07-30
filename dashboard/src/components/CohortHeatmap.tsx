import { useEffect, useRef } from 'react'
import { init, use } from 'echarts/core'
import { HeatmapChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { RetentionCohort } from '../api/types'
import { chartTheme, onColorSchemeChange } from './chartTheme'

use([HeatmapChart, GridComponent, TooltipComponent, VisualMapComponent, CanvasRenderer])

const OFFSETS = [1, 7, 30] as const
const COLUMNS = ['D1', 'D7', 'D30']

// null means "blank cell" — either the window hasn't elapsed yet (the
// classic cohort triangle's missing upper-right corner) or the cohort
// was empty, not a real 0%.
function pctCell(cohort: RetentionCohort, offset: number, to: string): number | null {
  const windowEndsAt = new Date(`${cohort.cohort_day}T00:00:00Z`)
  windowEndsAt.setUTCDate(windowEndsAt.getUTCDate() + offset)
  if (windowEndsAt > new Date(`${to}T23:59:59Z`)) return null
  if (cohort.cohort_size === 0) return null
  const retained = offset === 1 ? cohort.d1 : offset === 7 ? cohort.d7 : cohort.d30
  return Number(((retained / cohort.cohort_size) * 100).toFixed(1))
}

function heatmapPoints(cohorts: RetentionCohort[], to: string): Array<[number, number, number]> {
  const points: Array<[number, number, number]> = []
  cohorts.forEach((cohort, row) => {
    OFFSETS.forEach((offset, col) => {
      const value = pctCell(cohort, offset, to)
      if (value !== null) points.push([col, row, value])
    })
  })
  return points
}

function heatmapOption(cohorts: RetentionCohort[], points: Array<[number, number, number]>) {
  const theme = chartTheme()
  const axisStyle = {
    axisLine: { lineStyle: { color: theme.border } },
    axisLabel: { color: theme.textMuted },
  }
  return {
    grid: { left: 110, right: 16, top: 16, bottom: 56 },
    // No splitArea — a blank cell (window not yet elapsed) must read as
    // empty background, not a decorative shaded band that looks like data.
    xAxis: { type: 'category', data: COLUMNS, ...axisStyle },
    yAxis: { type: 'category', data: cohorts.map((c) => c.cohort_day), ...axisStyle },
    visualMap: {
      min: 0,
      max: 100,
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: 0,
      inRange: { color: [theme.bg, theme.accent] },
      textStyle: { color: theme.text },
    },
    tooltip: {
      backgroundColor: theme.bg,
      borderColor: theme.border,
      textStyle: { color: theme.text },
      formatter: (p: { data: [number, number, number] }) => `${cohorts[p.data[1]].cohort_day} — ${COLUMNS[p.data[0]]}: ${p.data[2]}%`,
    },
    series: [{ type: 'heatmap', data: points }],
  }
}

// ECharts heatmap — a genuinely different chart shape (matrix, not
// category/series pairs) from TrendChart, so it gets its own tiny
// component rather than forcing it through the trend/bar generalization.
export function CohortHeatmap({ cohorts, to }: { cohorts: RetentionCohort[]; to: string }) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!ref.current) return
    const chart = init(ref.current)
    const applyOption = () => chart.setOption(heatmapOption(cohorts, heatmapPoints(cohorts, to)), true)
    applyOption()
    const onResize = () => chart.resize()
    window.addEventListener('resize', onResize)
    const stopWatchingScheme = onColorSchemeChange(applyOption)
    return () => {
      window.removeEventListener('resize', onResize)
      stopWatchingScheme()
      chart.dispose()
    }
  }, [cohorts, to])

  return (
    <div
      ref={ref}
      role="img"
      aria-label="Cohort retention heatmap: D1, D7, D30 percentages by cohort day"
      style={{ width: '100%', height: 100 + cohorts.length * 32 }}
    />
  )
}
