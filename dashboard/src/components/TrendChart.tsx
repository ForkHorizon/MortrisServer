import { useEffect, useRef } from 'react'
import { init, use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

const registerEChartsComponents = use

registerEChartsComponents([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

export interface TrendPoint {
  day: string
  count: number
}

// Section 3: "Apache ECharts — Locally bundled renderer only." Always
// pair this with a DataTable of the same points (section 10.2
// accessibility: textual values alongside charts) — this component is
// the visual, not the sole source of the data.
export function TrendChart({ data, label }: { data: TrendPoint[]; label: string }) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!ref.current) return
    const chart = init(ref.current)
    chart.setOption({
      grid: { left: 48, right: 16, top: 24, bottom: 32 },
      xAxis: { type: 'category', data: data.map((d) => d.day) },
      yAxis: { type: 'value', minInterval: 1 },
      series: [
        {
          name: label,
          type: 'line',
          data: data.map((d) => d.count),
          color: '#2563eb',
          smooth: false,
        },
      ],
      tooltip: { trigger: 'axis' },
    })
    const onResize = () => chart.resize()
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('resize', onResize)
      chart.dispose()
    }
  }, [data, label])

  return <div ref={ref} role="img" aria-label={`${label} trend chart`} style={{ width: '100%', height: 280 }} />
}
