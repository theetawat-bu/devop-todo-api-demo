# ---------- Stage 1: build ----------
FROM node:22-alpine AS builder
WORKDIR /app

# copy manifest ก่อน เพื่อให้ layer cache ทำงาน (โค้ดเปลี่ยนแต่ deps ไม่เปลี่ยน = ไม่ต้อง npm ci ใหม่)
COPY package*.json ./
RUN npm ci

COPY prisma ./prisma
RUN npx prisma generate

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# ---------- Stage 2: production deps ----------
FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev

# ---------- Stage 3: runtime ----------
FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ARG APP_VERSION=dev
ENV APP_VERSION=$APP_VERSION

RUN apk add --no-cache curl

COPY --from=deps   /app/node_modules ./node_modules
COPY --from=builder /app/node_modules/.prisma ./node_modules/.prisma
COPY --from=builder /app/dist ./dist
COPY package*.json ./
COPY prisma ./prisma

# รันด้วย non-root user (node มีอยู่แล้วใน image)
RUN chown -R node:node /app
USER node

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -fsS http://127.0.0.1:3000/healthz || exit 1

# migrate deploy ก่อนแล้วค่อยสตาร์ท (บน k8s เราย้ายไปใช้ initContainer แทน)
CMD ["sh", "-c", "npx prisma migrate deploy && node dist/index.js"]
