# 10 — Deploy ขึ้น Cloud ฟรี ด้วย Docker + Kubernetes + GitHub Actions

> **เป้าหมาย:** push โค้ดขึ้น GitHub แล้วระบบ build image, ทดสอบบน Kubernetes จริง และ deploy ให้เอง — โดยไม่เสียเงินสักบาท
> **สิ่งที่จะได้ฝึกจริง:** Docker multi-arch, Kubernetes บนเครื่องจริง (ไม่ใช่ minikube), reusable workflow, rollback อัตโนมัติ
> ⚠️ ตัวเลข free tier ในเอกสารนี้เช็คเมื่อ **สิงหาคม 2026** — ของพวกนี้เปลี่ยนบ่อยมาก ให้เข้าไปดูหน้า pricing อีกทีก่อนสมัคร

---

## 0. ความรู้ที่ต้องมีก่อน — อ่าน docs ไหนบ้าง

เอกสารนี้เป็น**ปลายทาง** ที่รวมทุกอย่างที่เรียนมา ถ้าลงมือเลยโดยไม่มีพื้นจะติดแน่นอน
ตารางนี้บอกว่าแต่ละขั้นตอนใช้ความรู้จากไหน

| Phase ในเอกสารนี้ | ต้องรู้อะไร | อ่านที่ | ถ้าไม่รู้จะเจออะไร |
| --- | --- | --- | --- |
| **0. Neon** | `golang-migrate` ใช้คำสั่ง `up` เดียวกันทั้ง dev/prod, `DATABASE_URL` ต่าง environment | [02](02-local-development.md) | migration ของ dev ไปแก้ตาราง prod |
| **1. k3d ใน CI** | probe, rollout, kustomize overlay | [08](08-kubernetes.md) | อ่านผล e2e ไม่ออกว่าพังเพราะอะไร |
| **2. PaaS** | image tag vs digest, GHCR permissions, environment/secret | [07](07-cd-ghcr.md) | push image ไม่ได้ / deploy ผิดเวอร์ชัน |
| **3. Kubernetes บน Oracle** | **multi-arch build (ARM)** | [03 ภาคลึก](03-docker.md) | `exec format error` แล้วหาสาเหตุไม่เจอ |
| | kubeconfig, `set image`, `rollout status` | [08 ภาคลึก](08-kubernetes.md) | ต่อคลัสเตอร์ไม่ได้ / workflow เขียวทั้งที่ pod พัง |
| | probe + `preStop` + graceful shutdown | [08](08-kubernetes.md), [09](09-where-to-configure.md) | เจอ 502 ทุกครั้งที่ deploy |
| **อ่าน workflow ทั้ง 5 ไฟล์** | expression, context, `if:`, outputs, shell semantics | [06 ภาคลึก](06-github-actions-ci.md) | อ่านได้แค่ "เดาว่าน่าจะทำอะไร" |
| | reusable workflow, `permissions`, `concurrency` | [06 ภาคลึก L7-L9](06-github-actions-ci.md) | แก้ workflow แล้วพังโดยไม่รู้สาเหตุ |
| **อ่านสคริปต์ bash ในนั้น** | `curl -fsS`, retry loop, `\|\| true`, `$( )` | [13](13-reading-scripts.md) | เขียน verify ที่เขียวเสมอ = ไร้ประโยชน์ |
| **ตั้งค่า timeout / rate limit** | timeout budget, ชั้นไหนทำอะไร | [09](09-where-to-configure.md) | ตั้ง timeout กลับด้าน → retry storm |

### เส้นทางอ่านที่สั้นที่สุด

ถ้าเวลาจำกัด อ่าน 4 ไฟล์นี้ก็ทำ Phase 1–2 ได้:

```
02 (env + migrate)  →  03 ภาคลึก (multi-arch)  →  06 ภาคลึก (อ่าน workflow)  →  13 (อ่าน bash)
```

จะทำ **Phase 3 (Kubernetes จริง)** ต้องเพิ่ม:

```
08 ทั้งไฟล์ + ภาคลึก  →  09 (timeout budget)
```

### ทดสอบตัวเองก่อนเริ่ม

ถ้าตอบได้ทั้ง 8 ข้อ แปลว่าพร้อมลงมือ ถ้าข้อไหนตอบไม่ได้ ให้กลับไปอ่าน doc ที่ระบุ

| # | คำถาม | อยู่ที่ |
| --- | --- | --- |
| 1 | `${{ }}` ถูกแทนค่าตอนไหน — ก่อนหรือหลัง shell เห็นบรรทัดนั้น | [06 L1](06-github-actions-ci.md) |
| 2 | `needs.build.result` เป็น `skipped` แล้ว job ถัดไปจะรันไหม | [06 L3](06-github-actions-ci.md) |
| 3 | ทำไม `curl` ในสคริปต์ CI ต้องมี `-f` | [13 §1.1](13-reading-scripts.md) |
| 4 | image เดียวรันได้ทั้ง x86 และ ARM ได้ยังไง | [03 X1](03-docker.md) |
| 5 | `kubectl set image` ต้องระบุ container กี่ตัวในโปรเจกต์นี้ เพราะอะไร | [08 D4](08-kubernetes.md) |
| 6 | ถ้าไม่มี `rollout status` จะเกิดอะไรขึ้นเมื่อ pod พัง | [08 D5](08-kubernetes.md) |
| 7 | `/healthz` กับ `/readyz` ต่างกันยังไง ทำไมต้องแยก | [08](08-kubernetes.md), [09](09-where-to-configure.md) |
| 8 | ทำไม job deploy ต้องตั้ง `cancel-in-progress: false` | [06 L9](06-github-actions-ci.md) |

---

## 1. ความจริงเรื่อง "Kubernetes ฟรี" ในปี 2026

ก่อนอื่นต้องเคลียร์ความเข้าใจผิดที่พบบ่อยที่สุด — **แทบไม่มี managed Kubernetes เจ้าไหนที่ฟรีจริง**

| ผู้ให้บริการ | ที่โฆษณาว่าฟรี | ความจริง |
| --- | --- | --- |
| **GKE (Google)** | เครดิต $74.40/เดือน | ครอบแค่ **ค่า control plane** ($0.10/ชม.) — **node ยังต้องจ่ายเต็ม** |
| **EKS (AWS)** | — | control plane $0.10/ชม. + node |
| **AKS (Azure)** | control plane ฟรี | node ยังต้องจ่าย |
| **DigitalOcean / Civo / Linode** | control plane ฟรี | node เริ่มที่ ~$12/เดือน |
| **Oracle Cloud (OKE + Always Free)** | ✅ **ฟรีถาวรจริง** | แต่ ARM ถูกหั่นครึ่งเมื่อ มิ.ย. 2026: จาก 4 OCPU/24GB **เหลือ 2 OCPU/12GB** |
| **k3d/kind ใน GitHub Actions** | ✅ ฟรี 100% | คลัสเตอร์อยู่แค่ระหว่าง workflow รัน แล้วหายไป |

**สรุป:** ถ้าอยากได้ Kubernetes ที่รันตลอด 24 ชม. โดยไม่จ่ายเงิน มีทางเดียวคือ **ติดตั้ง k3s เองบน VM ที่ฟรีถาวร** ซึ่งตอนนี้คือ Oracle Cloud Always Free

2 OCPU / 12 GB ยังเหลือเฟือสำหรับแอปตัวนี้ (ใช้จริงประมาณ 500 MB) — แค่ต้องรู้ว่าเป็น **ARM** ซึ่งกระทบเรื่อง Docker image โดยตรง

---

## 2. เลือกเส้นทางที่จะทำ

เอกสารนี้มี 3 เส้นทาง ทำทั้งหมดหรือเลือกเฉพาะที่สนใจก็ได้

| | 🅰️ PaaS | 🅱️ Kubernetes จริง | 🅲 k8s ใน CI |
| --- | --- | --- | --- |
| **ใช้อะไร** | Render/Koyeb + Neon | k3s บน Oracle Cloud + Neon | k3d ใน GitHub runner |
| **ฟรีจริงไหม** | ✅ ถาวร | ✅ ถาวร | ✅ ถาวร |
| **เวลาตั้งค่า** | 30 นาที | 2–3 ชั่วโมง | 10 นาที (ทำให้แล้ว) |
| **ได้ฝึก k8s ไหม** | ❌ | ✅ เต็มรูปแบบ | ✅ แต่คลัสเตอร์หายทุกครั้ง |
| **รันตลอดเวลาไหม** | หลับหลัง 15 นาที (Render) | ✅ | ❌ |
| **ต้องดูแลเซิร์ฟเวอร์** | ❌ | ✅ ต้อง patch เอง | ❌ |
| **workflow** | `deploy-paas.yml` | `deploy-k8s.yml` | `k8s-e2e.yml` |

**ลำดับที่แนะนำ:** 🅲 (ฟรีและเร็วที่สุด ได้เห็น k8s ทำงานทันที) → 🅰️ (ได้ URL จริงไว้โชว์) → 🅱️ (ของจริงเต็มรูปแบบ)

---

## 3. สถาปัตยกรรมรวม

```
                        git push
                           │
        ┌──────────────────┴───────────────────┐
        │        GitHub Actions                │
        │                                      │
        │  ci.yml ──────── ทุก push/PR         │
        │    ├─ quality      typecheck+test    │
        │    ├─ validate-k8s kustomize+schema  │
        │    └─ docker       build (ไม่ push)   │
        │                                      │
        │  k8s-e2e.yml ─── PR + push main      │
        │    └─ ปั้น k3d → deploy → ทดสอบ → ทิ้ง │
        │                                      │
        │  build-push.yml ─ reusable           │
        │    └─ multi-arch → GHCR → digest     │
        │           │                          │
        │     ┌─────┴─────┐                    │
        │     ▼           ▼                    │
        │ deploy-paas  deploy-k8s              │
        │  (dev)        (main)                 │
        └─────┬───────────┬────────────────────┘
              │           │
              ▼           ▼
      ┌────────────┐  ┌──────────────────────────┐
      │  Render    │  │ Oracle Cloud (ARM ฟรี)   │
      │  (Docker)  │  │  k3s + Traefik           │
      └─────┬──────┘  │  Deployment / HPA / PDB  │
            │         └────────────┬─────────────┘
            └──────────┬───────────┘
                       ▼
              ┌──────────────────┐
              │  Neon PostgreSQL │  ← ฐานข้อมูลตัวเดียว ใช้ร่วมกัน (คนละ database)
              └──────────────────┘
```

**จุดออกแบบที่สำคัญ:** ฐานข้อมูล **ไม่ได้อยู่ในคลัสเตอร์** — ใช้ Neon แทน
เพราะเครื่องฟรี 12 GB ถ้าเอาไปรัน Postgres ด้วยจะเหลือให้แอปน้อย และที่สำคัญกว่าคือ **ข้อมูลจะหายถ้าเครื่องพัง** ส่วน Neon มี backup ให้
นี่คือแพตเทิร์นเดียวกับที่องค์กรใช้จริง — **stateless workload อยู่ใน k8s ส่วน state ไปอยู่กับ managed service**

---

## 4. ชุด workflow ใหม่ทั้งหมด

เขียนใหม่ทั้งชุด 5 ไฟล์ แยกหน้าที่ชัดเจน

| ไฟล์ | trigger | ทำอะไร | แตะระบบจริงไหม |
| --- | --- | --- | --- |
| `ci.yml` | ทุก push/PR | typecheck, test, validate manifest, build (ไม่ push) | ❌ |
| `k8s-e2e.yml` | PR + push main | ปั้น k3d → deploy → ทดสอบ rolling update → ทิ้ง | ❌ |
| `build-push.yml` | ถูกเรียกเท่านั้น | build multi-arch → GHCR → คืน digest | push image |
| `deploy-paas.yml` | push `dev` | เรียก build-push → Render/Koyeb → verify | ✅ dev |
| `deploy-k8s.yml` | push `main` | เรียก build-push → k3s → verify → rollback | ✅ production |

### ทำไมต้องแยกเป็น 5 ไฟล์

**1. แยก "ตรวจ" ออกจาก "ส่งของ"** — `ci.yml` ไม่มีสิทธิ์ push อะไรเลย (`permissions: contents: read`)
workflow ที่รันบ่อยที่สุดควรมีสิทธิ์น้อยที่สุด ถ้ามีคนแก้ `ci.yml` แบบมุ่งร้าย มันก็ทำอะไรไม่ได้

**2. `build-push.yml` เป็น reusable workflow** — ทั้ง PaaS และ k8s เรียกตัวเดียวกัน
แปลว่า image ที่ขึ้นสองที่ **build ด้วยขั้นตอนเดียวกันเป๊ะ** ถ้าแยกเขียนสองที่ วันหนึ่งมันจะไม่ตรงกันแน่นอน

```yaml
# ในไฟล์ที่เรียกใช้
jobs:
  build:
    uses: ./.github/workflows/build-push.yml
    with:
      tag-prefix: main
      platforms: linux/amd64,linux/arm64
    secrets: inherit
```

**3. `k8s-e2e.yml` แยกออกมา** เพราะใช้เวลานาน (~10 นาที) ไม่ควรบล็อกทุก push
ตั้งให้รันเฉพาะ PR และเฉพาะเมื่อไฟล์ที่เกี่ยวข้องเปลี่ยน

---

## 5. Phase 0 — ฐานข้อมูลบน Neon

ใช้ร่วมกันทุกเส้นทาง ทำครั้งเดียวจบ

1. สมัครที่ [neon.com](https://neon.com) ด้วย GitHub (ไม่ต้องใช้บัตร)
2. **Create project** → ตั้งชื่อ `devops-todo-api` → เลือก region ใกล้สุด
3. สร้าง 2 database แยกกัน: `todo_dev` และ `todo_prod`
   (free tier ให้ 0.5 GB ต่อ project — พอสำหรับทั้งสอง)
4. คัดลอก connection string ปกติ จะได้หน้าตาแบบนี้:

```
postgresql://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/todo_prod?sslmode=require
```

**3 จุดที่พลาดกันบ่อยที่สุด:**

| จุด | ถ้าพลาดจะเจอ |
| --- | --- |
| ต้องมี `?sslmode=require` | `Error: connection is insecure` ตอน migrate |
| อย่าใช้ database เดียวกันทั้ง dev/prod | migration ของ dev ไปแก้ตาราง prod |
| Neon หลับหลังไม่ใช้ 5 นาที | request แรกช้า 2-3 วินาที (ปกติ ไม่ใช่บั๊ก) |

### pooled vs direct — Neon ให้มา 2 เส้น ใช้คนละงาน

```
postgresql://...@ep-xxx-pooler.ap-southeast-1.aws.neon.tech/...   ← pooled (มี -pooler)
postgresql://...@ep-xxx.ap-southeast-1.aws.neon.tech/...          ← direct (ไม่มี -pooler)
```

| เส้น | ผ่านอะไร | ใช้กับ |
| --- | --- | --- |
| **pooled** | PgBouncer | **แอปตอนรันจริง** — รองรับ connection พร้อมกันได้เยอะกว่ามาก |
| **direct** | ต่อ Postgres ตรง | **migration** — เพราะ PgBouncer ไม่รองรับ advisory lock และ DDL บางอย่าง |

**กฎที่จำง่าย: migration → direct / แอป → pooled**

ถ้าใช้ pooled รัน migration อาจเจออาการค้างนานหรือ advisory lock timeout แบบหาสาเหตุยาก

### ทดสอบจากเครื่องตัวเองก่อนเสมอ

**อย่าเพิ่งไปตั้งค่าบน cloud ถ้ายังไม่รู้ว่า connection string ใช้ได้จริง**

```bash
# ใช้เส้น direct (ไม่มี -pooler) สำหรับ migrate
migrate -path migrations -database "postgresql://...@ep-xxx.ap-southeast-1.aws.neon.tech/todo_prod?sslmode=require" up
```

ไม่มี GUI แบบ Prisma Studio ติดมาให้ดูว่าตาราง `todos` ถูกสร้างจริงไหม — ต่อด้วย `psql` ตรง ๆ หรือใช้ [TablePlus](https://tableplus.com/)/[DBeaver](https://dbeaver.io/) ถ้าอยากได้ GUI (เหมือนที่ [02](02-local-development.md) แนะนำไว้)

### ⚠️ ถ้าเจอ DB มีตารางอยู่แล้วแต่ไม่มีประวัติ migration (เทียบเท่า Prisma's P3005)

มักเกิดจากเคยสร้างตารางด้วยวิธีอื่นมาก่อน (SQL ตรง ๆ, tool อื่น) แล้วเพิ่งมาเริ่มใช้ `golang-migrate`
ถ้ามั่นใจว่า schema ปัจจุบันตรงกับ migration ไฟล์แรกเป๊ะ ให้ `force` เวอร์ชันนั้นแทนที่จะรัน `up` ตรง ๆ — ขั้นตอนเต็มพร้อมคำสั่งอยู่ที่ [12 — Troubleshooting](12-troubleshooting.md)

---

## 6. Phase 1 — เส้นทาง 🅲: Kubernetes ใน GitHub Actions

**เริ่มที่นี่ก่อน** เพราะไม่ต้องสมัครอะไรเลย และได้เห็น Kubernetes ทำงานจริงภายใน 10 นาที

ไฟล์ `k8s-e2e.yml` ทำ 4 อย่าง:

```yaml
- name: สร้างคลัสเตอร์ k3d
  uses: AbsaOSS/k3d-action@v2
  with:
    cluster-name: e2e
    args: --agents 1 --port 8080:80@loadbalancer --k3s-arg "--disable=traefik@server:0"
```

```yaml
- name: โหลด image เข้า k3d (ไม่ต้องผ่าน registry)
  run: k3d image import todo-api:e2e -c e2e
```

**`k3d image import` คือทริกที่ทำให้ e2e เร็ว** — ไม่ต้อง push ขึ้น registry แล้ว pull กลับ
ประหยัดเวลาไปหลายนาทีต่อรอบ และไม่ทิ้งขยะไว้ใน GHCR

### ส่วนที่มีค่าที่สุดของ workflow นี้

```bash
# ยิง request ต่อเนื่องขณะ rolling update แล้วนับว่าพลาดกี่ตัว
( for i in $(seq 1 200); do
    curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18081/healthz
    sleep 0.1
  done > /tmp/codes.txt ) &
kubectl -n todo-app rollout restart deployment/todo-api
kubectl -n todo-app rollout status deployment/todo-api --timeout=180s
bad=$(grep -vc '^200$' /tmp/codes.txt)
test "$bad" -eq 0 || exit 1
```

**นี่คือการทดสอบ zero-downtime แบบอัตโนมัติ** ซึ่งจับปัญหาที่การทดสอบแบบอื่นจับไม่ได้เลย:

- `maxUnavailable` ตั้งผิด
- readinessProbe ผ่านเร็วเกินจริง (แอปยังไม่พร้อมแต่บอกว่าพร้อม)
- ไม่มี `preStop` hook → มี request ตกช่วงที่ pod กำลังถูกถอดออกจาก Service
- แอปไม่ดัก SIGTERM → request ค้างถูกตัดกลางคัน

ทั้งสี่ข้อนี้ **ไม่มีทางเจอจากการกดทดสอบด้วยมือ** เพราะมันเกิดในช่วงเสี้ยววินาทีระหว่าง deploy

### ลองเลย

```bash
git checkout -b feature/test-e2e
git commit --allow-empty -m "test: ลอง e2e"
git push -u origin feature/test-e2e
# เปิด PR แล้วดูแท็บ Actions
```

---

## 7. Phase 2 — เส้นทาง 🅰️: PaaS (Render + Neon)

### 7.1 สร้าง service บน Render

1. สมัครที่ [render.com](https://render.com) ด้วย GitHub
2. **New +** → **Web Service** → แท็บ **Existing Image**
3. Image URL: `ghcr.io/<ชื่อคุณ>/devops-todo-api:dev` — รูปแบบเต็มคือ `ghcr.io/<owner>/<repo>:<tag>` มาจากชื่อ repo บน GitHub จริง (ตัวพิมพ์เล็กทั้งหมด) ไม่ต้องเดา ให้เข้าไปดู tag ที่มีจริงที่แท็บ **Packages** ของ repo (ต้อง push ให้ CI build-push รันผ่านอย่างน้อย 1 ครั้งก่อน — ดู [14-deploy-quickstart](14-deploy-quickstart.md#a2-สร้าง-web-service-บน-render) สำหรับขั้นตอนละเอียด)
4. Instance Type: **Free**, Region: Singapore
5. Environment Variables:

| Key | Value |
| --- | --- |
| `DATABASE_URL` | connection string ของ `todo_dev` (รวม `?sslmode=require`) |

> ⚠️ ต้องทำให้ package บน GHCR เป็น **public** ก่อน ไม่งั้น Render ดึงไม่ได้
> repo → แท็บ **Packages** → เลือก package → **Package settings** → **Change visibility** → Public

6. **Settings → Build & Deploy → Auto-Deploy → No**
   (ไม่ปิด = deploy ซ้อนสองทาง แล้วจะงงว่าของที่ขึ้นมาจากไหน)
7. **Settings → Deploy Hook** → คัดลอก URL

### 7.2 ตั้งค่าใน GitHub

**Settings → Secrets and variables → Actions**

| ประเภท | ชื่อ | ค่า |
| --- | --- | --- |
| Secret | `RENDER_DEPLOY_HOOK` | URL จากข้อ 7 |
| Variable | `DEV_URL` | `https://xxx.onrender.com` (ไม่มี `/` ท้าย) |

**Settings → Actions → General → Workflow permissions → Read and write**

### 7.3 ทดสอบ

```bash
git checkout -b dev && git push -u origin dev
```

จะเห็น 3 job เรียงกัน: `build` → `deploy` → `verify`

---

## 8. Phase 3 — เส้นทาง 🅱️: Kubernetes จริงบน Oracle Cloud

ส่วนที่ยาวที่สุดแต่ได้เยอะที่สุด

### 8.1 สร้าง VM (ARM Always Free)

1. สมัครที่ [cloud.oracle.com](https://cloud.oracle.com) — ต้องใส่บัตรเพื่อยืนยันตัวตน แต่**ไม่ถูกตัดเงินถ้าอยู่ใน Always Free**
2. **Compute → Instances → Create Instance**
3. Image: **Ubuntu 22.04**
4. Shape: **Ampere / VM.Standard.A1.Flex** → **2 OCPU / 12 GB** (เพดาน Always Free ปัจจุบัน)
5. เพิ่ม SSH key ของตัวเอง
6. Create

> 🔴 **"Out of host capacity" คือปัญหาที่เจอกันเกือบทุกคน**
> ARM ฟรีเป็นที่ต้องการสูงมาก ทางแก้: ลองสลับ Availability Domain (AD-1/2/3), ลองในช่วงเวลาอื่นของวัน,
> หรืออัปเกรดบัญชีเป็น Pay As You Go (ยังใช้ Always Free ได้เหมือนเดิม แต่ได้คิวสูงกว่า)

### 8.2 เปิด port — ต้องทำ **สองที่**

นี่คือกับดักคลาสสิกของ Oracle Cloud

**ที่ 1: VCN Security List** (Networking → VCN → Security Lists → Add Ingress Rules)

| Source | Port | ใช้ทำอะไร |
| --- | --- | --- |
| 0.0.0.0/0 | 80 | HTTP |
| 0.0.0.0/0 | 443 | HTTPS |

**ที่ 2: iptables ในเครื่อง** — Oracle Ubuntu image มี firewall ในเครื่องมาด้วย

```bash
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo netfilter-persistent save
```

**ถ้าเปิดแค่ที่เดียวจะเข้าไม่ได้ และไม่มี error บอกว่าเพราะอะไร** — คนติดตรงนี้กันเยอะมาก

### 8.3 ติดตั้ง k3s

```bash
ssh ubuntu@<PUBLIC_IP>

# --tls-san สำคัญมาก: ทำให้ certificate ของ API server ใช้กับ public IP ได้
curl -sfL https://get.k3s.io | sh -s - \
  --write-kubeconfig-mode 644 \
  --tls-san <PUBLIC_IP> \
  --disable metrics-server        # ประหยัดแรม; ถ้าจะใช้ HPA ค่อยเปิดทีหลัง

sudo systemctl status k3s
sudo k3s kubectl get nodes
```

**ทำไมเลือก k3s ไม่ใช่ k8s เต็ม:** k3s เป็น Kubernetes ที่ผ่านการรับรอง (CNCF certified) แต่รวมเป็นไบนารีเดียว ใช้แรมประมาณ 500 MB
ส่วน kubeadm ใช้เกิน 2 GB แค่ control plane — บนเครื่อง 12 GB ที่ต้องแบ่งให้แอปด้วย k3s คุ้มกว่ามาก

**k3s แถมอะไรมาให้บ้าง:** Traefik (ingress), ServiceLB, local-path storage, CoreDNS — พร้อมใช้ทันที
นี่คือเหตุผลที่ overlay `cloud` ของเราตั้ง `ingressClassName: traefik` แทน `nginx`

### 8.4 ดึง kubeconfig ออกมา

```bash
# บนเครื่อง VM
sudo cat /etc/rancher/k3s/k3s.yaml
```

ค่าที่ได้จะมี `server: https://127.0.0.1:6443` — **ต้องแก้เป็น public IP ก่อน** ไม่งั้น GitHub Actions ต่อไม่ได้

```bash
# บนเครื่องตัวเอง
scp ubuntu@<PUBLIC_IP>:/etc/rancher/k3s/k3s.yaml ./kubeconfig
sed -i '' "s/127.0.0.1/<PUBLIC_IP>/" ./kubeconfig    # macOS
kubectl --kubeconfig=./kubeconfig get nodes           # ต้องใช้ได้ก่อนถึงไปต่อ
base64 -i ./kubeconfig | pbcopy
```

### 8.5 ให้ GitHub Actions ต่อเข้าคลัสเตอร์อย่างปลอดภัย

มี 2 ทาง เลือกอย่างใดอย่างหนึ่ง

| | เปิด port 6443 สู่อินเทอร์เน็ต | **Tailscale (แนะนำ)** |
| --- | --- | --- |
| ตั้งค่า | เพิ่ม ingress rule port 6443 | ติดตั้ง tailscale ทั้งบน VM และใน workflow |
| ความปลอดภัย | ⚠️ API server ของคุณเปิดให้ทั้งโลกเห็น | ✅ อยู่ในเครือข่ายส่วนตัว ไม่มีใครนอกกลุ่มเห็น |
| ฟรีไหม | ฟรี | ✅ ฟรีถึง 3 ผู้ใช้ / 100 เครื่อง |
| เหมาะกับ | ทดลองระยะสั้น | **ใช้จริง** |

**วิธีทำแบบ Tailscale:**

```bash
# บน VM
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
tailscale ip -4        # ได้ IP แบบ 100.x.x.x — ใช้ IP นี้ใน kubeconfig แทน public IP
```

แล้วสร้าง OAuth client ที่ Tailscale admin → Settings → OAuth clients (scope `auth_keys`, tag `tag:ci`)

workflow มี step นี้รออยู่แล้ว ทำงานเองเมื่อตั้ง secret ครบ:

```yaml
- name: เชื่อม Tailscale
  if: ${{ secrets.TS_OAUTH_CLIENT_ID != '' }}
  uses: tailscale/github-action@v3
  with:
    oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}
    oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}
    tags: tag:ci
```

**ถ้าเลือกเปิด 6443 ตรง ๆ** อย่างน้อยให้จำกัด source IP เป็นช่วงของ GitHub Actions (`curl https://api.github.com/meta`)
แต่ช่วง IP นั้นกว้างมากและเปลี่ยนบ่อย — ซึ่งเป็นเหตุผลที่ Tailscale ดีกว่าในทางปฏิบัติ

### 8.6 ตั้งค่าใน GitHub

| ประเภท | ชื่อ | ค่า |
| --- | --- | --- |
| Secret | `KUBE_CONFIG` | kubeconfig ที่ base64 แล้ว |
| Secret | `DATABASE_URL` | connection string ของ `todo_prod` |
| Secret | `TS_OAUTH_CLIENT_ID` | (ถ้าใช้ Tailscale) |
| Secret | `TS_OAUTH_SECRET` | (ถ้าใช้ Tailscale) |
| Variable | `PROD_URL` | `http://todo.<PUBLIC_IP>.nip.io` |

**`DATABASE_URL` ต้องเป็น Secret ไม่ใช่ Variable** — และสังเกตว่า workflow ไม่ได้ commit มันลง git แต่สร้าง k8s Secret ให้ตอน deploy:

```yaml
- name: สร้าง/อัปเดต Secret
  run: |
    kubectl create namespace todo-app --dry-run=client -o yaml | kubectl apply -f -
    kubectl -n todo-app create secret generic todo-secret \
      --from-literal=DATABASE_URL='${{ secrets.DATABASE_URL }}' \
      --dry-run=client -o yaml | kubectl apply -f -
```

ทริก `--dry-run=client -o yaml | kubectl apply -f -` คือวิธีมาตรฐานในการทำ "create หรือ update ก็ได้"
ถ้าใช้ `kubectl create` เฉย ๆ จะพังเมื่อมี resource อยู่แล้ว

### 8.7 ตั้งชื่อโดเมนโดยไม่ต้องซื้อ

ใช้ **nip.io** — บริการ DNS ฟรีที่แปลง IP ในชื่อโดเมนกลับเป็น IP นั้น

```
todo.203.0.113.10.nip.io  →  203.0.113.10
```

แก้ `k8s/overlays/cloud/kustomization.yaml` เปลี่ยน `todo.203.0.113.10.nip.io` เป็น IP จริงของคุณ

### 8.8 เปิด HTTPS (ทำเมื่อ HTTP ใช้ได้แล้ว)

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl -n cert-manager wait --for=condition=available deploy --all --timeout=180s

cat <<'EOF' | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: you@example.com
    privateKeySecretRef: { name: letsencrypt-account }
    solvers:
      - http01:
          ingress: { class: traefik }
EOF
```

แล้วเพิ่มใน overlay:

```yaml
- target: { kind: Ingress, name: todo-ingress }
  patch: |-
    - op: add
      path: /metadata/annotations
      value:
        cert-manager.io/cluster-issuer: letsencrypt
    - op: add
      path: /spec/tls
      value:
        - hosts: [todo.203.0.113.10.nip.io]
          secretName: todo-tls
```

> ⚠️ Let's Encrypt จำกัด 5 ใบต่อโดเมนต่อสัปดาห์ — ทดสอบด้วย staging server ก่อน (`acme-staging-v02`) ไม่งั้นโดนล็อกยาว

### 8.9 Deploy

```bash
git checkout main && git push
```

---

## 9. เรื่อง ARM ที่ต้องรู้ (สำคัญมาก)

Oracle free tier เป็น **ARM64** แต่ GitHub runner เป็น **x86** — ถ้า build ตรง ๆ image จะรันบน Oracle ไม่ได้

```
exec /usr/local/bin/docker-entrypoint.sh: exec format error
```

ข้อความนี้แปลว่า **สถาปัตยกรรมของ image ไม่ตรงกับเครื่อง** — เจอครั้งเดียวก็จำไปตลอด

`build-push.yml` แก้ให้แล้วด้วย 2 บรรทัด:

```yaml
- uses: docker/setup-qemu-action@v3        # ให้ x86 จำลอง ARM ได้
...
    platforms: linux/amd64,linux/arm64
```

| | ผลที่ตามมา |
| --- | --- |
| ข้อดี | image เดียวใช้ได้ทั้ง Mac M-series, Oracle ARM, และ x86 |
| ข้อเสีย | **build ช้าขึ้น 3-10 เท่า** เพราะ QEMU จำลองทีละคำสั่ง |

นี่คือเหตุผลที่ `deploy-paas.yml` ส่ง `platforms: linux/amd64` อย่างเดียว (Render เป็น x86) ส่วน `deploy-k8s.yml` ส่งทั้งสอง
**อย่า build multi-arch ถ้าไม่ได้ใช้** — เสียเวลาฟรี ๆ

ถ้าอยากเร็วกว่านี้: ใช้ ARM runner ของ GitHub (`ubuntu-24.04-arm`, ฟรีสำหรับ public repo) แล้ว build native แทน QEMU

---

## 10. อ่าน workflow ทีละจุดสำคัญ

### `set image` ต้องเปลี่ยนทั้ง container และ initContainer

```yaml
kubectl -n todo-app set image deployment/todo-api api="$IMAGE" migrate="$IMAGE"
```

`migrate` คือ initContainer ที่รัน `migrate -path ./migrations -database "$DATABASE_URL" up`

**ถ้าลืมใส่ `migrate=`** จะเกิดสถานการณ์ที่ debug ยากมาก: container หลักเป็นโค้ดใหม่ แต่ migration ยังรันจาก image เก่า
→ schema ไม่ตรงกับโค้ด → พังแบบมีเงื่อนไข (บาง endpoint ใช้ได้ บางอันไม่ได้)

### `rollout status` คือตัวที่ทำให้ workflow แดงจริง

```yaml
kubectl -n todo-app rollout status deployment/todo-api --timeout=300s
```

`set image` คืนค่าทันทีโดยไม่รอผล — **ถ้าไม่มีบรรทัดนี้ workflow จะเขียวทั้งที่ pod เข้า `CrashLoopBackOff` อยู่**

### rollback อัตโนมัติ

```yaml
- name: ย้อนกลับอัตโนมัติเมื่อล้มเหลว
  if: failure() && steps.rollout.outcome != 'skipped'
  run: |
    kubectl -n todo-app rollout undo deployment/todo-api
    kubectl -n todo-app rollout status deployment/todo-api --timeout=180s
    kubectl -n todo-app describe pods -l app=todo-api | tail -40
```

`describe pods` ท้ายสุดสำคัญ — ตอนที่คุณมาอ่าน log พรุ่งนี้ pod ที่พังถูกลบไปแล้ว แต่ข้อความ Events ยังอยู่ในนี้

### deploy ย้อนเวอร์ชันด้วยมือ

```yaml
workflow_dispatch:
  inputs:
    image-digest:
      description: 'ระบุ digest เพื่อ deploy ย้อนเวอร์ชัน'
```

ไปที่ Actions → Deploy → Kubernetes → **Run workflow** แล้วใส่ digest เก่า
job `build` จะถูกข้ามไปเลย (`if: ${{ inputs.image-digest == '' }}`) — **deploy ของเดิมโดยไม่ build ใหม่**

นี่คือคุณสมบัติที่แยก CD ที่ใช้งานได้จริงออกจาก CD ที่เขียนไว้สวย ๆ:
**deploy ย้อนเวอร์ชันต้องไม่ต้อง build ใหม่** เพราะ build ใหม่ = ได้ image คนละตัวกับที่เคยรันได้

---

## 11. overlay `cloud` ต่างจาก `dev` ยังไง

ดูไฟล์ `k8s/overlays/cloud/kustomization.yaml`

| ปรับอะไร | ทำไม |
| --- | --- |
| ลบ Postgres StatefulSet + Service | ใช้ Neon แทน — ประหยัดแรม + มี backup |
| ลบ Secret ตัวอย่าง | workflow สร้างจาก GitHub Secret ไม่ commit ลง git |
| `ingressClassName: traefik` | k3s มี Traefik มาให้อยู่แล้ว |
| requests ลดเหลือ 50m/128Mi | node เดียวมี 2 OCPU ต้องแบ่งให้ระบบด้วย |
| เพิ่ม `preStop: sleep 5` | ให้ Service ถอด pod ออกก่อนแอปเริ่มปิด — กัน 502 |

> **ทำไมไม่มีแถว heap tuning แบบสมัย Node แล้ว:** ตอนยังเป็น TypeScript overlay นี้เคยต้องตั้ง `NODE_OPTIONS=--max-old-space-size=200` เพราะ V8 heap ไม่รู้จัก container memory limit เอง ปล่อยไว้เฉย ๆ จะโตจนโดน `OOMKilled`
> พอย้ายมา Go ปัญหานี้หายไปเลย — Go runtime อ่าน memory limit จาก cgroup ได้เองในระดับที่พอเพียงสำหรับแอปขนาดนี้ ไม่ต้องตั้งอะไรเพิ่ม (ถ้าจะบีบเพิ่มจริง ๆ มี `GOMEMLIMIT` ให้ตั้งได้ แต่ไม่จำเป็นในเคสนี้)
> ดูคอมเมนต์ในไฟล์จริงที่ `k8s/overlays/cloud/kustomization.yaml`

**`preStop` คือสิ่งที่แยก "รันได้" ออกจาก "รันได้ดี"** และเป็นบทเรียนที่มักได้มาจากการเจอปัญหาใน production

การลบ resource ออกจาก base ทำได้ด้วย `$patch: delete`:

```yaml
- target: { kind: StatefulSet, name: postgres }
  patch: |-
    $patch: delete
    apiVersion: apps/v1
    kind: StatefulSet
    metadata:
      name: postgres
```

---

## 12. เทียบสองเส้นทางหลังใช้จริง

| | 🅰️ PaaS | 🅱️ Kubernetes |
| --- | --- | --- |
| เวลาตั้งค่าครั้งแรก | 30 นาที | 2–3 ชั่วโมง |
| deploy ครั้งถัดไป | เท่ากัน (push แล้วจบ) | เท่ากัน |
| cold start | 30–60 วิ (Render) | ไม่มี |
| ควบคุม probe / rolling update | ❌ | ✅ |
| rollback | ต้อง deploy image เก่า | `rollout undo` ใน 5 วินาที |
| scale ตามโหลด | ❌ (free tier) | ✅ HPA |
| ดู log | หน้าเว็บของผู้ให้บริการ | `kubectl logs` |
| ต้อง patch OS เอง | ❌ | ✅ **ต้องทำ** |
| ค่าใช้จ่ายเมื่อโตขึ้น | จ่ายตาม instance | จ่ายตาม node |

**PaaS เหมาะกับ:** อยากให้ของขึ้นเน็ตเร็ว ๆ ไม่อยากดูแลอะไร
**Kubernetes เหมาะกับ:** อยากฝึกของจริง หรือมีหลาย service ที่ต้องอยู่ด้วยกัน

**ข้อคิดที่มักถูกมองข้าม:** Kubernetes บนเครื่องเดียวไม่ได้ทำให้ระบบทนทานขึ้นเลย
เครื่องดับ = ทุกอย่างดับ เหมือนกัน สิ่งที่ได้จริงคือ **ประสบการณ์และเครื่องมือ** ไม่ใช่ความทนทาน
ความทนทานเริ่มมีความหมายเมื่อมีหลาย node ซึ่งไม่ฟรีแล้ว

---

## 13. ปัญหาที่เจอบ่อย

**`exec format error`**
image ผิดสถาปัตยกรรม — build ด้วย `platforms: linux/arm64` แล้วหรือยัง ตรวจด้วย `docker manifest inspect <image>`

**`Out of host capacity` ตอนสร้าง VM**
ARM ฟรีเต็ม — สลับ Availability Domain, ลองเวลาอื่น, หรืออัปเป็น Pay As You Go (ยังใช้ Always Free ได้)

**เปิด port แล้วยังเข้าไม่ได้**
Oracle มี firewall สองชั้น — VCN Security List **และ** iptables ในเครื่อง ต้องเปิดทั้งคู่

**`kubectl` จาก GitHub Actions ต่อไม่ได้ (`connection refused` / `x509`)**
kubeconfig ยังชี้ `127.0.0.1` หรือลืม `--tls-san <PUBLIC_IP>` ตอนติดตั้ง k3s
แก้ `--tls-san` ต้องติดตั้ง k3s ใหม่ หรือแก้ `/etc/systemd/system/k3s.service` แล้ว restart

**Ingress ใช้ไม่ได้ ทั้งที่ pod ขึ้นปกติ**
`ingressClassName` ไม่ตรงกับ controller — k3s ใช้ `traefik` ไม่ใช่ `nginx`
ตรวจด้วย `kubectl get ingressclass`

**pod โดน `OOMKilled` ทั้งที่ดูแล้วแรมเหลือ**
สมัย Node ปัญหานี้ส่วนใหญ่มาจาก V8 heap ไม่รู้จัก container limit ต้องตั้ง `NODE_OPTIONS=--max-old-space-size` เอง — ฝั่ง Go ไม่มีปัญหานี้แล้ว (GC อ่าน limit จาก cgroup ได้เอง) ถ้ายังเจอ OOMKilled ให้สงสัย `resources.limits.memory` ตั้งไว้ต่ำเกินจริง หรือมี goroutine/connection รั่วแทน

**`dial tcp: connect: connection refused` / `context deadline exceeded` ตอน migrate ขึ้น Neon**
Neon compute หลับอยู่ ปกติตื่นเองในไม่กี่วินาที ถ้าเจอบ่อยตอน migrate ให้เติม `&connect_timeout=15` ใน DSN

**e2e ใน GitHub Actions timeout**
runner มี 2 CPU / 7 GB — ถ้าคลัสเตอร์ช้าให้ลด replica ใน overlay dev เหลือ 1 และเพิ่ม `--timeout`

**`kubectl diff` ทำให้ workflow แดง**
`kubectl diff` คืน exit code 1 เมื่อมีความต่าง (ซึ่งเป็นเรื่องปกติ) — ในไฟล์เราจึงมี `|| true` ต่อท้าย

**Render ดึง image ไม่ได้ `manifest unknown`**
package ยังเป็น private หรือ tag ที่ระบุยังไม่มี — ทดสอบด้วย `docker pull` จากเครื่องตัวเอง

---

## 14. เช็กลิสต์

**พื้นฐาน**

- [ ] `git push` แล้ว workflow ทำงานเองครบทุก job
- [ ] เปิด URL จากมือถือ (ปิด wifi) แล้วใช้งานได้จริง
- [ ] ข้อมูลใน Neon ยังอยู่หลัง deploy รอบใหม่

**พิสูจน์ว่า pipeline ทำงานจริง** (สำคัญที่สุด)

- [ ] ทำให้ `quality` พังโดยตั้งใจ → ยืนยันว่า **ไม่มีการ deploy** เกิดขึ้น
- [ ] ทำให้แอปพังตอน runtime → ยืนยันว่า `rollout status` จับได้และ **rollback อัตโนมัติ**
- [ ] แก้ manifest ให้ผิด schema → ยืนยันว่า `validate-k8s` จับได้
- [ ] ทำลาย zero-downtime (เอา `preStop` ออก) → ยืนยันว่า `k8s-e2e` จับได้

**ความปลอดภัย**

- [ ] ไม่มี `DATABASE_URL` โผล่ใน log สักที่
- [ ] `ci.yml` มี `permissions: contents: read` เท่านั้น
- [ ] API server ไม่ได้เปิดสู่อินเทอร์เน็ตแบบไม่จำกัด (ใช้ Tailscale หรือจำกัด source IP)
- [ ] ไม่มี Secret จริงอยู่ใน git

4 ข้อในหมวดที่สองสำคัญที่สุด — **pipeline ที่ไม่เคยแดงเลย คือ pipeline ที่ยังไม่รู้ว่าตัวเองใช้ได้จริงไหม**

---

## 15. ต่อยอดจากตรงนี้

| อยากทำอะไรต่อ | ไปอ่าน |
| --- | --- |
| อ่าน workflow ทั้ง 5 ไฟล์ให้ออกทุกบรรทัด | [13 — อ่านสคริปต์ deploy](13-reading-scripts.md) |
| ให้ git เป็นแหล่งความจริง (GitOps) | [11 — Enterprise GitOps](11-enterprise-gitops.md) |
| ตั้ง rate limit / probe ให้ถูกชั้น | [09 — ตั้งค่าที่ชั้นไหนดี](09-where-to-configure.md) |
| ฝึกทำเองทีละขั้น | [แบบฝึกหัด CI/CD](../exercises/cicd/03-production.md) · [Kubernetes](../exercises/kubernetes/03-production.md) |
| แก้ปัญหาที่เจอ | [12 — Troubleshooting](12-troubleshooting.md) |

---

## Sources

- [Oracle Quietly Halves Free Tier Ampere A1 Compute Limits — InfoQ](https://www.infoq.com/news/2026/07/oracle-cloud-free-tier-limits/)
- [Oracle Cloud free tier 2026: 4 OCPU/24GB cut to 2 OCPU/12GB — TerminalBytes](https://terminalbytes.com/oracle-cloud-free-tier-changes-2026/)
- [Google Kubernetes Engine pricing](https://cloud.google.com/kubernetes-engine/pricing)
- [GKE Pricing Explained: Autopilot vs Standard — CloudZero](https://www.cloudzero.com/blog/gke-pricing/)
- [Render Free Tier 2026: 750 Hours, Redis, Cron Jobs](https://unanswered.io/guide/render-free-tier-details)
- [Deploy Hooks — Render Docs](https://render.com/docs/deploy-hooks)
- [Managed Postgres free tier — Neon FAQ](https://neon.com/faqs/managed-postgres-databases-free-tier)
- [Koyeb Free Tier 2026](https://www.srvrlss.io/provider/koyeb/)

## 🪛 Playground

- [ ] เลือกทำเส้นทาง 🅲 (k3d ใน CI) ให้จบก่อน — เปิด PR แล้วดูว่า `k8s-e2e.yml` รันครบ 4 ขั้นตอนจริงไหม
- [ ] ทำ 🅰️ PaaS (Render) ให้ขึ้นจริง แล้วปิด wifi มือถือลองเปิด URL — ยืนยันว่าออกเน็ตได้จริงไม่ใช่แค่ localhost
- [ ] ตอบให้ได้ก่อนเริ่ม 🅱️: ทำไมองค์กรจริงถึงยอมเสียเวลา 2-3 ชม. ตั้ง k3s เองทั้งที่ PaaS ใช้เวลาแค่ 30 นาที — เขียนเหตุผลสัก 2-3 ข้อ แล้วเทียบกับตาราง §12
- [ ] จงใจทำให้ deploy พังกลางทาง (เช่น push manifest ที่ผิด schema) แล้วดูว่า pipeline **หยุด** ก่อนถึง production จริงไหม ตามเช็กลิสต์ §14
- [ ] ลองสลับ `platforms:` ใน `build-push.yml` เหลือ `linux/amd64` อย่างเดียว แล้ว deploy ไป Oracle (ARM) ดู `exec format error` ด้วยตาตัวเอง แล้วแก้กลับ
