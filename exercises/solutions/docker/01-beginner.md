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
  -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?schema=public" \
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

layer ที่ใหญ่ที่สุดคือ `COPY --from=deps /app/node_modules ./node_modules` เพราะ `node_modules` ใหญ่กว่าโค้ดเราหลายเท่า

**ทำไมถึงเป็นแบบนี้:** โค้ดที่เราเขียนเองมีขนาดหลัก KB แต่ dependency ทั้งหมดรวมกันเป็นหลัก MB
นี่คือเหตุผลที่การตัด devDependencies ออก (`npm ci --omit=dev`) ได้ผลมากกว่าการไปบีบโค้ดตัวเอง

---

## D1.4 สำรวจข้างใน

```bash
docker exec -it api-manual sh
/app $ whoami          # node
/app $ id              # uid=1000(node) gid=1000(node)
/app $ ls -la /app     # dist, node_modules, package.json, prisma
/app $ ls /app/dist    # app.js, index.js, routes/…
```

**ทำไมไม่ใช่ root:** Dockerfile มี `USER node` — ถ้าแอปโดนเจาะ ผู้โจมตีได้สิทธิ์แค่ user ธรรมดา ไม่ใช่ root
และเป็นเงื่อนไขที่ทำให้ `runAsNonRoot: true` ใน k8s ทำงานได้

**ทำไมไม่มีไฟล์ `.ts`:** TypeScript ถูกคอมไพล์เป็น JavaScript ตั้งแต่ stage `builder` แล้ว stage สุดท้ายก็อปมาเฉพาะ `dist/`
source code ไม่ติดไปด้วย — ทั้งเล็กลงและไม่เปิดเผยโค้ดต้นฉบับโดยไม่จำเป็น

---

## D1.5 log และ exit code

```bash
docker run --name broken devops-todo-api:local
docker logs broken
# Error: DATABASE_URL is not set
#     at Object.<anonymous> (/app/dist/env.js:...)

docker ps -a --filter name=broken --format '{{.Status}}'
# Exited (1) ...
```

**ทำไมถึงตายทันที:** `src/env.ts` โยน error ทันทีที่ import ถ้าไม่มี `DATABASE_URL`

```ts
if (!env.databaseUrl) {
  throw new Error("DATABASE_URL is not set");
}
```

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
builder ──▶ /app/dist                    ─┐
        └─▶ /app/node_modules/.prisma    ─┤
                                          ├──▶ runner (image สุดท้าย)
deps    ──▶ /app/node_modules (prod only)─┘
```

**ทำไมต้อง 3 stage:** เพราะเราต้องการของจาก 2 แหล่งที่มีเงื่อนไขต่างกัน

- `builder` ต้องมี devDependencies (typescript, prisma CLI) เพื่อคอมไพล์ → แต่ไม่ควรติดไปใน image สุดท้าย
- `deps` ติดตั้งเฉพาะ production dependencies
- `runner` หยิบเฉพาะผลลัพธ์ที่ต้องใช้จริง

ผลคือ image สุดท้าย **ไม่มี** source code, typescript, prisma CLI, หรือ npm cache เลย

---

## 🎯 ต่อยอด

- `docker run --rm` ต่างจากไม่ใส่ยังไง (ลองดูใน `docker ps -a`)
- ลอง `docker inspect devops-todo-api:local` แล้วหา `Env`, `Cmd`, `User`, `Healthcheck`
- ลบ image แล้ว build ใหม่ จับเวลาเทียบกับตอนมี cache
