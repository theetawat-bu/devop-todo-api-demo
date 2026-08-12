import { createApp } from './app';
import { env } from './env';
import { prisma } from './prisma';

const app = createApp();
const server = app.listen(env.port, () => {
  console.log(`API listening on :${env.port} (env=${env.nodeEnv}, version=${env.appVersion})`);
});

// graceful shutdown — สำคัญมากตอน rolling update บน Kubernetes
async function shutdown(signal: string) {
  console.log(`${signal} received, shutting down...`);
  server.close(async () => {
    await prisma.$disconnect();
    process.exit(0);
  });
  setTimeout(() => process.exit(1), 10_000).unref();
}

process.on('SIGTERM', () => void shutdown('SIGTERM'));
process.on('SIGINT', () => void shutdown('SIGINT'));
