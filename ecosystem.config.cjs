const path = require("path");

const appEnv = process.env.APP_ENV || (process.env.NODE_ENV === "production" ? "production" : "local");
const envFile = process.env.ENV_FILE || path.join(__dirname, ".env");
const cwd = process.env.DEPLOY_CWD || __dirname;
const hostname = process.env.HOSTNAME || "127.0.0.1";
const dashboardPort = process.env.DASHBOARD_PORT || 3100;
const rendererPort = process.env.RENDERER_PORT || 3200;

module.exports = {
  apps: [
    {
      name: `notepub-dashboard-${appEnv}`,
      script: "npm",
      args: `run start --workspace @notepub/dashboard -- --hostname ${hostname} --port ${dashboardPort}`,
      cwd,
      env: {
        APP_ENV: appEnv,
        NODE_ENV: "production",
        PORT: dashboardPort,
        HOSTNAME: hostname,
      },
      env_production: { APP_ENV: "production", NODE_ENV: "production" },
      env_staging: { APP_ENV: "staging", NODE_ENV: "production" },
      env_file: envFile,
      max_restarts: 3,
      restart_delay: 5000,
    },
    {
      name: `notepub-renderer-${appEnv}`,
      script: "npm",
      args: `run start --workspace @notepub/renderer -- --hostname ${hostname} --port ${rendererPort}`,
      cwd,
      env: {
        APP_ENV: appEnv,
        NODE_ENV: "production",
        PORT: rendererPort,
        HOSTNAME: hostname,
      },
      env_production: { APP_ENV: "production", NODE_ENV: "production" },
      env_staging: { APP_ENV: "staging", NODE_ENV: "production" },
      env_file: envFile,
      max_restarts: 3,
      restart_delay: 5000,
    },
  ],
};
