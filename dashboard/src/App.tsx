import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { RequireAuth, RequireAdmin } from './auth/RequireAuth'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'
import { OverviewPage } from './pages/OverviewPage'
import { EventExplorerPage } from './pages/EventExplorerPage'
import { FunnelPage } from './pages/FunnelPage'
import { RetentionPage } from './pages/RetentionPage'
import { InstallationTimelinePage } from './pages/InstallationTimelinePage'
import { CatalogPage } from './pages/CatalogPage'
import { SystemHealthPage } from './pages/SystemHealthPage'
import { PolicyAdminPage } from './pages/PolicyAdminPage'

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
                  <OverviewPage />
                </RequireAuth>
              }
            />
            <Route
              path="/events"
              element={
                <RequireAuth>
                  <EventExplorerPage />
                </RequireAuth>
              }
            />
            <Route
              path="/funnel"
              element={
                <RequireAuth>
                  <FunnelPage />
                </RequireAuth>
              }
            />
            <Route
              path="/retention"
              element={
                <RequireAuth>
                  <RetentionPage />
                </RequireAuth>
              }
            />
            <Route
              path="/installations"
              element={
                <RequireAdmin>
                  <InstallationTimelinePage />
                </RequireAdmin>
              }
            />
            <Route
              path="/catalog"
              element={
                <RequireAuth>
                  <CatalogPage />
                </RequireAuth>
              }
            />
            <Route
              path="/system"
              element={
                <RequireAuth>
                  <SystemHealthPage />
                </RequireAuth>
              }
            />
            <Route
              path="/policy"
              element={
                <RequireAuth>
                  <PolicyAdminPage />
                </RequireAuth>
              }
            />
          </Routes>
        </Layout>
      </AuthProvider>
    </BrowserRouter>
  )
}
