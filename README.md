# devops-todo-api

Todo REST API เล็ก ๆ (TypeScript + Express + Prisma + PostgreSQL) ที่ทำขึ้นมาเพื่อ **ฝึก DevOps** โดยเฉพาะ
ตัวแอปตั้งใจให้ง่ายที่สุด เพื่อให้โฟกัสไปที่ Docker / Nginx / CI/CD / Kubernetes ได้เต็มที่

> ⚠️ ก่อนเริ่ม: ในโฟลเดอร์นี้อาจมี `node_modules/` ที่ติดตั้งไว้ตอนทดสอบ (เป็นไบนารีของ Linux)
> ให้ลบทิ้งแล้วติดตั้งใหม่บนเครื่องตัวเอง: `rm -rf node_modules dist && npm install`

## Quick start

```bash
cp .env.example .env
docker compose up -d --build

curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/api/todos -H 'Content-Type: application/json' -d '{"title":"เรียน DevOps"}'
curl http://localhost:8080/api/todos
```

## API

| Method | Path | คำอธิบาย |
|---|---|---|
| GET | `/healthz` | liveness — process ยังอยู่ไหม |
| GET | `/readyz` | readiness — ต่อ DB ได้ไหม |
| GET | `/api/todos` | ดูทั้งหมด |
| GET | `/api/todos/:id` | ดูรายการเดียว |
| POST | `/api/todos` | สร้าง — body: `{ "title": string, "done"?: boolean }` |
| PATCH | `/api/todos/:id` | แก้ไข |
| DELETE | `/api/todos/:id` | ลบ (204) |

## เอกสารสอน (อ่านตามลำดับ)

| # | หัวข้อ | เนื้อหา |
|---|---|---|
| 01 | [ภาพรวม & สถาปัตยกรรม](docs/01-overview.md) | โครงสร้างโปรเจกต์, request วิ่งยังไง, roadmap การฝึก |
| 02 | [รันในเครื่อง (Local Dev)](docs/02-local-development.md) | Node, Prisma, migration, env |
| 03 | [Docker](docs/03-docker.md) | multi-stage build, layer cache, image เล็กลง, non-root |
| 04 | [Docker Compose](docs/04-docker-compose.md) | network, volume, healthcheck, depends_on, scale |
| 05 | [Nginx](docs/05-nginx.md) | reverse proxy, load balance, rate limit, header |
| 06 | [GitHub Actions — CI](docs/06-github-actions-ci.md) | workflow, job, service container, cache |
| 07 | [CD + GHCR](docs/07-cd-ghcr.md) | build & push image, tag strategy, secrets, environment |
| 08 | [Kubernetes](docs/08-kubernetes.md) | Deployment, Service, Ingress, probe, HPA, kustomize, rollback |
| 09 | [แบบฝึกหัด](docs/09-exercises.md) | โจทย์ไล่ระดับ พร้อมเฉลยแนวคิด |
| 10 | [แก้ปัญหาที่เจอบ่อย](docs/10-troubleshooting.md) | error ยอดฮิตของแต่ละหัวข้อ |

## โครงสร้างไฟล์

```
.
├── src/                        โค้ด Express + TypeScript
├── prisma/                     schema + migrations
├── nginx/                      config ของ reverse proxy
├── k8s/
│   ├── base/                   manifests หลัก
│   └── overlays/{dev,production}   kustomize overlay
├── .github/workflows/
│   ├── ci.yml                  ทุก push/PR: typecheck, build, smoke test
│   └── cd.yml                  merge main: build → push GHCR → deploy k8s
├── Dockerfile                  multi-stage
├── docker-compose.yml          db + api(x2) + nginx
├── docker-compose.dev.yml      override สำหรับ hot reload
└── docs/                       เอกสารสอน
```
