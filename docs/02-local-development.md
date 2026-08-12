# 02 — รันในเครื่อง (Local Development)

## สิ่งที่ต้องมี

- Node.js 22+
- Docker Desktop (ใช้รัน Postgres)

## ขั้นตอน

```bash
# 1) เตรียม env
cp .env.example .env

# 2) ติดตั้ง dependencies
rm -rf node_modules dist   # ถ้ามีของเก่าค้างอยู่
npm install

# 3) ปั้น Postgres ขึ้นมาตัวเดียว (ยังไม่ต้องรัน api ใน docker)
docker compose up -d db

# 4) สร้างตารางจาก migration
npx prisma migrate deploy

# 5) รันแบบ hot reload
npm run dev
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

| คำสั่ง | ทำอะไร |
|---|---|
| `npm run dev` | รันแบบ watch (tsx) |
| `npm run typecheck` | ตรวจ type อย่างเดียว ไม่ build — CI ใช้ตัวนี้ |
| `npm run build` | คอมไพล์ TS → `dist/` |
| `npm start` | รันไฟล์ที่ build แล้ว (โหมด production) |
| `npx prisma studio` | เปิด GUI ดู/แก้ข้อมูลใน DB |

## Prisma ที่ต้องเข้าใจ 3 คำสั่ง

```bash
npx prisma generate           # สร้าง TypeScript client จาก schema — ต้องรันทุกครั้งที่แก้ schema
npx prisma migrate dev        # (dev เท่านั้น) เทียบ schema กับ DB แล้วสร้างไฟล์ migration ใหม่
npx prisma migrate deploy     # (prod/CI) รัน migration ที่มีอยู่แล้วเท่านั้น ไม่สร้างใหม่ ไม่ถามอะไร
```

**กฎเหล็ก:** `migrate dev` ใช้แค่ตอน develop เพราะมันอาจ reset DB ได้
ใน CI/CD และ production ใช้ `migrate deploy` เสมอ (ในโปรเจกต์นี้ Dockerfile และ initContainer ของ k8s ใช้ตัวนี้)

### แก้ schema แล้วทำยังไงต่อ

```bash
# แก้ prisma/schema.prisma เช่นเพิ่ม field priority
npx prisma migrate dev --name add_priority
# → จะได้โฟลเดอร์ใหม่ใน prisma/migrations/ ให้ commit ลง git ด้วย
```

## เรื่อง environment variable

`src/env.ts` อ่านค่าแล้ว **โยน error ทันทีถ้าไม่มี `DATABASE_URL`**
นี่เป็น pattern ที่ดีเรียกว่า *fail fast* — แอปตายตั้งแต่ boot ดีกว่าไปตายตอนมี request จริง
(บน k8s pod จะเข้า `CrashLoopBackOff` ให้เห็นเลยว่าตั้งค่าผิด)

`DATABASE_URL` ต่างกันตามที่รัน:

| รันที่ไหน | host ใน DATABASE_URL |
|---|---|
| เครื่องตัวเอง (`npm run dev`) | `localhost` |
| ใน docker compose | `db` (ชื่อ service) |
| ใน Kubernetes | `postgres` (ชื่อ Service) |

จำหลักไว้: **ใน container network ให้ใช้ชื่อ service เป็น hostname เสมอ ไม่ใช่ localhost**
(`localhost` ใน container หมายถึงตัว container นั้นเอง)

## `.env` กับ git

`.env` อยู่ใน `.gitignore` แล้ว — **ห้าม commit เด็ดขาด**
ให้ commit แค่ `.env.example` ที่มีแต่ค่าตัวอย่าง เพื่อนร่วมทีมจะได้รู้ว่าต้องตั้งตัวแปรอะไรบ้าง

➡️ ต่อไป: [03 — Docker](03-docker.md)
