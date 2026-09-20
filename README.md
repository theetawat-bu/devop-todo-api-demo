# devops-todo-api

Todo REST API เล็ก ๆ (Go + Gin + database/sql + PostgreSQL) ที่ทำขึ้นมาเพื่อ **ฝึก DevOps** โดยเฉพาะ
ตัวแอปตั้งใจให้ง่ายที่สุด เพื่อให้โฟกัสไปที่ Docker / Nginx / CI/CD / Kubernetes / GitOps ได้เต็มที่

## Quick start

```bash
cp .env.example .env
go mod download
docker compose up -d --build

curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/api/todos -H 'Content-Type: application/json' -d '{"title":"เรียน DevOps"}'
curl http://localhost:8080/api/todos
```

## API

| Method | Path | คำอธิบาย |
| --- | --- | --- |
| GET | `/healthz` | liveness — process ยังอยู่ไหม (**ไม่แตะ DB**) |
| GET | `/readyz` | readiness — ต่อ DB ได้ไหม |
| GET | `/api/todos` | ดูทั้งหมด |
| GET | `/api/todos/:id` | ดูรายการเดียว |
| POST | `/api/todos` | สร้าง — body: `{ "title": string, "done"?: boolean }` |
| PATCH | `/api/todos/:id` | แก้ไข |
| DELETE | `/api/todos/:id` | ลบ (204) |

---

## 📚 เอกสารสอน (`docs/`)

อ่านตามลำดับ แต่ละไฟล์ต่อยอดจากไฟล์ก่อนหน้า

| # | หัวข้อ | เนื้อหา |
| --- | --- | --- |
| 00 | [🗺️ Roadmap การเรียนรู้](docs/00-roadmap.md) | เส้นทางเรียนทั้งหมดตั้งแต่ศูนย์ถึง deploy ฟรี + scale — เริ่มที่นี่ถ้ายังไม่รู้จะเริ่มตรงไหน |
| 01 | [ภาพรวม & สถาปัตยกรรม](docs/01-overview.md) | โครงสร้างโปรเจกต์, request วิ่งยังไง, roadmap |
| 02 | [รันในเครื่อง](docs/02-local-development.md) | Go, golang-migrate, migration, env |
| 03 | [Docker](docs/03-docker.md) | multi-stage build, layer cache, image เล็กลง, non-root |
| 04 | [Docker Compose](docs/04-docker-compose.md) | network, volume, healthcheck, depends_on, scale |
| 05 | [Nginx](docs/05-nginx.md) | reverse proxy, **เปรียบเทียบอัลกอริทึม LB**, rate limit 5 ชั้น, health check |
| 06 | [GitHub Actions — CI](docs/06-github-actions-ci.md) | workflow, job, service container, cache |
| 07 | [CD + GHCR](docs/07-cd-ghcr.md) | build & push image, tag strategy, secrets, environment |
| 08 | [Kubernetes](docs/08-kubernetes.md) | **probe 3 แบบเปรียบเทียบ**, LB บน k8s, HPA, kustomize, rollback |
| 09 | [จะตั้งค่าที่ชั้นไหนดี](docs/09-where-to-configure.md) | 🎯 **LB / rate limit / health check ควรอยู่ชั้นไหน** + timeout budget + retry |
| 10 | [Deploy ขึ้น cloud ฟรี](docs/10-deploy-free-cloud.md) | 🚀 Docker + **Kubernetes จริง** บน cloud ฟรี + workflow ครบชุด 5 ไฟล์ |
| 11 | [Enterprise GitOps](docs/11-enterprise-gitops.md) | 🏢 GitHub Actions → Harbor → Argo CD → private cloud หลัง VPN |
| 12 | [แก้ปัญหาที่เจอบ่อย](docs/12-troubleshooting.md) | error ยอดฮิตของแต่ละหัวข้อ |
| 13 | [อ่านสคริปต์ deploy](docs/13-reading-scripts.md) | 🔍 **อ่าน workflow + bash ทีละบรรทัด** — `curl -f`, retry loop, `if:`, outputs |
| 14 | [Deploy Quickstart](docs/14-deploy-quickstart.md) | ⚡ **คู่มือทำตามล้วน ๆ ไม่มีทฤษฎี** — CI/CD → Docker → ขึ้น cloud ฟรีจริง (PaaS + Kubernetes) |

> **จะทำ [10 — deploy ขึ้น cloud](docs/10-deploy-free-cloud.md) ต้องรู้อะไรก่อน?**
> ดูตารางความรู้ที่ต้องมี + แบบทดสอบตัวเอง 8 ข้อ ที่[หัวข้อ 0 ของ docs/10](docs/10-deploy-free-cloud.md)
> ทางลัด: [02](docs/02-local-development.md) → [03 ภาคลึก](docs/03-docker.md) → [06 ภาคลึก](docs/06-github-actions-ci.md) → [13](docs/13-reading-scripts.md) → [08 ภาคลึก](docs/08-kubernetes.md)

---

## 🏋️ แบบฝึกหัด (`exercises/`)

แยกตามหัวข้อ ไล่จาก **ระดับ 1 (เริ่มต้น) → ระดับ 5 (มืออาชีพ)** ทุกข้อมีเกณฑ์ "ผ่านเมื่อ" ที่วัดได้
👉 [เริ่มที่นี่](exercises/README.md) · [เฉลย](exercises/solutions/README.md)

| หัวข้อ | 🟢 1 | 🔵 2 | 🟡 3 | 🟠 4 | 🔴 5 |
| --- | --- | --- | --- | --- | --- |
| 🐳 **Docker** | [เริ่มต้น](exercises/docker/01-beginner.md) | [พื้นฐานแน่น](exercises/docker/02-intermediate.md) | [ใช้งานจริง](exercises/docker/03-production.md) | [ขั้นสูง](exercises/docker/04-advanced.md) | [มืออาชีพ](exercises/docker/05-expert.md) |
| 🌐 **Nginx** | [เริ่มต้น](exercises/nginx/01-beginner.md) | [พื้นฐานแน่น](exercises/nginx/02-intermediate.md) | [ใช้งานจริง](exercises/nginx/03-production.md) | [ขั้นสูง](exercises/nginx/04-advanced.md) | [มืออาชีพ](exercises/nginx/05-expert.md) |
| ☸️ **Kubernetes** | [เริ่มต้น](exercises/kubernetes/01-beginner.md) | [พื้นฐานแน่น](exercises/kubernetes/02-intermediate.md) | [ใช้งานจริง](exercises/kubernetes/03-production.md) | [ขั้นสูง](exercises/kubernetes/04-advanced.md) | [มืออาชีพ](exercises/kubernetes/05-expert.md) |
| ⚙️ **CI/CD** | [เริ่มต้น](exercises/cicd/01-beginner.md) | [พื้นฐานแน่น](exercises/cicd/02-intermediate.md) | [ใช้งานจริง](exercises/cicd/03-production.md) | [ขั้นสูง](exercises/cicd/04-advanced.md) | [มืออาชีพ](exercises/cicd/05-expert.md) |

---

## โครงสร้างไฟล์

```
.
├── cmd/api/                      entrypoint (main.go)
├── internal/                     โค้ด Go (app, config, db, todos)
├── migrations/                   golang-migrate .up.sql / .down.sql
├── nginx/                        config ของ reverse proxy
├── k8s/
│   ├── base/                     manifests หลัก
│   └── overlays/{dev,production,cloud} kustomize overlay
├── .github/workflows/
│   ├── ci.yml                    ทุก push/PR: typecheck, test, validate k8s, build
│   ├── k8s-e2e.yml               PR: ปั้น k3d → deploy จริง → ทดสอบ zero-downtime
│   ├── build-push.yml            reusable: build multi-arch → GHCR → คืน digest
│   ├── deploy-paas.yml           push dev: → Render/Koyeb → verify
│   └── deploy-k8s.yml            push main: → k3s บน Oracle Cloud → verify → rollback
├── docs/                         เอกสารสอน 12 หัวข้อ
├── exercises/                    แบบฝึกหัด 4 หัวข้อ × 5 ระดับ
│   └── solutions/                เฉลยแยกไฟล์
├── Dockerfile                    multi-stage
├── docker-compose.yml            db + api(x2) + nginx
└── docker-compose.dev.yml        override สำหรับ hot reload
```

## เส้นทาง deploy 3 แบบที่โปรเจกต์นี้สอน

| แบบ | workflow | ปลายทาง | อ่านที่ |
| --- | --- | --- | --- |
| **ฝึกในเครื่อง** | — | docker compose / minikube | [03](docs/03-docker.md), [04](docs/04-docker-compose.md), [08](docs/08-kubernetes.md) |
| **ขึ้นเน็ตฟรี (PaaS)** | `deploy-paas.yml` | Render + Neon (push `dev`) | [10](docs/10-deploy-free-cloud.md) |
| **Kubernetes ฟรีจริง** | `deploy-k8s.yml` | k3s บน Oracle Cloud ARM (push `main`) | [10](docs/10-deploy-free-cloud.md) |
| **ระดับองค์กร** | เอกสารอย่างเดียว | Harbor + Argo CD + private cloud หลัง VPN | [11](docs/11-enterprise-gitops.md) |
