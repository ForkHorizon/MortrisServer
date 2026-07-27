import { useEffect, useRef } from 'react'
import { init, use } from 'echarts/core'
import { LineChart, BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

const registerEChartsComponents = use

registerEChartsComponents([LineChart, BarChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

export interface TrendPoint {
  day: string
  count: number
}

export interface ChartSeries {
  name: string
  data: number[]
}

export type ChartType = 'line' | 'bar' | 'stacked'

// Section 3: "Apache ECharts — Locally bundled renderer only." Always
// pair this with a DataTable of the same points (section 10.2
// accessibility: textual values alongside charts) — this component is
// the visual, not the sole source of the data. One component for every
// line/bar/stacked-bar chart in the dashboard (Phase 3) instead of a new
// component per page.
export function TrendChart({
  categories,
  series,
  label,
  type = 'line',
  horizontal = false,
  height = 280,
  valueLabel,
}: {
  categories: string[]
  series: ChartSeries[]
  label: string
  type?: ChartType
  horizontal?: boolean
  height?: number
  valueLabel?: (value: number, dataIndex: number) => string
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!ref.current) return
    const chart = init(ref.current)
    const categoryAxis = { type: 'category' as const, data: categories }
    const valueAxis = { type: 'value' as const, minInterval: 1 }
    chart.setOption({
      grid: { left: horizontal ? 140 : 48, right: horizontal && valueLabel ? 110 : 16, top: 24, bottom: series.length > 1 ? 56 : 32 },
      xAxis: horizontal ? valueAxis : categoryAxis,
      yAxis: horizontal ? categoryAxis : valueAxis,
      legend: series.length > 1 ? { bottom: 0 } : undefined,
      tooltip: { trigger: 'axis' },
      series: series.map((s) => ({
        name: s.name,
        type: type === 'line' ? 'line' : 'bar',
        stack: type === 'stacked' ? 'total' : undefined,
        data: s.data,
        smooth: false,
        label: valueLabel
          ? { show: true, position: horizontal ? 'right' : 'top', formatter: (p: { value: number; dataIndex: number }) => valueLabel(p.value, p.dataIndex) }
          : undefined,
      })),
    })
    const onResize = () => chart.resize()
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('resize', onResize)
      chart.dispose()
    }
  }, [categories, series, type, horizontal, valueLabel])

  return <div ref={ref} role="img" aria-label={`${label} chart`} style={{ width: '100%', height }} />
}
