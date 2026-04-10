import { BrowserRouter, Routes, Route } from 'react-router-dom'
import Layout from './components/layout'
import Dashboard from './pages/dashboard'
import Connections from './pages/connections'
import Combos from './pages/combos'
import Models from './pages/models'
import Keys from './pages/keys'
import Settings from './pages/settings'
import Logs from './pages/logs'
import Backup from './pages/backup'
import Playground from './pages/playground'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/connections" element={<Connections />} />
          <Route path="/combos" element={<Combos />} />
          <Route path="/models" element={<Models />} />
          <Route path="/keys" element={<Keys />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/logs" element={<Logs />} />
          <Route path="/backup" element={<Backup />} />
          <Route path="/playground" element={<Playground />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
