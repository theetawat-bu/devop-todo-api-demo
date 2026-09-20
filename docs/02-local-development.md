# 02 — รันในเครื่อง (Local Development)

## สิ่งที่ต้องมี

- Go 1.25+
- Docker Desktop (ใช้รัน Postgres)
- [`golang-migrate` CLI](https://github.com/golang-migrate/migrate) — `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1`

## ขั้นตอน

```bash
# 1) เตรียม env
cp .env.example .env

# 2) โหลด dependencies
go mod download

# 3) ปั้น Postgres ขึ้นมาตัวเดียว (ยังไม่ต้องรัน api ใน docker)
docker compose up -d db

# 4) สร้างตารางจาก migration
migrate -path migrations -database "postgresql://app:app_password@localhost:5432/tododb?sslmode=disable" up

# 5) รัน
go run ./cmd/api
```

เปิด http://localhost:3000/healthz ควรได้ `{"status":"ok",...}`

## ลองยิง API

```bash
curl -X POST http://localhost:3000/api/todos \
  -H 'Content-Type: application/json' \
  -d '{"title":"เรียน Docker"}'

curl http://localhost:3000/api/todos
curl -X PATCH http://localhost:3000/api/todos/1 -H 'Content-Type: application/json' -d '{"done":true}'
curl -X DELETE http://localhost:3000/api/todos/1 -i
```

## คำสั่งที่ใช้บ่อย

| คำสั่ง                                         | ทำอะไร                                                                                                                                                  |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go run ./cmd/api`                             | รันจาก source ตรง ๆ (คอมไพล์ทุกครั้งที่รัน)                                                                                                             |
| `go vet ./...`                                 | ตรวจโค้ดหา bug ที่ compiler จับไม่ได้ — CI ใช้ตัวนี้                                                                                                    |
| `go build -o api ./cmd/api`                    | คอมไพล์เป็น binary                                                                                                                                      |
| `./api`                                        | รัน binary ที่ build แล้ว (โหมด production)                                                                                                             |
| `docker compose exec db psql -U app -d tododb` | เปิด psql ดู/แก้ข้อมูลใน DB (ไม่มี GUI แบบ Prisma Studio ในตัว — ใช้ [TablePlus](https://tableplus.com/)/[DBeaver](https://dbeaver.io/) ถ้าอยากได้ GUI) |

### `go mod download` vs `go mod tidy`

สองคำสั่งนี้มักสับสนกันเพราะทั้งคู่เกี่ยวกับ dependency แต่ทำหน้าที่คนละอย่าง:

| คำสั่ง | ทำอะไร | แก้ `go.mod`/`go.sum` ไหม |
| --- | --- | --- |
| `go mod download` | ดาวน์โหลด module ที่ **มีอยู่แล้ว** ใน `go.mod`/`go.sum` ลง local cache (`$GOPATH/pkg/mod`) | ไม่แก้ |
| `go mod tidy` | สแกน import ในโค้ดจริงทั้งโปรเจกต์ แล้วเพิ่ม dependency ที่ขาด/ลบตัวที่ไม่ใช้แล้วออกจาก `go.mod`, อัปเดต `go.sum` ให้ครบ | แก้ |

ใช้ `go mod download` ตอนแค่อยากเตรียม dependency ไว้ก่อนรัน/build (ตามขั้นตอนด้านบน) หรือ warm-up cache ใน Docker layer — เดายังไงก็ไม่แก้ไฟล์ `go.mod`

ใช้ `go mod tidy` ทุกครั้งที่เพิ่ม/ลบ import ในโค้ด เช่นหลัง `go get <package>` เพื่อให้ `go.mod`/`go.sum` sync กับโค้ดจริง — ถ้าลืมรัน CI อาจฟ้อง `missing go.sum entry` (ดู [12](12-troubleshooting.md))

## golang-migrate ที่ต้องเข้าใจ 3 คำสั่ง

```bash
migrate create -ext sql -dir migrations -seq add_priority   # สร้างไฟล์ .up.sql/.down.sql คู่ใหม่ให้แก้เอง
migrate -path migrations -database "$DATABASE_URL" up       # รัน migration ที่ยังไม่ได้ apply ทั้งหมด
migrate -path migrations -database "$DATABASE_URL" down 1   # ย้อนกลับ 1 migration ล่าสุด
```

ต่างจาก Prisma ตรงที่ **ไม่มีการ generate code จาก schema** — เราเขียน SQL เองตรง ๆ ทั้ง up และ down
ข้อดีคือควบคุมได้เต็มที่ ข้อเสียคือต้องเขียน SQL เองทุกครั้ง ไม่มี type ให้ช่วยตรวจ

**กฎเหล็ก:** ทั้ง dev และ prod/CI ใช้คำสั่ง `up` เดียวกัน (ไม่มีคำสั่งแยกแบบ `migrate dev` ของ Prisma)
สิ่งที่ป้องกัน accident คือไฟล์ `.sql` ที่เขียนเอง — ต้องระวังเองว่า `up` ทำอะไรกับข้อมูลจริงบ้าง

### แก้ schema แล้วทำยังไงต่อ

```bash
migrate create -ext sql -dir migrations -seq add_priority
# → ได้ migrations/000002_add_priority.up.sql และ .down.sql เปล่า ๆ ให้เขียน SQL เอง
# เช่น up.sql: ALTER TABLE todos ADD COLUMN priority INT NOT NULL DEFAULT 0;
#     down.sql: ALTER TABLE todos DROP COLUMN priority;
# แล้ว commit ทั้งคู่ลง git
```

## เรื่อง environment variable

`internal/config/config.go` เรียก [`godotenv.Load()`](https://github.com/joho/godotenv) ก่อนอ่านค่าใด ๆ — โหลด `.env` ในโฟลเดอร์ปัจจุบันเข้ามาเป็น environment variable ให้อัตโนมัติ **ไม่ต้อง `export` เอง**
ถ้าไม่เจอไฟล์ `.env` เลย (เช่นใน container ที่ prod ไม่ได้ copy ไฟล์นี้เข้าไป) จะไม่ error เพราะ prod ตั้ง env vars จริงผ่าน compose/k8s แทนอยู่แล้ว

หลังจากนั้นจะอ่านค่าแล้ว **โยน error ทันทีถ้าไม่มี `DATABASE_URL`**
นี่เป็น pattern ที่ดีเรียกว่า _fail fast_ — แอปตายตั้งแต่ boot ดีกว่าไปตายตอนมี request จริง
(บน k8s pod จะเข้า `CrashLoopBackOff` ให้เห็นเลยว่าตั้งค่าผิด)

`DATABASE_URL` ต่างกันตามที่รัน:

| รันที่ไหน                          | host ใน DATABASE_URL      |
| ---------------------------------- | ------------------------- |
| เครื่องตัวเอง (`go run ./cmd/api`) | `localhost`               |
| ใน docker compose                  | `db` (ชื่อ service)       |
| ใน Kubernetes                      | `postgres` (ชื่อ Service) |

จำหลักไว้: **ใน container network ให้ใช้ชื่อ service เป็น hostname เสมอ ไม่ใช่ localhost**
(`localhost` ใน container หมายถึงตัว container นั้นเอง)

⚠️ **`.env.example` (และ `.env` ที่ copy มา) ตั้ง host เป็น `db`** เพราะออกแบบไว้ให้รันผ่าน `docker compose up` เป็นหลัก (ดู [04](04-docker-compose.md))
ถ้าจะรัน `go run ./cmd/api` ตรงจากเครื่องตามขั้นตอนด้านบนจริง ๆ ต้องแก้ `DATABASE_URL` ใน `.env` ให้เป็น `localhost` เอง ไม่งั้นจะเจอ `lookup db: no such host` (ดู [12](12-troubleshooting.md))

อีกจุดที่ต้องระวัง: `sslmode` ต้องตรงกับปลายทาง — DB ในเครื่อง/compose/k8s (ไม่เปิด TLS) ต้องมี `?sslmode=disable`
ส่วน DB บน cloud อย่าง Neon (บังคับ TLS) ต้องมี `?sslmode=require` — ใส่ผิดฝั่งแล้วต่อ DB ไม่ติดทั้งคู่ (ดู [12](12-troubleshooting.md))

## `.env` กับ git

`.env` อยู่ใน `.gitignore` แล้ว — **ห้าม commit เด็ดขาด**
ให้ commit แค่ `.env.example` ที่มีแต่ค่าตัวอย่าง เพื่อนร่วมทีมจะได้รู้ว่าต้องตั้งตัวแปรอะไรบ้าง

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] ลบ `DATABASE_URL` ออกจาก `.env` ชั่วคราวแล้ว `go run ./cmd/api` — error message บอกอะไร
- [ ] เปลี่ยน `PORT` ใน `.env` เป็น `4000` แล้วรันใหม่ ยิง `curl localhost:4000/healthz`
- [ ] รัน `migrate ... up` ซ้ำอีกครั้งบน DB เดิม (ที่ apply ไปแล้ว) — เกิดอะไรขึ้น
- [ ] เพิ่ม todo ผ่าน curl แล้วเปิด `docker compose exec db psql -U app -d tododb -c 'SELECT * FROM todos;'` ดูข้อมูลตรงจาก DB
- [ ] ลองต่อ DB ด้วย `?sslmode=require` ทั้งที่ DB ในเครื่องไม่ได้เปิด TLS ดู error ที่ได้จริง

➡️ ต่อไป: [03 — Docker](03-docker.md)
