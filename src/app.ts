import express, { NextFunction, Request, Response } from 'express';
import { ZodError } from 'zod';
import { env } from './env';
import { prisma } from './prisma';
import { todosRouter } from './routes/todos';

export function createApp() {
  const app = express();

  // ให้ req.ip เป็น IP จริงเมื่ออยู่หลัง Nginx / Ingress
  app.set('trust proxy', true);
  app.disable('x-powered-by');

  app.use(express.json());

  // log แบบง่าย ๆ (production จริงควรใช้ pino/winston)
  app.use((req, _res, next) => {
    console.log(JSON.stringify({ level: 'info', method: req.method, path: req.path, ip: req.ip }));
    next();
  });

  // liveness: แค่บอกว่า process ยังไม่ตาย
  app.get('/healthz', (_req, res) => {
    res.json({ status: 'ok', version: env.appVersion, uptime: process.uptime() });
  });

  // readiness: ต่อ DB ได้ไหม (k8s ใช้ตัวนี้ตัดสินใจส่ง traffic เข้ามา)
  app.get('/readyz', async (_req, res) => {
    try {
      await prisma.$queryRaw`SELECT 1`;
      res.json({ status: 'ready' });
    } catch {
      res.status(503).json({ status: 'not-ready' });
    }
  });

  app.use('/api/todos', todosRouter);

  app.use((_req, res) => {
    res.status(404).json({ error: 'Not found' });
  });

  app.use((err: unknown, _req: Request, res: Response, _next: NextFunction) => {
    // body ไม่ผ่าน schema
    if (err instanceof ZodError) {
      return res.status(400).json({ error: 'Validation failed', details: err.issues });
    }
    // JSON พัง / body ใหญ่เกิน — body-parser โยน error ที่มี status มาให้แล้ว
    if (typeof err === 'object' && err !== null && 'status' in err) {
      const status = Number((err as { status: unknown }).status);
      if (status >= 400 && status < 500) {
        return res.status(status).json({ error: 'Bad request' });
      }
    }
    console.error(err);
    res.status(500).json({ error: 'Internal server error' });
  });

  return app;
}
