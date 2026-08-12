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

---

# ภาคลึก — multi-arch และ ARG/ENV ข้าม stage

ส่วนนี้จำเป็นสำหรับ [10 — deploy ขึ้น cloud ฟรี](10-deploy-free-cloud.md) เพราะ Oracle Cloud free tier เป็น **ARM**
ถ้าไม่เข้าใจส่วนนี้ จะเจอ `exec format error` แล้วหาสาเหตุไม่เจอ

## X1. image หนึ่งตัวรันได้หลายสถาปัตยกรรมได้ยังไง

image ที่รองรับหลาย arch ไม่ใช่ image เดียว แต่เป็น **manifest list** — สารบัญที่ชี้ไปหา image จริงหลายตัว

```
ghcr.io/you/todo:main   (manifest list)
├── linux/amd64  → sha256:aaa…
└── linux/arm64  → sha256:bbb…
```

ตอน `docker pull` client จะบอก arch ของตัวเองไป แล้ว registry ส่งตัวที่ตรงกลับมา — เป็นเหตุผลที่คำสั่งเดียวใช้ได้ทั้ง Mac M-series และ server Intel

```bash
docker manifest inspect ghcr.io/you/todo:main | jq '.manifests[].platform'
# {"architecture":"amd64","os":"linux"}
# {"architecture":"arm64","os":"linux"}
```

**`exec format error` แปลว่าอะไร:** kernel พยายามรันไบนารีที่คอมไพล์มาสำหรับ CPU คนละตระกูล
ไม่ใช่ปัญหาของ Docker หรือของโค้ด แต่คือ image ตัวนั้นไม่มีเวอร์ชัน arm64

---

## X2. build ข้าม arch ทำได้ 2 วิธี

```bash
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/you/todo:main --push .
```

| วิธี | ทำงานยังไง | ความเร็ว |
| --- | --- | --- |
| **QEMU emulation** (ที่เราใช้) | จำลอง CPU อีกตระกูลทีละคำสั่ง | **ช้ากว่า 3–10 เท่า** |
| **Native runner** | build บนเครื่องที่เป็น arch นั้นจริง แล้วรวม manifest | เร็วเท่าปกติ |
| **Cross-compile** | คอมไพเลอร์สร้างไบนารีของอีก arch จากเครื่องเดิม | เร็ว แต่ต้องรองรับในภาษานั้น |

`docker/setup-qemu-action@v3` ใน `build-push.yml` คือตัวที่ลง QEMU ให้ — **ถ้าไม่มี บรรทัด `platforms:` จะพังทันที**

**ทำไม `--push` แทน `--load`:** local image store ของ Docker เก็บได้แค่ arch เดียว
manifest list เป็นแนวคิดของ registry — เก็บในเครื่องไม่ได้ ต้อง push ขึ้นไปเลย

**ตัวแปรอัตโนมัติที่ BuildKit ให้มา:**

```dockerfile
FROM --platform=$BUILDPLATFORM node:22-alpine AS builder
ARG TARGETPLATFORM   # เช่น linux/arm64  — เครื่องปลายทาง
ARG BUILDPLATFORM    # เช่น linux/amd64  — เครื่องที่ build
RUN echo "build บน $BUILDPLATFORM เพื่อไปรันบน $TARGETPLATFORM"
```

เทคนิคขั้นสูง: ให้ stage `builder` รันบน `$BUILDPLATFORM` (เร็ว ไม่ต้องจำลอง) แล้วให้เฉพาะ stage สุดท้ายเป็น arch ปลายทาง
สำหรับ Node ที่ผลลัพธ์เป็น JavaScript ล้วน วิธีนี้ประหยัดเวลาได้มาก

---

## X3. ARG มีขอบเขตแค่ไหน — จุดที่คนพลาดบ่อย

```dockerfile
ARG NODE_VERSION=22            # ← ก่อน FROM = global แต่ใช้ได้เฉพาะในบรรทัด FROM
FROM node:${NODE_VERSION}-alpine AS builder
RUN echo $NODE_VERSION         # ← ว่าง! ต้องประกาศซ้ำในแต่ละ stage

FROM node:${NODE_VERSION}-alpine AS runner
ARG NODE_VERSION               # ← ประกาศซ้ำ (ไม่ต้องใส่ค่า) ถึงจะใช้ได้
RUN echo $NODE_VERSION         # 22
```

**กฎ 3 ข้อ:**

1. `ARG` ก่อน `FROM` ใช้ได้เฉพาะในบรรทัด `FROM` เท่านั้น
2. `ARG` ใน stage หนึ่ง **ไม่ตกทอด**ไปอีก stage — ต้องประกาศซ้ำ
3. `ARG` หายไปตอน runtime — ถ้าอยากให้เหลือ ต้องส่งต่อเป็น `ENV`

```dockerfile
ARG APP_VERSION=dev
ENV APP_VERSION=$APP_VERSION   # ← บรรทัดนี้ทำให้ /healthz อ่านค่าได้ตอนรัน
```

**ตรวจว่า ARG ถูกส่งเข้ามาจริงไหม:**

```bash
docker build --build-arg APP_VERSION=1.2.3 -t t . --progress=plain 2>&1 | grep APP_VERSION
docker inspect t --format '{{json .Config.Env}}' | jq
```

⚠️ `ARG` และ `ENV` **ห้ามใส่ความลับ** — `ARG` ติดอยู่ใน build history และ `ENV` เห็นได้จาก `docker inspect` โดยใครก็ได้ที่ pull image ไป

---

## X4. อ่านผลของ build ให้ออก

```bash
docker build -t t . --progress=plain
```

| สิ่งที่เห็นใน output | แปลว่า |
| --- | --- |
| `CACHED` | ใช้ layer เดิม — เป้าหมายที่ต้องการ |
| `transferring context: 200kB` | ขนาดที่ส่งเข้า daemon — ถ้าเป็นหลัก MB แปลว่า `.dockerignore` ไม่ทำงาน |
| `exporting layers` | ขั้นตอนสุดท้าย เขียน image ลง store |
| `importing cache` | ดึง cache จาก GHA/registry มาใช้ |
| `sha256:...` ท้ายสุด | digest ของ image — ค่าที่ควรใช้ตอน deploy |

**ดึง digest มาใช้ต่อในสคริปต์:**

```bash
docker buildx build --push -t ghcr.io/you/todo:main --metadata-file /tmp/meta.json .
jq -r '."containerimage.digest"' /tmp/meta.json
```

ใน GitHub Actions `docker/build-push-action` ทำให้แล้ว — อ่านได้จาก `steps.build.outputs.digest`
ซึ่งเป็นค่าที่ `build-push.yml` ส่งกลับออกไปให้ job deploy ใช้

---

➡️ ต่อไป: [04 — Docker Compose](04-docker-compose.md)
