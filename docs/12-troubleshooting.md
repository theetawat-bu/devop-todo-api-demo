# 12 — แก้ปัญหาที่เจอบ่อย

## Go / Migrations

**`DATABASE_URL is not set`**
ยังไม่มี `.env` → `cp .env.example .env`
บน k8s แปลว่า Secret ไม่ถูก mount ตรวจ `kubectl -n todo-app describe pod <pod>`

**`Can't reach database server at localhost:5432` / `dial tcp ... connect: connection refused`**
ต่อ DB ผิด host — ใน container `localhost` คือตัว container เอง
ใช้ `db` (compose) หรือ `postgres` (k8s) เป็น hostname

**`lookup db: no such host` / `lookup postgres: no such host`**
กลับกันกับข้อบน — รัน `go run ./cmd/api` **ตรงจากเครื่อง** (นอก container) แต่ `DATABASE_URL` ใน `.env` ยังตั้ง host เป็น `db`/`postgres`
ชื่อ service แบบนี้ resolve ได้แค่ในวง network ของ docker compose/k8s เท่านั้น ถ้ารันนอก container ให้แก้ `.env` ใช้ `localhost` แทน (ดู [02](02-local-development.md)) หรือรันผ่าน `docker compose up` ทั้งชุดแทน

**`pq: SSL is not enabled on the server`**
DSN ไม่ได้ใส่ `?sslmode=disable` ตอนต่อ Postgres ในเครื่อง/compose/k8s (ที่ไม่ได้เปิด TLS)
ตัวอย่าง: `postgresql://app:app_password@db:5432/tododb?sslmode=disable`
ส่วน Neon/cloud DB ที่บังคับ TLS ให้ใช้ `?sslmode=require` แทน — เลือกผิดฝั่งแล้วต่อไม่ติดทั้งคู่

**`Dirty database version N. Fix and force version.`** ⭐ เทียบเท่า Prisma's P3009 (migration ค้างสถานะ failed)

`golang-migrate` รัน migration แล้วพังกลางทาง (เช่น ไฟล์ `.sql` ผิด syntax) มันเลย mark เวอร์ชันนั้นว่า **dirty**
ไม่กล้ารันต่อจนกว่าเราจะยืนยันสถานะจริงของ DB เอง

```bash
migrate -path migrations -database "$DATABASE_URL" version   # ดูว่าค้างที่เวอร์ชันไหน
```

ตรวจด้วยตาว่า DB ตอนนี้ตรงกับ migration เวอร์ชันไหนจริง ๆ (เปิด `.sql` ไฟล์นั้นเทียบกับตารางจริงใน DB) แล้ว:

```bash
# ถ้า schema ตรงกับเวอร์ชันนั้นแล้วจริง ๆ (แค่ mark ว่าไม่ dirty ไม่รัน SQL ซ้ำ)
migrate -path migrations -database "$DATABASE_URL" force N

# แล้วค่อยรันต่อตามปกติ
migrate -path migrations -database "$DATABASE_URL" up
```

⚠️ `force` ไม่รัน SQL ใด ๆ แค่เปลี่ยน metadata — ถ้า schema จริงไม่ตรงกับเวอร์ชันที่ force จะพังต่อใน migration ถัดไป

**อยากล้างแล้วเริ่มใหม่ทั้งหมด (โปรเจกต์ฝึกส่วนใหญ่ใช้ทางนี้):**

```bash
migrate -path migrations -database "$DATABASE_URL" drop -f   # ลบทุกตาราง + version tracking
migrate -path migrations -database "$DATABASE_URL" up        # apply migration ใหม่ตั้งแต่ต้น
```

⚠️ **ลบข้อมูลทั้ง database** — ห้ามรันกับ production เด็ดขาด

**DB มีตารางอยู่แล้วจากทางอื่น (ไม่เคยผ่าน golang-migrate มาก่อน) — เทียบเท่า Prisma's P3005**

`golang-migrate` เก็บสถานะไว้ในตาราง `schema_migrations` — ถ้า DB มีตาราง `todos` อยู่แล้วแต่ไม่มีตารางนี้ มันจะพยายาม `CREATE TABLE` ซ้ำแล้ว error ว่ามีอยู่แล้ว
ถ้ามั่นใจว่า schema ปัจจุบันตรงกับ `migrations/0001_init.up.sql` เป๊ะ ให้ force เวอร์ชันนั้นแทนที่จะรัน `up`:

```bash
migrate -path migrations -database "$DATABASE_URL" force 1
```

---

**migrate ค้างนาน / advisory lock timeout บน Neon**

สังเกต host ใน error ว่ามี `-pooler` ไหม เช่น `ep-xxx-pooler.ap-southeast-1.aws.neon.tech`

connection แบบ pooled ผ่าน PgBouncer ซึ่ง**ไม่รองรับ advisory lock และ DDL บางอย่าง**ที่ `golang-migrate` ใช้ล็อกกันรัน migration ซ้ำซ้อน
Neon ให้ connection string มา 2 แบบ — ให้ใช้ตัวที่**ไม่มี `-pooler`** ตอนรัน migration:

```bash
# migration ใช้ direct (ไม่มี -pooler)
migrate -path migrations -database "postgresql://...@ep-xxx.ap-southeast-1.aws.neon.tech/neondb?sslmode=require" up

# ส่วนแอปตอนรันจริงใช้ pooled ได้ (รองรับ connection เยอะกว่า) — คนละตัวแปรกับตอน migrate
```

**สรุปสั้น ๆ: migration → direct / แอป → pooled**

**ติดตั้งบนเครื่องแล้ว `migrate: command not found`**
`go install` ไม่ได้เติม `$GOPATH/bin` เข้า `$PATH` → เช็คด้วย `go env GOPATH` แล้วเติม `export PATH=$PATH:$(go env GOPATH)/bin`

**`database driver: unknown driver postgres(ql) (forgotten import?)`**

ตัว `migrate` CLI เป็น binary ที่ต้อง **compile พร้อม build tag ระบุ driver** ตั้งแต่ตอนติดตั้ง ถ้าใช้คำสั่งติดตั้งแบบเปล่า ๆ (ไม่ใส่ `-tags`) จะได้ binary ที่ไม่มี database driver ติดมาเลย — เช็คได้จาก `migrate -help` แล้วดูบรรทัดท้ายสุด:

```bash
migrate -help
# Source drivers: file
# (ถ้าไม่มีบรรทัด "Database drivers: ..." ต่อท้าย = ไม่มี driver ติดมาเลย)
```

วิธีแก้ — ติดตั้งใหม่โดยใส่ `-tags 'postgres'`:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

ติดตั้งเสร็จแล้วเช็คอีกครั้งต้องเห็น `Database drivers: stub, postgres, postgresql` — ตอนนี้ใช้ได้ทั้ง `postgres://...` และ `postgresql://...` ใน `-database`

**cross-compile แล้วรันไม่ได้ / `exec format error`**
build บนเครื่องหนึ่ง (เช่น Mac arm64) แล้วเอาไปรันบน server อีก arch (เช่น amd64) โดยไม่ตั้ง `GOOS`/`GOARCH`
แก้ด้วย `GOOS=linux GOARCH=amd64 go build ./cmd/api` ให้ตรงกับเครื่องปลายทาง (อ่านเพิ่มที่ [03 ภาคลึก](03-docker.md))

## Docker

**build ช้าทุกครั้ง ไม่ยอม cache**
เรียงคำสั่งผิด — `COPY go.mod go.sum` ต้องมาก่อน `COPY cmd`/`COPY internal` ดู [03](03-docker.md)
เช็คว่ามี `.dockerignore` กัน `.git`, `docs`, `exercises` ไว้แล้ว (ลดขนาด build context)

**`port is already allocated`**

```bash
lsof -i :8080          # หาว่าใครใช้อยู่
docker compose down
```

**container ขึ้นแล้วดับทันที**

```bash
docker compose logs api          # อ่าน error
docker compose ps -a             # ดู exit code
```

exit 1 = แอปโยน error เอง, exit 137 = โดนฆ่าเพราะ memory เต็ม

**แก้ไฟล์แล้วไม่มีอะไรเปลี่ยน**
image ยังเป็นตัวเก่า → `docker compose up -d --build`
ถ้าอยากล้างจริง ๆ: `docker compose build --no-cache`

**`permission denied` ในไฟล์ที่ mount**
เกิดจาก non-root user (uid 1000) ไม่มีสิทธิ์ในไฟล์ของ host — dev override ใช้ stage `builder` ที่ยังเป็น root อยู่จึงไม่เจอ

## Compose

**api ขึ้นก่อน db พร้อม**
ต้องมี `condition: service_healthy` ไม่ใช่แค่ `depends_on: [db]`

**ข้อมูล DB หายหลัง restart**
เผลอ `docker compose down -v` (`-v` ลบ volume) หรือลืมประกาศ volume

**เปลี่ยนค่าใน `.env` แล้วไม่มีผล**
ต้อง recreate container: `docker compose up -d --force-recreate`
เช็คค่าที่ถูกแทนจริงด้วย `docker compose config`

## Nginx

**502 Bad Gateway**
nginx ต่อ upstream ไม่ได้ — เช็คว่า api ขึ้นจริงไหม (`docker compose ps`) และชื่อใน `upstream` ตรงกับชื่อ service

**504 Gateway Timeout**
แอปตอบช้าเกิน `proxy_read_timeout` → เพิ่มเวลา หรือไปแก้ที่แอปให้เร็วขึ้น

**เจอ 429 ตลอด**
rate limit เข้มเกิน → เพิ่ม `rate` หรือ `burst` ใน `nginx.conf`

**แก้ config แล้วไม่มีผล**

```bash
docker compose exec nginx nginx -t     # ตรวจ syntax ก่อนเสมอ
docker compose restart nginx
```

mount เป็น `:ro` ต้อง restart container ไม่ใช่แค่เซฟไฟล์

**แอปเห็น IP เป็น 172.x ทุก request**
ขาด `proxy_set_header X-Forwarded-For` หรือฝั่ง Go ไม่ได้ตั้ง `r.SetTrustedProxies(...)` (ดู [05](05-nginx.md))

## GitHub Actions

**`missing go.sum entry` ใน CI แต่ในเครื่องปกติ**
ลืม commit `go.sum` หลังเพิ่ม/เปลี่ยน dependency ใหม่ — รัน `go mod tidy` แล้ว commit ทั้ง `go.mod` และ `go.sum`

**`denied: permission_denied` ตอน push GHCR**

- ยังไม่ได้ใส่ `permissions: packages: write`
- Settings → Actions → General → Workflow permissions ยังเป็น read-only
- ชื่อ image ต้อง **ตัวพิมพ์เล็กทั้งหมด**

**workflow ไม่รันเลย**

- ไฟล์ต้องอยู่ที่ `.github/workflows/*.yml` เป๊ะ ๆ
- branch ใน `on:` ตรงกับ branch จริงไหม (`main` vs `master`)
- YAML ผิด indent → GitHub จะขึ้น error ที่แท็บ Actions

**cache ไม่ทำงาน**
`actions/setup-go@v5` cache module ให้อัตโนมัติตาม `go.sum` — เช็คว่ามีไฟล์ `go.sum` อยู่จริงและ commit แล้ว

**secret เป็นค่าว่าง**
secret ไม่ถูกส่งให้ workflow ที่มาจาก fork PR — เป็นพฤติกรรมด้านความปลอดภัยที่ตั้งใจ

## Kubernetes

> เริ่มจาก `kubectl -n todo-app describe pod <pod>` แล้วอ่าน **Events** ท้ายสุดเสมอ

| อาการ                         | สาเหตุที่พบบ่อย                                | ตรวจยังไง                         |
| ----------------------------- | ---------------------------------------------- | --------------------------------- |
| `ImagePullBackOff`            | ชื่อ image ผิด / private ไม่มี imagePullSecret | `describe pod` → Events           |
| `CrashLoopBackOff`            | แอปตายซ้ำ ๆ ตอน boot                           | `logs <pod> --previous`           |
| `Pending`                     | ทรัพยากรไม่พอ / ไม่มี PV ให้ผูก                | `describe pod` → Events           |
| `OOMKilled`                   | ใช้ memory เกิน `limits`                       | `describe pod` → Last State       |
| `0/1 Running` ไม่ ready สักที | readinessProbe ไม่ผ่าน                         | `logs` + ลอง curl endpoint ใน pod |
| `Init:0/1` ค้าง               | initContainer (migration) พัง                  | `logs <pod> -c migrate`           |

**ดึง image จาก GHCR ที่เป็น private ไม่ได้**

```bash
kubectl -n todo-app create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io --docker-username=<user> --docker-password=<PAT>
```

แล้วเพิ่ม `imagePullSecrets: [{ name: ghcr-secret }]` ใน pod spec
(หรือง่ายกว่า: ตั้ง package ให้เป็น public ที่หน้า Packages)

**ingress ยิงไม่ได้ 404**

- ติดตั้ง controller แล้วหรือยัง: `kubectl get pods -n ingress-nginx`
- `ingressClassName: nginx` ตรงกับ controller ไหม
- ใส่ `/etc/hosts` ชี้ `todo.local` ไป IP ของ ingress แล้วหรือยัง

**HPA ขึ้น `<unknown>` ในคอลัมน์ TARGETS**
ไม่มี metrics-server → `minikube addons enable metrics-server` แล้วรอสักครู่
หรือ Deployment ไม่ได้ตั้ง `resources.requests` (HPA คำนวณจากค่านี้)

**`kubectl apply -k` ฟ้อง field ไม่รู้จัก**
เวอร์ชัน kubectl เก่าเกิน — `kubectl version` แล้วอัปเดต

**PVC ค้างที่ Pending**
คลัสเตอร์ไม่มี default StorageClass → `kubectl get storageclass`

## เทคนิคดีบักทั่วไป

1. **อ่าน error ให้จบ** — บรรทัดสุดท้ายมักบอกสาเหตุจริง
2. **แยกให้แคบลง** — พังที่แอป, ที่ network, หรือที่ config? ลองยิงจากในสุดออกมาทีละชั้น
3. **เทียบกับที่ที่มันเวิร์ก** — รันในเครื่องได้แต่ใน docker ไม่ได้ = ปัญหาอยู่ที่ environment ไม่ใช่โค้ด
4. **เปลี่ยนทีละอย่าง** — เปลี่ยนสามที่พร้อมกันแล้วหาย จะไม่รู้ว่าอะไรแก้
5. **`describe` / `logs` / `exec` คือเพื่อนที่ดีที่สุด**

## 🪛 Playground

ฝึกวินิจฉัยจริง — จงใจทำพังแล้วดูว่าเจออาการตรงตามที่เอกสารบอกไหม:

- [ ] ลบ `DATABASE_URL` ออกจาก `.env` แล้วรัน `docker compose up` — เจอ error ตรงกับที่เขียนไว้ไหม
- [ ] แก้ DSN ให้ผิด host (เช่น `db2` แทน `db`) แล้วดู error message จริง เทียบกับที่เอกสารบอก
- [ ] รัน `migrate ... up` สองครั้งติดกันบน DB เดิม — เกิดอะไรขึ้นครั้งที่สอง
- [ ] ทำให้ migration ไฟล์หนึ่งมี syntax ผิดโดยตั้งใจ แล้วดูสถานะ `dirty` ที่เกิดขึ้นจริง แล้วลองแก้ตามขั้นตอนด้านบน
- [ ] ลองทุกอาการใน "เทคนิคดีบักทั่วไป" ข้อ 2 (แยกให้แคบลง) กับปัญหาใดก็ได้ที่เจอระหว่างทำ workshop นี้
