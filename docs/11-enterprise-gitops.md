# 11 — Deploy ระดับองค์กร: GitHub → Actions → Harbor → Argo CD → Private Cloud (หลัง VPN)

> เอกสารนี้อธิบายสถาปัตยกรรมแบบที่บริษัทใช้จริง และวิธี implement ทีละเฟส
> ต่างจาก [10 — deploy ฟรีบน cloud](10-deploy-free-cloud.md) ตรงที่ทุกอย่างอยู่ในเครือข่ายภายใน เข้าถึงได้เฉพาะคนที่ต่อ VPN

---

## 1. ภาพรวมสถาปัตยกรรม

```
┌─ อินเทอร์เน็ต ────────────────────────────────────────────────┐
│                                                              │
│   นักพัฒนา ──push──▶ GitHub (app repo)                        │
│                          │                                   │
│                          │ webhook                           │
└──────────────────────────┼───────────────────────────────────┘
                           ▼
              ┌────────────────────────────┐
              │  Self-hosted Runner        │  ← อยู่ในวงเน็ตเวิร์กบริษัท
              │  (GitHub Actions)          │     เพราะ Harbor/K8s ไม่มี public IP
              │  build → test → scan → sign│
              └────────────┬───────────────┘
                           │ docker push
                           ▼
              ┌────────────────────────────┐
              │  Harbor (private registry) │  ← registry.company.internal
              │  scan-on-push, RBAC, sign  │
              └────────────┬───────────────┘
                           │
                           │ runner แก้ image tag แล้ว commit
                           ▼
              ┌────────────────────────────┐
              │  GitHub (config repo)      │  ← manifest/kustomize/helm values
              │  แหล่งความจริงของ desired  │     "อะไรควรรันอยู่ตอนนี้"
              │  state                     │
              └────────────┬───────────────┘
                           │
                           │ Argo CD ดึงเอง (pull) ทุก ~3 นาที หรือทันทีถ้ามี webhook
                           ▼
┌─ Private Cloud (ในองค์กร) ────────────────────────────────────┐
│                                                              │
│  ┌──────────┐   sync    ┌───────────────────────────────┐    │
│  │ Argo CD  │──────────▶│ Kubernetes (dev/uat/prod)     │    │
│  └──────────┘           │  Deployment / Service / …     │    │
│                         └──────────────┬────────────────┘    │
│                                        │                     │
│                         ┌──────────────▼────────────────┐    │
│                         │ Ingress (internal only)       │    │
│                         │ app.company.internal          │    │
│                         └──────────────┬────────────────┘    │
└────────────────────────────────────────┼─────────────────────┘
                                         │
                          ต้องต่อ VPN ก่อนถึงจะเข้าถึงได้
                                         │
                                    ผู้ใช้ / QA / นักพัฒนา
```

### จุดที่ต้องเข้าใจก่อนอย่างอื่น

**GitHub อยู่นอกองค์กร แต่ Harbor กับ Kubernetes อยู่ใน** — นี่คือข้อจำกัดที่กำหนดทุกการตัดสินใจที่เหลือ

จากข้อจำกัดนี้เกิดคำถาม 2 ข้อ:

1. runner ที่ build จะ push เข้า Harbor ยังไง ในเมื่อ Harbor ไม่มี public IP → **ต้องใช้ self-hosted runner**
2. CI จะสั่ง `kubectl apply` เข้าคลัสเตอร์ยังไง → **ไม่สั่ง** ให้ Argo CD ในคลัสเตอร์ดึงเองแทน (pull-based)

---

## 2. ทำไมต้อง GitOps (pull) แทน `kubectl apply` จาก CI (push)

| | Push-based (CI สั่ง kubectl) | **Pull-based (Argo CD)** |
| --- | --- | --- |
| ใครเริ่มการเปลี่ยนแปลง | CI ข้างนอกยิงเข้ามา | ตัวคลัสเตอร์ดึงออกไปเอง |
| ต้องเปิดทางเข้าคลัสเตอร์จากภายนอกไหม | ✅ ต้องเปิด (หรือต้องมี runner ในวง) | ❌ **ไม่ต้องเปิดเลย** — ปลอดภัยกว่ามาก |
| credential ของคลัสเตอร์ | อยู่ใน CI (หลุดแล้วซวย) | ไม่ต้องมีอยู่นอกคลัสเตอร์เลย |
| ถ้ามีคนแก้ของใน cluster ด้วยมือ | ไม่มีใครรู้ | **ตรวจเจอทันทีว่า OutOfSync** และแก้กลับให้ได้ |
| อยากรู้ว่าตอนนี้ prod รันอะไรอยู่ | ต้องไปถามคลัสเตอร์ | ดูที่ git ก็รู้ |
| rollback | รัน pipeline ย้อน | `git revert` |
| audit ว่าใครเปลี่ยนอะไรเมื่อไร | ดู log ของ CI | **git history + PR review** |

ประโยคที่สรุปทั้งหมดได้: **git คือแหล่งความจริง คลัสเตอร์แค่ตามให้ทัน**

---

## 3. โครงสร้าง repo — แยก 2 repo เสมอ

```
company/todo-api              ← app repo: โค้ด, Dockerfile, workflow CI
company/todo-api-config       ← config repo: manifest ที่ Argo CD มอง
```

### หน้าตาของ config repo

```
todo-api-config/
├── base/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ingress.yaml
└── overlays/
    ├── dev/
    │   ├── kustomization.yaml       # image tag: dev-a1b2c3d
    │   └── patches/replicas.yaml
    ├── uat/
    │   └── kustomization.yaml       # image tag: uat-… (คนละตัวกับ dev)
    └── prod/
        ├── kustomization.yaml       # image tag: v1.4.2 (semver เท่านั้น)
        └── patches/hpa.yaml
```

### ทำไมต้องแยก 2 repo (ไม่ใช่ทำในโฟลเดอร์เดียวกัน)

| เหตุผล | อธิบาย |
| --- | --- |
| **ตัดวงจร CI ไม่รู้จบ** | CI commit image tag กลับเข้า repo เดิม → trigger CI ใหม่ → commit อีก → วนไม่จบ |
| **สิทธิ์คนละชุด** | dev ทุกคนแก้โค้ดได้ แต่ **แก้ overlay ของ prod ได้เฉพาะคนที่ได้รับอนุญาต** |
| **history อ่านรู้เรื่อง** | history ของ config repo คือประวัติการ deploy ล้วน ๆ ไม่ปนกับ commit โค้ด |
| **ใช้กับหลาย service ได้** | config repo เดียวดูแลได้หลายแอป |

> ถ้าองค์กรยืนยันจะใช้ repo เดียว ต้องใส่ `[skip ci]` ใน commit ที่บอทสร้าง และตั้ง `paths-ignore` ให้ workflow — แต่ยังเจอปัญหาเรื่องสิทธิ์อยู่ดี

---

## 4. เฟสการ implement

### Phase 0 — สำรวจก่อนลงมือ

ตอบให้ได้ก่อนเขียนอะไรสักบรรทัด:

- Harbor URL, ใครเป็นแอดมิน, มี project ให้ทีมเราหรือยัง
- Kubernetes มีกี่คลัสเตอร์ แยก dev/uat/prod หรือแยกแค่ namespace
- Argo CD ติดตั้งไว้แล้วหรือยัง อยู่คลัสเตอร์ไหน ใครดูแล
- VPN ใช้อะไร (OpenVPN / WireGuard / Cisco AnyConnect / Zero-trust) ขอสิทธิ์ยังไง
- DNS ภายในชื่ออะไร ใครเพิ่ม record ได้
- มี self-hosted runner อยู่แล้วไหม หรือทีมเราต้องตั้งเอง
- ระบบจัดการ secret ใช้อะไร (Vault / Sealed Secrets / External Secrets)

**อย่าข้ามเฟสนี้** — 80% ของความล่าช้าในองค์กรมาจากการรอสิทธิ์ ไม่ใช่จากการเขียนโค้ด

---

### Phase 1 — Self-hosted Runner

GitHub-hosted runner อยู่บนอินเทอร์เน็ต เข้าถึง `registry.company.internal` ไม่ได้ จึงต้องมี runner ในวงเน็ตเวิร์กเรา

**ทางเลือกในการติดตั้ง:**

| แบบ | เหมาะกับ | ข้อดี/ข้อเสีย |
| --- | --- | --- |
| VM ธรรมดา + service | เริ่มต้นเร็ว | ง่าย แต่ scale ไม่ได้ และ build ปนกันระหว่างงาน |
| **Actions Runner Controller (ARC) บน k8s** | องค์กรที่มี k8s อยู่แล้ว | **แนะนำ** — runner เกิด/ดับตามงาน แต่ละ job ได้ pod สะอาดใหม่ |
| runner ในคอนเทนเนอร์ | ระดับกลาง | ต้องจัดการ docker-in-docker เอง |

**ตัวอย่างการลงทะเบียนแบบ VM (ให้เห็นภาพ):**

```bash
mkdir actions-runner && cd actions-runner
curl -o actions-runner-linux-x64.tar.gz -L https://github.com/actions/runner/releases/download/v2.x.x/…
tar xzf ./actions-runner-linux-x64.tar.gz
./config.sh --url https://github.com/company/todo-api --token <TOKEN> --labels self-hosted,linux,internal
sudo ./svc.sh install && sudo ./svc.sh start
```

แล้วใน workflow เปลี่ยนแค่บรรทัดเดียว:

```yaml
jobs:
  build:
    runs-on: [self-hosted, linux, internal]   # แทน ubuntu-latest
```

**ข้อควรระวังด้านความปลอดภัยที่สำคัญมาก:**

- ❌ **ห้ามใช้ self-hosted runner กับ public repo เด็ดขาด** — ใครก็เปิด PR ที่รันโค้ดอะไรก็ได้ในเครือข่ายภายในของคุณ
- ให้ runner อยู่ใน network segment ของตัวเอง เปิด egress เท่าที่จำเป็น (GitHub, Harbor, npm registry)
- ใช้ ephemeral runner (ทำงานเสร็จแล้วทิ้ง) เพื่อไม่ให้งานหนึ่งทิ้งของไว้ให้อีกงานเจอ
- จำกัดว่า workflow ไหนใช้ runner label นี้ได้บ้าง

---

### Phase 2 — Harbor

#### 2.1 โครงสร้าง project

```
harbor.company.internal/
├── todo-api/           ← project ของทีม (private)
├── base-images/        ← base image ที่องค์กรรับรองแล้ว
└── dockerhub-proxy/    ← proxy cache ของ Docker Hub
```

**proxy cache** สำคัญกว่าที่คิด — ในองค์กรที่ออกเน็ตไม่ได้ตรง ๆ หรือโดน rate limit ของ Docker Hub
ให้ตั้ง Harbor เป็น proxy cache แล้วเปลี่ยน `FROM node:22-alpine` เป็น `FROM harbor.company.internal/dockerhub-proxy/library/node:22-alpine`

#### 2.2 Robot Account (ห้ามใช้บัญชีคน)

Harbor → Project → **Robots** → New Robot Account

| Robot | สิทธิ์ | ใช้ที่ไหน |
| --- | --- | --- |
| `robot$todo-api+ci` | push, pull | GitHub Actions |
| `robot$todo-api+k8s` | **pull อย่างเดียว** | imagePullSecret ในคลัสเตอร์ |

**หลักการ least privilege:** ตัวที่ deploy ไม่จำเป็นต้อง push ได้ ถ้า token ของคลัสเตอร์หลุด คนร้ายก็ยัง push image ปลอมไม่ได้

เก็บ token ไว้ที่ GitHub Secrets: `HARBOR_USERNAME`, `HARBOR_PASSWORD`

#### 2.3 นโยบายที่ควรเปิด

| ตั้งค่า | ที่ไหน | ทำไม |
| --- | --- | --- |
| **Automatically scan images on push** | Project → Configuration | รู้ทันทีว่า image ที่เพิ่ง build มีช่องโหว่ไหม |
| **Prevent vulnerable images from running** + severity `High` | Project → Configuration | Harbor ปฏิเสธการ pull image ที่มีช่องโหว่ระดับ High ขึ้นไป |
| **Content Trust / cosign verification** | Project → Configuration | pull ได้เฉพาะ image ที่มีลายเซ็น |
| **Tag Retention** | Project → Policy | เก็บ tag ล่าสุด N ตัวต่อ repo ที่เหลือลบ — ไม่งั้นดิสก์เต็มใน 6 เดือน |
| **Tag Immutability** | Project → Policy | ห้าม push ทับ tag เดิม — สำคัญมากสำหรับ tag ที่ขึ้น prod |
| **Quota** | Project → Configuration | กันทีมเดียวกินดิสก์หมด |
| **Replication** | Administration → Replication | ซิงก์ image ไป data center อื่น / ดึงจาก registry ภายนอกเข้ามา |

**Tag Immutability คือสิ่งที่ต้องเปิดตั้งแต่วันแรก** — ถ้า tag `v1.4.2` ถูก push ทับได้ แปลว่า "เวอร์ชันที่ผ่าน QA" กับ "เวอร์ชันที่ขึ้น prod" อาจเป็นคนละอันโดยไม่มีใครรู้

#### 2.4 กลยุทธ์การตั้ง tag

| environment | รูปแบบ tag | ตัวอย่าง | เปลี่ยนทับได้ไหม |
| --- | --- | --- | --- |
| dev | `dev-<short-sha>` | `dev-a1b2c3d` | ไม่ (immutable) |
| uat | `uat-<short-sha>` หรือ rc | `uat-a1b2c3d`, `v1.5.0-rc.1` | ไม่ |
| prod | **semver เท่านั้น** | `v1.4.2` | ไม่ (ล็อกด้วย immutability) |

และไม่ว่าจะ tag อะไร **ตอน deploy ให้ใช้ digest เสมอ**:

```
harbor.company.internal/todo-api/api@sha256:9f2e…
```

digest คือ hash ของ image เอง ปลอมไม่ได้ ชี้ผิดตัวไม่ได้

---

### Phase 3 — CI บน GitHub Actions

หน้าที่ของ CI ในสถาปัตยกรรมนี้มี 5 อย่าง แล้วจบ — **ไม่มี `kubectl` อยู่ใน CI เลย**

```yaml
name: CI

on:
  push:
    branches: [main, develop]

jobs:
  build:
    runs-on: [self-hosted, linux, internal]
    steps:
      - uses: actions/checkout@v4

      # 1) ทดสอบ
      - run: npm ci && npm run typecheck && npm test

      # 2) build + push เข้า Harbor
      - uses: docker/login-action@v3
        with:
          registry: harbor.company.internal
          username: ${{ secrets.HARBOR_USERNAME }}
          password: ${{ secrets.HARBOR_PASSWORD }}

      - id: build
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: harbor.company.internal/todo-api/api:dev-${{ github.sha }}

      # 3) สแกนซ้ำฝั่งเรา (Harbor สแกนให้อยู่แล้ว แต่เราอยากให้ pipeline แดงเองด้วย)
      - uses: aquasecurity/trivy-action@master
        with:
          image-ref: harbor.company.internal/todo-api/api:dev-${{ github.sha }}
          severity: HIGH,CRITICAL
          exit-code: "1"

      # 4) เซ็นลายเซ็น
      - run: cosign sign --key env://COSIGN_KEY harbor.company.internal/todo-api/api@${{ steps.build.outputs.digest }}
        env:
          COSIGN_KEY: ${{ secrets.COSIGN_KEY }}

      # 5) อัปเดต config repo — ขั้นตอนที่เชื่อม CI เข้ากับ CD
      - name: bump image ใน config repo
        run: |
          git clone https://x-access-token:${{ secrets.CONFIG_REPO_TOKEN }}@github.com/company/todo-api-config.git
          cd todo-api-config/overlays/dev
          kustomize edit set image api=harbor.company.internal/todo-api/api@${{ steps.build.outputs.digest }}
          git config user.name "ci-bot"
          git commit -am "chore(dev): deploy ${{ github.sha }}"
          git push
```

step ที่ 5 คือจุดเชื่อม CI↔CD ทั้งหมด — **CI ไม่ deploy อะไรเลย มันแค่ "เขียนความต้องการ" ลง git** แล้ว Argo CD จะไปทำให้เป็นจริง

#### ทางเลือก: Argo CD Image Updater แทน step ที่ 5

Argo CD Image Updater เป็นตัวที่คอยส่องว่ามี image ใหม่ใน registry ไหม แล้ว commit กลับเข้า git ให้เอง

| | CI commit เอง (แนะนำ) | Image Updater |
| --- | --- | --- |
| ควบคุมว่า commit ตอนไหน | ✅ แม่นยำ | ตาม polling |
| ต้องให้ CI มี token ของ config repo | ✅ ต้องมี | ❌ ไม่ต้อง |
| ผูก deploy กับ commit ของโค้ดได้ | ✅ ตรงไปตรงมา | ต้องอ่านจาก annotation |
| เหมาะกับ | สายงานที่ต้องการความชัดเจน | dev/staging ที่อยากให้อัตโนมัติเต็มที่ |

---

### Phase 4 — Argo CD

#### 4.1 Application พื้นฐาน

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: todo-api-dev
  namespace: argocd
spec:
  project: team-backend
  source:
    repoURL: https://github.com/company/todo-api-config.git
    targetRevision: main
    path: overlays/dev
  destination:
    server: https://kubernetes.default.svc
    namespace: todo-dev
  syncPolicy:
    automated:
      prune: true # ลบของที่หายไปจาก git ออกจากคลัสเตอร์ด้วย
      selfHeal: true # มีคนแก้ด้วยมือ → ดึงกลับให้ตรง git
    syncOptions:
      - CreateNamespace=true
    retry:
      limit: 3
      backoff: { duration: 10s, factor: 2 }
```

**ความหมายของสองบรรทัดที่สำคัญที่สุด:**

- `prune: true` — ลบ resource ที่ถูกลบออกจาก git แล้ว ถ้าไม่เปิด ของเก่าจะค้างในคลัสเตอร์ตลอดกาล
- `selfHeal: true` — ใครแก้ด้วย `kubectl edit` Argo จะดึงกลับให้ตรง git ภายในไม่กี่วินาที
  **นี่คือสิ่งที่ทำให้ "git คือความจริง" เป็นเรื่องจริง ไม่ใช่แค่คำพูด**

#### 4.2 ตั้งค่าต่างกันตาม environment

| environment | automated sync | selfHeal | ใครอนุมัติ |
| --- | --- | --- | --- |
| **dev** | ✅ | ✅ | ไม่ต้อง — push แล้วขึ้นเลย |
| **uat** | ✅ | ✅ | PR ต้องมีคน review |
| **prod** | ❌ **manual sync** | ✅ | PR + คนกด Sync ใน Argo UI |

prod ควรปิด auto-sync ในช่วงแรก เพื่อให้มีมนุษย์เป็นด่านสุดท้าย พอทีมมั่นใจในระบบทดสอบแล้วค่อยเปิด

#### 4.3 ApplicationSet — เลิกก็อป Application ทีละอัน

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: todo-api
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - env: dev
            autoSync: "true"
          - env: uat
            autoSync: "true"
          - env: prod
            autoSync: "false"
  template:
    metadata:
      name: "todo-api-{{env}}"
    spec:
      project: team-backend
      source:
        repoURL: https://github.com/company/todo-api-config.git
        targetRevision: main
        path: "overlays/{{env}}"
      destination:
        server: https://kubernetes.default.svc
        namespace: "todo-{{env}}"
```

Git Directory generator ยิ่งสะดวกกว่า — สร้างโฟลเดอร์ใหม่ใน `overlays/` แล้ว Application เกิดเอง

#### 4.4 App-of-Apps

Application หนึ่งตัวที่ชี้ไปยังโฟลเดอร์ที่เก็บ Application ตัวอื่น ๆ ทั้งหมด
ผลคือ **การเพิ่มบริการใหม่เข้าคลัสเตอร์ = เปิด PR** ไม่ต้องมีใคร `kubectl apply` ด้วยมืออีก

#### 4.5 Sync Waves — ลำดับการ apply

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "-1" # ยิ่งน้อยยิ่งมาก่อน
```

| wave | ใส่อะไร |
| --- | --- |
| -2 | Namespace, ConfigMap, Secret |
| -1 | **Job สำหรับรัน database migration** |
| 0 | Deployment, Service (ค่าเริ่มต้น) |
| 1 | Ingress, HPA |

wave -1 คือคำตอบของคำถาม *"migration ควรรันตรงไหนใน GitOps"* — ทำเป็น Job + `PreSync` hook แล้ว Argo จะรอให้จบก่อนค่อย apply Deployment

```yaml
metadata:
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: BeforeHookCreation
```

#### 4.6 RBAC และ Project

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: team-backend
  namespace: argocd
spec:
  sourceRepos:
    - https://github.com/company/todo-api-config.git # จำกัดว่าดึงได้จาก repo ไหน
  destinations:
    - namespace: "todo-*" # แตะได้เฉพาะ namespace ที่ขึ้นต้นด้วย todo-
      server: https://kubernetes.default.svc
  clusterResourceWhitelist: [] # ห้ามสร้าง cluster-scoped resource
```

`AppProject` คือรั้วกั้นระหว่างทีม ถ้าไม่ตั้ง ทีมหนึ่งจะ deploy ทับ namespace ของอีกทีมได้

ต่อ SSO ขององค์กร (OIDC/LDAP) แล้วแมป group → role:

```
p, role:backend-dev, applications, sync, team-backend/*-dev, allow
p, role:backend-dev, applications, sync, team-backend/*-prod, deny
g, company:backend-team, role:backend-dev
```

---

### Phase 5 — Secret

**ห้าม commit Secret ลง config repo** แม้ repo จะ private ก็ตาม เพราะ base64 ไม่ใช่การเข้ารหัส

| วิธี | หลักการ | เหมาะกับ |
| --- | --- | --- |
| **Sealed Secrets** | เข้ารหัสด้วย public key ของคลัสเตอร์ commit ได้ ถอดได้เฉพาะในคลัสเตอร์นั้น | องค์กรที่ยังไม่มี Vault |
| **External Secrets Operator** | ใน git มีแค่ "ตัวชี้" ค่าจริงอยู่ใน Vault | **องค์กรที่มี Vault อยู่แล้ว — แนะนำ** |
| SOPS + age/KMS | เข้ารหัสไฟล์ทั้งไฟล์ | ทีมเล็กที่ชอบทำงานกับไฟล์ |

ตัวอย่าง External Secrets:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: todo-db
spec:
  secretStoreRef: { name: vault-backend, kind: ClusterSecretStore }
  target: { name: todo-secret }
  data:
    - secretKey: DATABASE_URL
      remoteRef: { key: todo-api/prod, property: database_url }
```

ใน git มีแต่ path ไปหาค่า ไม่มีค่าจริง — คนที่อ่าน repo ได้ ก็ยังไม่รู้รหัสผ่าน

**imagePullSecret สำหรับ Harbor:**

```bash
kubectl -n todo-dev create secret docker-registry harbor-cred \
  --docker-server=harbor.company.internal \
  --docker-username='robot$todo-api+k8s' \
  --docker-password='<token>'
```

แล้วผูกกับ ServiceAccount `default` ของ namespace เพื่อไม่ต้องใส่ในทุก Deployment:

```bash
kubectl -n todo-dev patch serviceaccount default \
  -p '{"imagePullSecrets":[{"name":"harbor-cred"}]}'
```

---

### Phase 6 — เครือข่ายภายในและ VPN

#### 6.1 ทำให้เว็บออกสู่ภายนอกไม่ได้จริง ๆ

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: todo-api
  annotations:
    # ใช้ ingress controller ตัวที่ผูกกับ LB วงใน ไม่ใช่ตัวที่มี public IP
    kubernetes.io/ingress.class: nginx-internal
    nginx.ingress.kubernetes.io/whitelist-source-range: "10.0.0.0/8,172.16.0.0/12"
spec:
  ingressClassName: nginx-internal
  rules:
    - host: todo-api.dev.company.internal
```

การป้องกันที่ถูกต้องมี **3 ชั้นซ้อนกัน** ไม่ใช่ชั้นเดียว:

1. **Ingress controller แยกตัว** — ตัวที่ใช้ผูกกับ Service type LoadBalancer ที่เป็น internal LB (ไม่มี public IP)
2. **whitelist-source-range** — รับเฉพาะ IP ในวงบริษัทและวง VPN
3. **NetworkPolicy** — จำกัดว่า pod ไหนคุยกับ pod ไหนได้ภายในคลัสเตอร์

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: todo-api-allow
spec:
  podSelector: { matchLabels: { app: todo-api } }
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector: { matchLabels: { name: ingress-nginx } }
      ports: [{ port: 3000 }]
```

#### 6.2 DNS

ต้องมี **split-horizon DNS** — ชื่อ `*.company.internal` ต้องตอบเฉพาะจาก resolver ภายใน
ถ้าตอบจากภายนอกได้ แปลว่าเปิดเผยผังเครือข่ายภายในให้คนนอกโดยไม่จำเป็น

| ชื่อ | ชี้ไป |
| --- | --- |
| `harbor.company.internal` | Harbor |
| `argocd.company.internal` | Argo CD UI |
| `todo-api.dev.company.internal` | internal ingress ของ dev |
| `todo-api.company.internal` | internal ingress ของ prod |

#### 6.3 VPN

| แบบ | ตัวอย่าง | ลักษณะ |
| --- | --- | --- |
| **Full-tunnel VPN** | Cisco AnyConnect, OpenVPN | traffic ทุกอย่างวิ่งผ่านบริษัท ควบคุมง่ายแต่ช้าและกิน bandwidth |
| **Split-tunnel VPN** | WireGuard | เฉพาะ subnet ของบริษัทที่วิ่งผ่าน VPN — เร็วกว่ามาก |
| **Zero-trust** | Tailscale, Cloudflare Access, Teleport | ตรวจตัวตนรายบุคคล+รายเครื่องต่อ service ไม่ใช่ให้สิทธิ์ทั้งวง — **ทิศทางที่องค์กรกำลังย้ายไป** |

**สิ่งที่คนมักเข้าใจผิด: VPN ไม่ใช่ระบบยืนยันตัวตนของแอป**
ต่อ VPN ได้ = อยู่ในวงเน็ตเวิร์ก **ไม่ได้แปลว่าเป็นคนที่มีสิทธิ์ใช้แอปนั้น**
แอปยังต้องมี authentication/authorization ของตัวเองเสมอ (หลักการ zero-trust: อย่าเชื่อใครเพียงเพราะเขาอยู่ในวงเดียวกัน)

#### 6.4 ผลกระทบต่อการทำงานประจำวัน

| งาน | ทำยังไงเมื่อทุกอย่างอยู่หลัง VPN |
| --- | --- |
| QA ทดสอบ | ต้องต่อ VPN ก่อน — ต้องเตรียมสิทธิ์ให้ QA ตั้งแต่แรก ไม่ใช่ค่อยมาขอตอนจะเทส |
| ทดสอบจากมือถือ | ติดตั้ง VPN client บนมือถือ หรือใช้ zero-trust ที่รองรับ |
| webhook จากภายนอก (เช่น payment gateway) | ต้องเปิด reverse proxy เฉพาะ path นั้นออกสู่ภายนอก — ต้องคุยกับทีม network ล่วงหน้า |
| ดู log / metrics | Grafana, Kibana ก็อยู่หลัง VPN เหมือนกัน |
| pull image ลงเครื่องตัวเอง | ต่อ VPN แล้ว `docker login harbor.company.internal` |
| Argo CD รับ webhook จาก GitHub | **GitHub ยิงเข้ามาไม่ได้** เพราะ Argo อยู่วงใน → Argo จะใช้วิธี poll ทุก 3 นาทีแทน ถ้าอยากได้ทันทีต้องมี relay หรือ self-hosted runner เป็นตัวยิงให้ |

ข้อสุดท้ายเป็นเรื่องที่ทำให้หลายคนสับสนว่า "ทำไม deploy ช้า 3 นาที" — คำตอบคือ Argo poll อยู่ ไม่ใช่ระบบพัง

---

## 5. เส้นทางการเลื่อนขั้น (Promotion)

```
feature/* ──PR──▶ develop ──▶ [CI] ──▶ Harbor: dev-a1b2c3d
                                          │
                                          ▼
                          config repo: overlays/dev  (บอทแก้ให้อัตโนมัติ)
                                          │
                                    Argo sync → dev ✅
                                          │
                         ทดสอบผ่าน → เปิด PR แก้ overlays/uat
                                          │
                                    Argo sync → uat ✅
                                          │
                       QA ผ่าน → tag v1.5.0 → PR แก้ overlays/prod
                                          │
                              ผู้มีสิทธิ์กด Sync ใน Argo → prod ✅
```

**หลักที่ห้ามละเมิด: image ที่ขึ้น prod ต้องเป็น digest เดียวกับที่ผ่าน uat มาแล้ว**
ห้าม build ใหม่สำหรับ prod เด็ดขาด — ถ้า build ใหม่ แปลว่าสิ่งที่ทดสอบกับสิ่งที่ขึ้นจริงเป็นคนละก้อน ทดสอบมาทั้งหมดก็สูญเปล่า

การเลื่อนขั้นจึงเป็นแค่ "แก้ digest ในไฟล์ overlay" ไม่ใช่การ build

---

## 6. Rollback

| สถานการณ์ | วิธีทำ | เวลาที่ใช้ |
| --- | --- | --- |
| เพิ่ง deploy แล้วพังทันที | `git revert` ใน config repo แล้วปล่อยให้ Argo sync | 1–3 นาที |
| ต้องการเร็วที่สุด | Argo UI → History → Rollback | ~30 วินาที |
| กรณีฉุกเฉินสุดขีด | `kubectl rollout undo` | ทันที |

⚠️ วิธีที่ 2 และ 3 ทำให้ **คลัสเตอร์ไม่ตรงกับ git** → Argo จะขึ้น OutOfSync (และถ้าเปิด selfHeal มันจะดึงกลับไปเวอร์ชันพังอีก!)
**ใช้ได้เฉพาะเป็นการห้ามเลือดชั่วคราว แล้วต้องรีบ commit ลง git ตามทันที**

นี่คือกฎที่ต้องเขียนไว้ใน runbook ให้ชัด ไม่งั้นคืนที่เกิดเหตุจะมีคน rollback แล้วงงว่าทำไมของพังกลับมาเอง

---

## 7. กับดักที่เจอบ่อยในองค์กร

| ปัญหา | สาเหตุ | ทางแก้ |
| --- | --- | --- |
| Argo ขึ้น OutOfSync ตลอดแม้ไม่มีใครแก้ | มี controller อื่น (HPA, mutating webhook, service mesh) เขียนค่าทับ | ใส่ `ignoreDifferences` สำหรับ field นั้น เช่น `/spec/replicas` เมื่อใช้ HPA |
| deploy แล้วไม่มีอะไรเปลี่ยน | tag เดิม image ใหม่ (mutable tag) k8s เลยไม่ pull ใหม่ | ใช้ digest + เปิด Tag Immutability ใน Harbor |
| CI วนไม่จบ | บอท commit เข้า repo ที่ trigger CI ตัวเอง | แยก config repo หรือใส่ `[skip ci]` |
| pod ขึ้น `ImagePullBackOff` | robot account หมดอายุ / ไม่มี imagePullSecret ใน namespace ใหม่ | ตั้ง token ไม่มีวันหมดอายุแล้วหมุนตามรอบ + ผูก secret กับ ServiceAccount |
| ดิสก์ Harbor เต็ม | ไม่ได้ตั้ง retention และไม่ได้รัน GC | Tag Retention + Garbage Collection ตามตาราง |
| migration รันพร้อมกันหลาย pod | ใส่ migration ไว้ใน CMD ของแอป | ย้ายไป PreSync hook Job |
| prod พังเพราะ config ต่างจาก uat | overlay ของแต่ละ env เขียนแยกกันจนหลุดจากกัน | ให้ base เป็นตัวหลัก overlay มีแค่ส่วนต่างจริง ๆ แล้ว `kubectl diff` เทียบกันเป็นระยะ |
| ไม่มีใครรู้ว่า prod รันเวอร์ชันไหน | deploy ด้วยมือแทรกเข้ามา | เปิด selfHeal + ปิดสิทธิ์ `kubectl` write ของคนใน prod |

---

## 8. เช็กลิสต์ก่อนบอกว่า pipeline พร้อมใช้งานจริง

**ความปลอดภัย**

- [ ] self-hosted runner ไม่ได้ผูกกับ public repo
- [ ] robot account ของคลัสเตอร์เป็น pull-only
- [ ] เปิด scan-on-push และตั้ง prevent vulnerable images
- [ ] เปิด Tag Immutability สำหรับ project ที่ขึ้น prod
- [ ] ไม่มี secret อยู่ใน git ทุก repo (ตรวจด้วย gitleaks)
- [ ] Argo CD ผูกกับ SSO และตั้ง RBAC แยกสิทธิ์ dev/prod แล้ว

**ความถูกต้อง**

- [ ] deploy ด้วย digest ไม่ใช่ tag ลอย
- [ ] image ที่ขึ้น prod คือ digest เดียวกับที่ผ่าน uat
- [ ] migration รันเป็น PreSync hook ไม่ใช่ใน CMD ของแอป
- [ ] เปิด `prune` และ `selfHeal` แล้ว (อย่างน้อยใน dev/uat)

**การกู้คืน**

- [ ] ซ้อม rollback จริงอย่างน้อย 1 ครั้ง จับเวลาได้
- [ ] runbook ระบุชัดว่าเมื่อไรใช้ `git revert` เมื่อไรใช้ Argo rollback
- [ ] มีคนอย่างน้อย 2 คนที่ทำ rollback เป็น

**เครือข่าย**

- [ ] ยืนยันแล้วว่าจากเน็ตข้างนอก (ปิด VPN) เข้าเว็บไม่ได้จริง
- [ ] QA และผู้เกี่ยวข้องได้สิทธิ์ VPN แล้ว
- [ ] แอปมี authentication ของตัวเอง ไม่ได้พึ่ง VPN อย่างเดียว

---

## 9. เทียบกับที่เราทำในโปรเจกต์นี้

| | โปรเจกต์ฝึก (docs 06-08, 10) | องค์กร (เอกสารนี้) |
| --- | --- | --- |
| Registry | GHCR (public cloud) | Harbor (ในองค์กร) |
| Runner | GitHub-hosted | Self-hosted ในวงเน็ตเวิร์ก |
| วิธี deploy | CI สั่ง `kubectl set image` (push) | Argo CD ดึงจาก git (pull) |
| แหล่งความจริง | workflow ของ CI | **config repo** |
| Secret | GitHub Secrets | Vault / External Secrets |
| เข้าถึงเว็บ | สาธารณะ | เฉพาะหลัง VPN |
| Rollback | `kubectl rollout undo` ใน workflow | `git revert` |

**เส้นทางการเรียนรู้ที่แนะนำ:** ทำแบบ push-based ให้เข้าใจก่อน (โปรเจกต์นี้) แล้วค่อยเข้าใจว่าทำไมองค์กรถึงย้ายไป pull-based
คนที่ข้ามมา Argo CD เลยมักจะใช้เป็นแต่ไม่เข้าใจว่ามันแก้ปัญหาอะไรให้

---

## Sources

- [ArgoCD Best Practices for CI/CD Integration](https://oneuptime.com/blog/post/2026-02-26-argocd-best-practices-cicd-integration/view)
- [Argo CD GitOps Tutorial 2026](https://tutorials.technology/tutorials/argocd-gitops-tutorial-2026.html)
- [Getting Started With Argo CD Image Updater](https://octopus.com/devops/argo-cd/argo-cd-image-updater/)
- [How to Run Harbor Container Registry with Vulnerability Scanning](https://oneuptime.com/blog/post/2026-02-08-how-to-run-harbor-container-registry-with-vulnerability-scanning/view)
- [Container Registry Security Hardening: Harbor + Trivy + RBAC](https://www.hostmycode.com/blog/container-registry-security-hardening-harbor-trivy-scanner-rbac-dedicated-servers)
- [Harbor 2.0 OCI support — Harbor blog](https://goharbor.io/blog/harbor-2.0/)

➡️ [12 — แก้ปัญหาที่เจอบ่อย](12-troubleshooting.md) · 🏋️ [แบบฝึกหัด CI/CD](../exercises/cicd/01-beginner.md)
