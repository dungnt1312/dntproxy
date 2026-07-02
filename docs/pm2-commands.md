# PM2 Commands for dntproxy

## Quick Start

### Start Application
```bash
# Using Laragon Node.js
cd C:\laragon\www\dntproxy
"C:\laragon\bin\nodejs\node-v22.20.0-win-x64\npx.cmd" pm2 start ecosystem.config.js

# Or if PM2 is in PATH
pm2 start ecosystem.config.js
```

### Common Commands

```bash
# View status
pm2 status

# View logs (streaming)
pm2 logs dntproxy

# View logs (last 50 lines, no stream)
pm2 logs dntproxy --lines 50 --nostream

# View error logs only
pm2 logs dntproxy --err --lines 50 --nostream

# Restart application
pm2 restart dntproxy

# Restart with updated environment
pm2 restart dntproxy --update-env

# Stop application
pm2 stop dntproxy

# Delete from PM2
pm2 delete dntproxy

# Monitor in real-time
pm2 monit

# View detailed info
pm2 info dntproxy

# Kill all PM2 processes
pm2 kill
```

## Auto-start on Boot

```bash
# Generate startup script
pm2 startup

# Save current process list
pm2 save

# On Windows, use NSSM or Task Scheduler instead
```

## Environment Variables

Set in `ecosystem.config.js`:

```javascript
env: {
  PORT: '20199',
  DNTPROXY_LOG_RAW_BODIES: '1',
}
```

## Runtime

`dntproxy` runs from the local installed binary:

```javascript
script: 'C:\\Users\\dungnt\\.local\\bin\\dntproxy.exe',
args: '--port=20199',
```

Rebuild and reinstall from source before restarting when you want PM2 to use fresh local code:

```bash
bash ./install-local.sh
pm2 restart dntproxy --update-env
```

## Logs Location

- **Output logs:** `C:\laragon\www\dntproxy\logs\pm2-out.log`
- **Error logs:** `C:\laragon\www\dntproxy\logs\pm2-error.log`

## Troubleshooting

### Port Already in Use
```bash
# Kill process on port 20199
netstat -ano | findstr :20199
taskkill /PID <PID> /F

# Then restart
pm2 restart dntproxy
```

### Build Fails - File Locked
```bash
# Kill all dntproxy.exe processes
tasklist | findstr dntproxy
taskkill /IM dntproxy.exe /F

# Restart
pm2 restart dntproxy
```

### PM2 Not Found
```bash
# Install PM2 globally
npm install -g pm2

# Or use Laragon Node.js
"C:\laragon\bin\nodejs\node-v22.20.0-win-x64\npm.cmd" install -g pm2
```

## Access URLs

- **Backend API:** http://localhost:20199
- **Playground:** http://localhost:20199/playground
- **Health Check:** http://localhost:20199/health
