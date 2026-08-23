import { Route, Routes } from 'react-router-dom'
import ConfirmHost from './components/ConfirmHost'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import ToastHost from './components/ToastHost'
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
      <ToastHost />
      <ConfirmHost />
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
