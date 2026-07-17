import { Suspense, lazy } from 'react'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { RequireAuth, RequireAdmin } from './auth/RequireAuth'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'

const OverviewPage = lazy(() => import('./pages/OverviewPage').then((m) => ({ default: m.OverviewPage })))
const EventExplorerPage = lazy(() =>
  import('./pages/EventExplorerPage').then((m) => ({ default: m.EventExplorerPage })),
)
const FunnelPage = lazy(() => import('./pages/FunnelPage').then((m) => ({ default: m.FunnelPage })))
const RetentionPage = lazy(() => import('./pages/RetentionPage').then((m) => ({ default: m.RetentionPage })))
const InstallationTimelinePage = lazy(() =>
  import('./pages/InstallationTimelinePage').then((m) => ({ default: m.InstallationTimelinePage })),
)
const CatalogPage = lazy(() => import('./pages/CatalogPage').then((m) => ({ default: m.CatalogPage })))
const SystemHealthPage = lazy(() =>
  import('./pages/SystemHealthPage').then((m) => ({ default: m.SystemHealthPage })),
)
const PolicyAdminPage = lazy(() => import('./pages/PolicyAdminPage').then((m) => ({ default: m.PolicyAdminPage })))

function RouteFallback() {
  return <p role="status">Loading…</p>
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Layout>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route
              path="/"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <OverviewPage />
                  </Suspense>
                </RequireAuth>
              }
            />
            <Route
              path="/events"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <EventExplorerPage />
                  </Suspense>
                </RequireAuth>
              }
            />
            <Route
              path="/funnel"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <FunnelPage />
                  </Suspense>
                </RequireAuth>
              }
            />
            <Route
              path="/retention"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <RetentionPage />
                  </Suspense>
                </RequireAuth>
              }
            />
            <Route
              path="/installations"
              element={
                <RequireAdmin>
                  <Suspense fallback={<RouteFallback />}>
                    <InstallationTimelinePage />
                  </Suspense>
                </RequireAdmin>
              }
            />
            <Route
              path="/catalog"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <CatalogPage />
                  </Suspense>
                </RequireAuth>
              }
            />
            <Route
              path="/system"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <SystemHealthPage />
                  </Suspense>
                </RequireAuth>
              }
            />
            <Route
              path="/policy"
              element={
                <RequireAuth>
                  <Suspense fallback={<RouteFallback />}>
                    <PolicyAdminPage />
                  </Suspense>
                </RequireAuth>
              }
            />
          </Routes>
        </Layout>
      </AuthProvider>
    </BrowserRouter>
  )
}
