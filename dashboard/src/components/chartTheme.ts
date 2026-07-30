// index.css defines --text/--text-muted/--border/--bg/--accent for both
// color schemes via `prefers-color-scheme`; ECharts needs resolved color
// strings, not var() references, so this reads them off the root element
// at option-build time instead of hardcoding one scheme's palette.
export function chartTheme() {
  const style = getComputedStyle(document.documentElement)
  const read = (name: string, fallback: string) => style.getPropertyValue(name).trim() || fallback
  return {
    text: read('--text', '#1a1d23'),
    textMuted: read('--text-muted', '#565d6b'),
    border: read('--border', '#d7dae0'),
    bg: read('--bg', '#ffffff'),
    accent: read('--accent', '#2563eb'),
  }
}

// Re-applies `apply` whenever the OS color scheme flips, so a chart
// already on screen re-themes without a page reload.
export function onColorSchemeChange(apply: () => void): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  media.addEventListener('change', apply)
  return () => media.removeEventListener('change', apply)
}
