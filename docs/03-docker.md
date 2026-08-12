# 03 — Docker

## แนวคิดที่ต้องแยกให้ออก

| คำ | คือ |
|---|---|
| **Dockerfile** | สูตรอาหาร |
| **Image** | อาหารที่ทำเสร็จแล้ว แช่แข็งไว้ (read-only) |
| **Container** | อาหารที่เอาออกมาอุ่นกิน (instance ที่กำลังรัน) |
| **Layer** | แต่ละคำสั่งใน Dockerfile = 1 layer ที่ cache ได้ |
| **Registry** | ตู้แช่กลาง (Docker Hub, GHCR) |

## อ่าน Dockerfile ของเราทีละ stage

ไฟล์นี้เป็น **multi-stage build** = มีหลาย `FROM` แต่ image สุดท้ายเอาเฉพาะ stage ท้ายสุด

### Stage 1: builder — คอมไพล์

```dockerfile
COPY package*.json ./
RUN npm ci
COPY prisma ./prisma
RUN npx prisma generate
COPY tsconfig.json ./
COPY src ./src
RUN npm run build
```

**ทำไม copy `package*.json` ก่อน แล้วค่อย copy `src`?**

นี่คือหัวใจของ layer caching Docker จะ cache layer ไว้และใช้ซ้ำถ้า input ไม่เปลี่ยน
ถ้าเรา `COPY . .` ทีเดียวตั้งแต่แรก → แก้โค้ด 1 บรรทัด = ทุก layer หลังจากนั้นพัง cache = `npm ci` ใหม่ทุกครั้ง (ช้ามาก)
แต่แยก copy แบบนี้ → แก้โค้ดไม่กระทบ `package.json` → `npm ci` ใช้ cache เดิม → build เร็วขึ้นหลายเท่า

**หลักจำง่าย: อะไรที่เปลี่ยนน้อย ให้ไว้บน อะไรที่เปลี่ยนบ่อย ให้ไว้ล่าง**

`npm ci` ≠ `npm install`:
- `npm ci` ติดตั้งตาม `package-lock.json` เป๊ะ ๆ, ลบ `node_modules` เดิมทิ้ง, ไม่แก้ lockfile → **ใช้ใน CI/Docker เสมอ**
- `npm install` แก้ lockfile ได้ → ใช้ตอน dev

### Stage 2: deps — เอาเฉพาะ dependency ของ production

```dockerfile
RUN npm ci --omit=dev
```

typescript, tsx, @types/* ไม่จำเป็นตอนรันจริง ตัดออกให้ image เล็กลง

### Stage 3: runner — image ที่ deploy จริง

```dockerfile
COPY --from=deps    /app/node_modules ./node_modules
COPY --from=builder /app/node_modules/.prisma ./node_modules/.prisma
COPY --from=builder /app/dist ./dist
```

หยิบเฉพาะของที่ต้องใช้: prod deps + Prisma client ที่ generate แล้ว + โค้ดที่คอมไพล์แล้ว
**source code, devDependencies, ไฟล์ระหว่างทาง ไม่ติดไปด้วยเลย** — ทั้งเล็กลงและปลอดภัยขึ้น

### ทำไมต้อง `USER node`

```dockerfile
RUN chown -R node:node /app
USER node
```

default ของ container คือรันเป็น root ถ้าคนแฮกเข้ามาได้ ก็ได้ root ไปเลย
`USER node` ลดความเสียหายลงมาก และเป็นเงื่อนไขบังคับของ `runAsNonRoot: true` ใน k8s manifest ของเรา

### HEALTHCHECK

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -fsS http://127.0.0.1:3000/healthz || exit 1
```

Docker จะยิงเช็คเองแล้วรายงานสถานะ `healthy`/`unhealthy` — compose ใช้ค่านี้ใน `depends_on: condition: service_healthy`
`start-period` = ช่วงผ่อนผันตอน boot ยังไม่นับว่าล้มเหลว

### `.dockerignore` สำคัญกว่าที่คิด

ไฟล์นี้กัน `node_modules`, `.git`, `.env` ไม่ให้ถูกส่งเข้า build context
- ถ้าไม่กัน `node_modules` → ส่งไฟล์เป็นแสนเข้า daemon ทุกครั้ง (ช้า) แถมทับ `node_modules` ที่ติดตั้งใน image (ผิด platform)
- ถ้าไม่กัน `.env` → **ความลับหลุดเข้าไปอยู่ใน image**

## คำสั่งที่ต้องคล่อง

```bash
docker build -t devops-todo-api:local .
docker build -t devops-todo-api:local --build-arg APP_VERSION=1.0.0 .

docker images                       # ดูขนาด image
docker run --rm -p 3000:3000 -e DATABASE_URL=... devops-todo-api:local
docker ps -a
docker logs -f <container>
docker exec -it <container> sh      # เข้าไปดูข้างใน
docker history devops-todo-api:local  # ดูว่า layer ไหนกินที่เท่าไร
docker system prune -af             # เก็บกวาดของที่ไม่ใช้ (ระวัง! ลบเยอะ)
```

## ลองพิสูจน์เรื่อง cache ด้วยตัวเอง

```bash
docker build -t t1 .                       # ครั้งแรก ช้า
docker build -t t1 .                       # ครั้งสอง เร็วมาก (CACHED ทุก layer)
echo "// x" >> src/index.ts
docker build -t t1 .                       # เร็วอยู่ — เพราะ npm ci ยัง CACHED
```

สังเกตคำว่า `CACHED` ในผลลัพธ์ นั่นแหละคือเหตุผลที่ต้องเรียงคำสั่งให้ถูก

➡️ ต่อไป: [04 — Docker Compose](04-docker-compose.md)
