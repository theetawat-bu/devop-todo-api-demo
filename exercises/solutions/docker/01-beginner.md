# เฉลย — 🐳 Docker ระดับ 1

⬅️ [กลับไปที่โจทย์](../../docker/01-beginner.md)

---

## D1.1 image vs container

```bash
docker build -t devops-todo-api:local .
docker run -d --name a devops-todo-api:local
docker run -d --name b devops-todo-api:local
docker images       # 1 รายการ
docker ps -a        # 2 รายการ
```

**ทำไมถึงเป็นแบบนี้:** image คือแม่พิมพ์ที่อ่านได้อย่างเดียว container คือ instance ที่รันจากแม่พิมพ์นั้น + เพิ่ม writable layer บาง ๆ ทับข้างบน
ที่ container 2 ตัวไม่กินที่เป็นสองเท่า เพราะทั้งคู่ใช้ layer เดียวกันร่วมกัน มีเฉพาะ writable layer ที่แยกกัน

---

## D1.2 รันให้ติด

```bash
docker compose up -d db
docker network ls | grep appnet     # ได้ชื่อประมาณ devops-todo-api_appnet

docker run -d --name api-manual \
  --network devops-todo-api_appnet -p 3000:3000 \
  -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" \
  devops-todo-api:local

curl localhost:3000/healthz
```

**ทำไม host ต้องเป็น `db`:** ใน container คำว่า `localhost` หมายถึง**ตัว container นั้นเอง** ไม่ใช่เครื่องเรา
Docker มี DNS ในตัวที่แปลงชื่อ service เป็น IP ของ container นั้น — ตราบใดที่อยู่ network เดียวกัน

**ข้อผิดพลาดที่พบบ่อย:** ลืม `--network` → container อยู่คนละ network → `getaddrinfo ENOTFOUND db`

---

## D1.3 อ่าน image

```bash
docker images devops-todo-api
docker history devops-todo-api:local
```

layer ที่ใหญ่ที่สุดมักเป็น `FROM alpine:3.20 AS runner` (base image) หรือ `COPY --from=builder /out/api ./api` เพราะ Go binary เป็น static binary ที่รวม runtime และทุก dependency เข้าไปในไฟล์เดียวแล้ว

**ทำไมถึงเป็นแบบนี้:** ไม่มี layer ของ dependency แยกต่างหากเหมือนฝั่ง Node เพราะ `go build` คอมไพล์ dependency ทั้งหมดเข้าไปใน binary ตั้งแต่ตอน build — ไม่มี `node_modules` ให้ copy ข้าม stage
นี่คือเหตุผลที่ image ของ Go เล็กกว่า Node แบบเทียบกันไม่ติดโดยไม่ต้องพยายามอะไรเป็นพิเศษ (ไม่ต้อง `--omit=dev`, ไม่ต้อง prune cache)

---

## D1.4 สำรวจข้างใน

```bash
docker exec -it api-manual sh
/app $ whoami          # app
/app $ id              # uid=1000(app) gid=1000(app)
/app $ ls -la /app     # api, migrate, migrations/
/app $ which go        # (ไม่มีอะไรเลย — ไม่เจอ)
```

**ทำไมไม่ใช่ root:** Dockerfile สร้าง user เองด้วย `addgroup -S app && adduser -S -G app -u 1000 app` แล้วปิดท้ายด้วย `USER app` — ถ้าแอปโดนเจาะ ผู้โจมตีได้สิทธิ์แค่ user ธรรมดา ไม่ใช่ root
(alpine ไม่มี built-in non-root user อย่าง `node` image ของ Node จึงต้องสร้างเอง) และเป็นเงื่อนไขที่ทำให้ `runAsNonRoot: true` ใน k8s ทำงานได้

**ทำไมไม่มีไฟล์ `.go` หรือ Go toolchain เลย:** `go build` คอมไพล์ source ทั้งหมดเป็น static binary ตั้งแต่ stage `builder` แล้ว stage สุดท้าย (`runner`) copy มาแค่ `api` (compiled binary), `migrate` (compiled binary), และโฟลเดอร์ `migrations/` (SQL เฉย ๆ ไม่ใช่โค้ด)
ไม่มี source code, ไม่มี `go.mod`/`go.sum`, ไม่มี module cache ติดไปด้วยเลย — เทียบกับ Node ที่อย่างน้อยยังต้องพก `node_modules` (runtime dependency) ไปด้วยเสมอ Go ไม่ต้องพกอะไรเลยนอกจาก binary เปล่า ๆ ตัวเดียว

---

## D1.5 log และ exit code

```bash
docker run --name broken devops-todo-api:local
docker logs broken
# migrate: DATABASE_URL is not set
# (หรือ panic จาก internal/config ตอน parse env ว่าง)

docker ps -a --filter name=broken --format '{{.Status}}'
# Exited (1) ...
```

**ทำไมถึงตายทันที:** CMD ของ container คือ `migrate -path ./migrations -database "$DATABASE_URL" up && ./api` — ถ้า `$DATABASE_URL` เป็นค่าว่าง คำสั่ง `migrate` จะ parse connection string ไม่ผ่านและ exit ด้วย non-zero code ทันที ก่อนที่ `./api` จะได้เริ่มทำงานด้วยซ้ำ
ถ้า migrate ผ่านไปได้ (เช่นทดสอบ path ที่ไม่ผ่าน migrate) `internal/config` ก็ยังอ่าน env ตอน startup แล้ว exit ทันทีถ้าค่าที่จำเป็นหายไป — ไม่ปล่อยให้ตัว server เริ่ม listen ทั้งที่ config ไม่ครบ

นี่คือแพตเทิร์น **fail fast** — ตายตั้งแต่วินาทีแรกดีกว่ารันไปได้ 3 ชั่วโมงแล้วค่อยพังตอนมีผู้ใช้จริง
บน k8s pod จะเข้า `CrashLoopBackOff` ให้เห็นทันทีว่าตั้งค่าผิด แทนที่จะขึ้นเขียวหลอก ๆ แล้วพัง request แรก

---

## D1.6 ทำความสะอาด

```bash
docker stop a b api-manual broken && docker rm a b api-manual broken
docker system df
```

| ประเภท | คือ |
| --- | --- |
| **Images** | layer ทั้งหมด (RECLAIMABLE = ที่ไม่มี container ใช้อยู่) |
| **Containers** | writable layer ของแต่ละ container |
| **Build Cache** | cache ของ BuildKit — โตเร็วมากถ้า build บ่อย มักเป็นตัวกินที่อันดับหนึ่ง |

```bash
docker container prune       # ลบ container ที่หยุดแล้ว
docker builder prune         # ลบ build cache
docker system prune -a       # ลบทุกอย่างที่ไม่ได้ใช้ (ระวัง!)
```

---

## D1.7 อ่าน Dockerfile

```
builder ──▶ /out/api           ─┐
        └─▶ /go/bin/migrate    ─┼──▶ runner (image สุดท้าย)
        (+ migrations/ จาก build context, copy ตรงไม่ผ่าน builder) ─┘
```

**ทำไมมีแค่ 2 stage (ไม่ใช่ 3 แบบที่โปรเจกต์ Node มักมี):** Go ไม่มีแนวคิด "production dependencies" แยกจาก "dev dependencies" เหมือน npm — `go build` คอมไพล์ทุกอย่างที่โค้ดใช้จริงเข้า binary เดียว ไม่มี dependency เหลือค้างให้ต้อง prune ทีหลัง

- `builder` มี Go toolchain ครบเพื่อคอมไพล์ `api` และติดตั้ง `migrate` CLI → ตัว toolchain เองไม่ควรติดไปใน image สุดท้าย (มันใหญ่และไม่จำเป็นตอน runtime)
- `runner` หยิบเฉพาะ binary ที่ compile เสร็จแล้ว 2 ตัว + โฟลเดอร์ SQL migration

ผลคือ image สุดท้าย **ไม่มี** source code, Go compiler, module cache (`/go/pkg/mod`), หรือ build tool ใด ๆ เลย — เป็นเวอร์ชันที่เข้มกว่าฝั่ง Node เสียอีก เพราะ Node ยังต้องพก `node_modules` (runtime dependency) ติดไปด้วยตลอด แต่ Go ไม่ต้องพกอะไรเลยนอกจาก binary

---

## 🎯 ต่อยอด

- `docker run --rm` ต่างจากไม่ใส่ยังไง (ลองดูใน `docker ps -a`)
- ลอง `docker inspect devops-todo-api:local` แล้วหา `Env`, `Cmd`, `User`, `Healthcheck`
- ลบ image แล้ว build ใหม่ จับเวลาเทียบกับตอนมี cache
