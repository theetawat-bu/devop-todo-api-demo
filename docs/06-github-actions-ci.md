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

- uses: actions/setup-node@v4
  with:
    node-version: "22"
    cache: "npm" # cache ~/.npm ตาม hash ของ package-lock.json

- run: npm ci # ติดตั้งตาม lockfile เป๊ะ ๆ
- run: npx prisma generate # ต้องมี ไม่งั้น type ของ prisma ไม่มี
- run: npm run typecheck # tsc --noEmit
- run: npm run build
- run: npx prisma migrate deploy # ทดสอบว่า migration รันผ่านจริง
```

`cache: 'npm'` ช่วยลดเวลา install จากหลักนาทีเหลือหลักวินาที

**pin เวอร์ชันของ action เสมอ** (`@v4` ไม่ใช่ `@main`) เพื่อไม่ให้ pipeline พังเองวันดีคืนดี

## Smoke test

step สุดท้ายสตาร์ท server จริงแล้วยิง endpoint:

```bash
node dist/index.js &
for i in $(seq 1 20); do curl -fsS localhost:3000/healthz && break; sleep 1; done
curl -fsS localhost:3000/readyz
curl -fsS -X POST localhost:3000/api/todos -H 'Content-Type: application/json' -d '{"title":"from ci"}'
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST localhost:3000/api/todos -H 'Content-Type: application/json' -d '{}')
test "$code" = "400" || exit 1
```

- `curl -f` ทำให้ exit code ไม่เป็น 0 เมื่อได้ 4xx/5xx → step ล้มเหลวเอง
- loop รอ server พร้อม ดีกว่า `sleep 10` แบบสุ่ม
- เช็ค 400 ด้วย เพราะ "path ที่ควรพัง ต้องพังให้ถูกวิธี" ก็เป็นพฤติกรรมที่ต้องทดสอบ

> smoke test นี้ตั้งใจให้เข้าใจง่าย ของจริงควรอัปเกรดเป็น Jest + Supertest (ดู [แบบฝึกหัด](09-exercises.md))

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

➡️ ต่อไป: [07 — CD + GHCR](07-cd-ghcr.md)
