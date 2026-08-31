# ---------- Stage 1: build ----------
FROM golang:1.25-alpine AS builder
WORKDIR /app

# copy manifest ก่อน เพื่อให้ layer cache ทำงาน (โค้ดเปลี่ยนแต่ deps ไม่เปลี่ยน = ไม่ต้อง download ใหม่)
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

# golang-migrate CLI สำหรับรัน migration ตอน start (เทียบเท่า npx prisma migrate deploy)
RUN CGO_ENABLED=0 go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1

# ---------- Stage 2: runtime ----------
FROM alpine:3.20 AS runner
WORKDIR /app

ARG APP_VERSION=dev
ENV APP_VERSION=$APP_VERSION

RUN apk add --no-cache curl

# ไม่มี built-in non-root user เหมือน image ของ node เลยสร้างเอง
RUN addgroup -S app && adduser -S -G app -u 1000 app

COPY --from=builder /out/api ./api
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY migrations ./migrations

RUN chown -R app:app /app
USER app

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -fsS http://127.0.0.1:3000/healthz || exit 1

# migrate ก่อนแล้วค่อยสตาร์ท (บน k8s เราย้ายไปใช้ initContainer แทน)
CMD ["sh", "-c", "migrate -path ./migrations -database \"$DATABASE_URL\" up && ./api"]
