export const env = {
  nodeEnv: process.env.NODE_ENV ?? 'development',
  port: Number(process.env.PORT ?? 3000),
  databaseUrl: process.env.DATABASE_URL ?? '',
  appVersion: process.env.APP_VERSION ?? 'dev',
};

if (!env.databaseUrl) {
  throw new Error('DATABASE_URL is not set');
}
