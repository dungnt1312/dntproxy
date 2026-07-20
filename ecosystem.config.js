const path = require('path')
const os = require('os')
const fs = require('fs')

const home = process.env.HOME || process.env.USERPROFILE || os.homedir()
const projectDir = process.env.DNTPROXY_PROJECT_DIR || path.resolve(__dirname)

// Prefer install-local.sh path, then install.ps1 path, then project-local binary.
const candidates = [
  path.join(home, '.local', 'bin', process.platform === 'win32' ? 'dntproxy.exe' : 'dntproxy'),
  path.join(process.env.LOCALAPPDATA || '', 'dntproxy', 'bin', 'dntproxy.exe'),
  path.join(projectDir, process.platform === 'win32' ? 'dntproxy.exe' : 'dntproxy'),
].filter(Boolean)

const script = candidates.find((p) => {
  try { return fs.existsSync(p) } catch { return false }
}) || candidates[0]

module.exports = {
  apps: [
    {
      name: 'dntproxy',
      script,
      args: 'serve',
      interpreter: 'none',
      cwd: projectDir,
      env: {
        PORT: process.env.PORT || '20199',
        // Keep raw body logging off by default (enable via env when debugging).
        DNTPROXY_LOG_RAW_BODIES: process.env.DNTPROXY_LOG_RAW_BODIES || '0',
      },
      autorestart: true,
      max_restarts: 10,
      restart_delay: 3000,
      watch: false,
      instances: 1,
      kill_timeout: 5000,
      listen_timeout: 10000,
    },
  ],
}
