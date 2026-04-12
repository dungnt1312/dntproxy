module.exports = {
  apps: [
    {
      name: 'dntproxy',
      script: 'bash',
      args: 'dev.sh',
      cwd: 'C:\\laragon\\www\\dntproxy',
      env: {
        PORT: '20199',
        DNTPROXY_LOG_RAW_BODIES: '1',
      },
      // Auto-restart on crash
      autorestart: true,
      // Max restart attempts
      max_restarts: 10,
      // Restart delay
      restart_delay: 3000,
      // Log files
      out_file: 'C:\\laragon\\www\\dntproxy\\logs\\pm2-out.log',
      error_file: 'C:\\laragon\\www\\dntproxy\\logs\\pm2-error.log',
      // Merge stdout/stderr
      merge_logs: true,
      // Don't watch files
      watch: false,
      // Instances
      instances: 1,
      // Kill timeout
      kill_timeout: 5000,
      // Listen on startup
      listen_timeout: 10000,
    },
  ],
}
