import { Route, Routes } from 'react-router-dom'
import { Toaster } from 'react-hot-toast'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import { AuthProvider } from './context/AuthContext'
import DashboardPage from './pages/DashboardPage'
import JobDetailPage from './pages/JobDetailPage'
import LoginPage from './pages/LoginPage'
import ProjectDetailPage from './pages/ProjectDetailPage'
import ProjectsPage from './pages/ProjectsPage'
import QueueDetailPage from './pages/QueueDetailPage'
import RegisterPage from './pages/RegisterPage'
import WorkersPage from './pages/WorkersPage'
import './App.css'

export default function App() {
  return (
    <AuthProvider>
      <Toaster
        position="top-right"
        toastOptions={{
          duration: 3500,
          style: {
            background: 'var(--bg)',
            color: 'var(--text-h)',
            border: '1px solid var(--border)',
            fontSize: '13.5px',
          },
          success: { iconTheme: { primary: 'var(--success)', secondary: 'var(--bg)' } },
          error: { iconTheme: { primary: 'var(--danger)', secondary: 'var(--bg)' } },
        }}
      />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<Layout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/projects" element={<ProjectsPage />} />
            <Route path="/projects/:projectId" element={<ProjectDetailPage />} />
            <Route path="/queues/:queueId" element={<QueueDetailPage />} />
            <Route path="/jobs/:jobId" element={<JobDetailPage />} />
            <Route path="/workers" element={<WorkersPage />} />
          </Route>
        </Route>
      </Routes>
    </AuthProvider>
  )
}
