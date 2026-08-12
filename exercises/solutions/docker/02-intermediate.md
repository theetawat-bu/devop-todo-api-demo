# เฉลย — 🐳 Docker ระดับ 2

⬅️ [กลับไปที่โจทย์](../../docker/02-intermediate.md)

---

## D2.1 layer cache

```bash
docker builder prune -af
time docker build -t t .                    # ~60-120 วิ
time docker build -t t .                    # ~1 วิ  (CACHED ทุก layer)
echo "// x" >> src/index.ts
time docker build -t t .                    # ~10-20 วิ
```

ในกรณีที่ 3:

| layer | สถานะ |
| --- | --- |
| `COPY package*.json` | CACHED |
| `RUN npm ci` | **CACHED** ← ตัวที่ประหยัดเวลาที่สุด |
| `RUN npx prisma generate` | CACHED |
| `COPY src ./src` | ทำใหม่ (ไฟล์เปลี่ยน) |
| `RUN npm run build` | ทำใหม่ |

**ทำไมถึงเป็นแบบนี้:** Docker คำนวณ hash จากคำสั่ง + ไฟล์ที่คำสั่งนั้นแตะ ถ้า hash เดิม → ใช้ layer เดิม
และ **พอ layer หนึ่งพัง cache ทุก layer หลังจากนั้นพังหมด** จึงต้องเรียงจาก "เปลี่ยนน้อย" ไป "เปลี่ยนบ่อย"

---

## D2.2 ทำลาย cache

ย้าย `COPY src ./src` ขึ้นไปก่อน `RUN npm ci` แล้ว build หลังแก้โค้ด → กลับไปช้าเท่า build ครั้งแรก (~60-120 วิ)

**ทำไม:** `COPY src` พัง cache → `npm ci` ที่อยู่ถัดไปพัง cache ตาม → ต้องดาวน์โหลด dependency ใหม่หมด
คิดเป็นเวลาที่เสียของทั้งทีม: build วันละ 20 ครั้ง × 60 วิ × 5 คน = เสียเวลาไปเกือบ 2 ชั่วโมงต่อวัน

---

## D2.3 `.dockerignore`

```bash
npm install
mv .dockerignore .dockerignore.bak
docker build -t t .    # => transferring context: ~300MB
mv .dockerignore.bak .dockerignore
docker build -t t .    # => transferring context: ~200kB
```

**ผลเสีย 2 ข้อ:**

1. **ช้า** — ต้องส่งไฟล์เป็นแสนเข้า daemon ทุกครั้งที่ build และ `node_modules` ของ host (macOS/Windows) อาจทับของใน image ที่ติดตั้งสำหรับ Linux → binary ผิด platform
2. **ความลับหลุด** — `.env` และ `.git` (ที่มี history ทั้งหมด) จะติดเข้าไปใน build context และอาจถูก `COPY . .` เข้า image

ข้อ 2 คือเหตุผลที่ควรมี `.dockerignore` แม้จะไม่สนใจเรื่องความเร็ว

---

## D2.4 ลดขนาด image

```dockerfile
# ใน stage deps
RUN npm ci --omit=dev && npm cache clean --force
```

**สำคัญ:** ต้องอยู่ใน `RUN` **บรรทัดเดียวกัน** ถ้าแยกเป็นสองบรรทัด cache จะถูกเขียนใน layer หนึ่งแล้วลบใน layer ถัดไป — ซึ่ง**ขนาดไม่ลดลงเลย** เพราะ layer เก่ายังอยู่

วิธีอื่นที่ได้ผล:

- ไม่ copy `prisma/` เข้า runner ถ้าไม่ได้รัน migrate จาก container (ประหยัดได้ถ้าย้ายไป initContainer)
- ตัด source map ออกใน production (`"sourceMap": false`)
- ใช้ `node:22-slim` ถ้าเจอปัญหา musl libc ของ alpine — แต่จะใหญ่กว่า

---

## D2.5 ARG vs ENV

```bash
docker build --build-arg APP_VERSION=1.2.3 -t t .
docker run --rm -p 3000:3000 -e DATABASE_URL=... t
curl localhost:3000/healthz
# {"status":"ok","version":"1.2.3",...}
```

ใน Dockerfile ต้องมีทั้งสองบรรทัด:

```dockerfile
ARG APP_VERSION=dev        # มีอยู่เฉพาะตอน build
ENV APP_VERSION=$APP_VERSION   # ส่งต่อให้ตอน runtime
```

**ถ้ามีแค่ `ARG`:** ค่าจะหายไปทันทีที่ build เสร็จ — `process.env.APP_VERSION` จะเป็น `undefined` ตอนรัน

| | ARG | ENV |
| --- | --- | --- |
| มีผลตอน | build | build + runtime |
| เห็นใน `docker inspect` | ❌ | ✅ |
| override ตอน run ได้ | ❌ | ✅ ด้วย `-e` |
| ใส่ความลับได้ไหม | ❌ (ติดใน history) | ❌ (เห็นใน inspect) |

---

## D2.6 HEALTHCHECK

```bash
docker run -d --name hc -e DATABASE_URL=... devops-todo-api:local
docker ps --format '{{.Names}}\t{{.Status}}'
# hc   Up 5 seconds (health: starting)     ← ช่วง start-period
# hc   Up 40 seconds (healthy)
```

ทำให้ unhealthy โดยแก้เป็น `CMD curl -fsS http://127.0.0.1:3000/ไม่มีจริง` แล้ว build ใหม่ → หลัง 3 ครั้ง (retries) จะขึ้น `(unhealthy)`

**`--start-period` มีไว้ทำไม:** ให้เวลาแอป boot โดยยังไม่นับว่าล้มเหลว ถ้าไม่มี แอปที่ boot 20 วิจะถูกมองว่า unhealthy ตั้งแต่ต้นและอาจโดน restart วนไม่จบ
แนวคิดเดียวกับ `startupProbe` ของ Kubernetes

---

## D2.7 CMD vs ENTRYPOINT

```bash
docker run --rm devops-todo-api:local node -e "console.log(process.version)"
# v22.x.x
```

**ทำไมได้:** `CMD` เป็นแค่ค่าเริ่มต้น ถ้าใส่คำสั่งต่อท้าย `docker run` มันจะ**ถูกแทนที่ทั้งหมด**

ถ้าเปลี่ยนเป็น `ENTRYPOINT ["node"]` แล้ว `docker run image -e "..."` จะกลายเป็น `node -e "..."` เพราะสิ่งที่พิมพ์ต่อท้ายจะถูก**ต่อท้าย** ENTRYPOINT ไม่ใช่แทนที่

| | เขียนแบบไหน | เหมาะกับ |
| --- | --- | --- |
| CMD | คำสั่งเริ่มต้นที่แทนที่ได้ | image ทั่วไป (แบบที่เราใช้) |
| ENTRYPOINT | คำสั่งตายตัว | image ที่ทำตัวเหมือน CLI tool |

---

## D2.8 debug container ที่ตายทันที

```bash
# override คำสั่งด้วย shell
docker run --rm -it --entrypoint sh devops-todo-api:local
# หรือถ้าใช้ CMD เฉย ๆ
docker run --rm -it devops-todo-api:local sh

/app $ node dist/index.js        # รันเองเพื่อดู error เต็ม ๆ
/app $ env | grep DATABASE
```

ถ้า container ตายไปแล้วและอยากดู log ของรอบก่อน:

```bash
docker logs <container>          # log ยังอยู่จนกว่าจะ docker rm
```

**ทำไมวิธีนี้ใช้ได้:** ปัญหาอยู่ที่**คำสั่งที่รัน** ไม่ใช่ที่ตัว image เอง — พอเปลี่ยนคำสั่งเป็น shell เราก็เข้าไปสำรวจ filesystem และ env ได้ตามปกติ

---

## 🎯 ต่อยอด

- ลอง `docker build --progress=plain` เพื่อเห็น output เต็มของทุก step
- ลอง `docker build --no-cache` แล้วเทียบเวลา
- อ่าน `docker inspect` ส่วน `RootFS.Layers` แล้วนับว่ามีกี่ layer จริง ๆ
