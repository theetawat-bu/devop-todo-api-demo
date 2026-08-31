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
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
```

**ทำไม copy `go.mod go.sum` ก่อน แล้วค่อย copy `cmd`/`internal`?**

นี่คือหัวใจของ layer caching Docker จะ cache layer ไว้และใช้ซ้ำถ้า input ไม่เปลี่ยน
ถ้าเรา `COPY . .` ทีเดียวตั้งแต่แรก → แก้โค้ด 1 บรรทัด = ทุก layer หลังจากนั้นพัง cache = `go mod download` ใหม่ทุกครั้ง (ช้ามาก)
แต่แยก copy แบบนี้ → แก้โค้ดไม่กระทบ `go.mod`/`go.sum` → `go mod download` ใช้ cache เดิม → build เร็วขึ้นหลายเท่า

**หลักจำง่าย: อะไรที่เปลี่ยนน้อย ให้ไว้บน อะไรที่เปลี่ยนบ่อย ให้ไว้ล่าง**

`CGO_ENABLED=0` สำคัญ: ปิด cgo แล้ว Go จะได้ static binary ตัวเดียวจบ ไม่ต้องพึ่ง libc ของ base image
ทำให้ก็อปไปรันบน image แทบเปล่า ๆ (เช่น `alpine`, หรือ `scratch`/`distroless`) ได้โดยไม่พัง

### golang-migrate CLI — ติดตั้งไว้ใน stage เดียวกัน

```dockerfile
RUN CGO_ENABLED=0 go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1
```

นี่คือของที่แทน `npx prisma migrate deploy` เดิม — ต่างกันตรงที่ไม่ใช่ script ที่รันผ่าน runtime เดียวกับแอป
แต่เป็น **CLI ไบนารีแยกต่างหาก** ที่รู้จักแค่โฟลเดอร์ `.sql` migration กับ `DATABASE_URL` เท่านั้น

### Stage 2: runner — image ที่ deploy จริง

```dockerfile
COPY --from=builder /out/api ./api
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY migrations ./migrations
```

หยิบเฉพาะของที่ต้องใช้: binary ที่ compile แล้ว + เครื่องมือ migrate + ไฟล์ migration
**Go toolchain, source code, module cache ไม่ติดไปด้วยเลย** — ต่างจาก Node ตรงที่ **ไม่มี "production dependencies" ให้แยก stage** เพราะ `go build` รวมทุกอย่างเป็นไบนารีเดียวไปแล้วตั้งแต่ stage แรก (Go จึงใช้ multi-stage แค่ 2 stage ไม่ใช่ 3 แบบที่ Node ต้องมี stage `deps` แยก)

### ทำไมต้องสร้าง user เอง

```dockerfile
RUN addgroup -S app && adduser -S -G app -u 1000 app
RUN chown -R app:app /app
USER app
```

default ของ container คือรันเป็น root ถ้าคนแฮกเข้ามาได้ ก็ได้ root ไปเลย
`golang:*-alpine`/`alpine` ไม่มี user สำเร็จรูปมาให้เหมือน `node:*-alpine` (ที่มี `node` user ในตัว) เลยต้องสร้างเอง
ผลลัพธ์เดียวกัน: ลดความเสียหายลงมาก และเป็นเงื่อนไขบังคับของ `runAsNonRoot: true` ใน k8s manifest ของเรา

### HEALTHCHECK

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -fsS http://127.0.0.1:3000/healthz || exit 1
```

Docker จะยิงเช็คเองแล้วรายงานสถานะ `healthy`/`unhealthy` — compose ใช้ค่านี้ใน `depends_on: condition: service_healthy`
`start-period` = ช่วงผ่อนผันตอน boot ยังไม่นับว่าล้มเหลว

### `.dockerignore` สำคัญกว่าที่คิด

ไฟล์นี้กัน `.git`, `.env`, `docs`, `exercises` ไม่ให้ถูกส่งเข้า build context
- ถ้าไม่กัน ไฟล์พวกนี้ → build context ใหญ่โดยไม่จำเป็น ส่งช้าทุกครั้งที่ `docker build`
- ถ้าไม่กัน `.env` → **ความลับหลุดเข้าไปอยู่ใน image**

(Go ไม่มีปัญหาแบบ `node_modules` เพราะไม่มีโฟลเดอร์ dependency ที่ก็อปลงเครื่องแบบนั้น — module cache อยู่นอก build context อยู่แล้ว)

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
echo "// x" >> cmd/api/main.go
docker build -t t1 .                       # เร็วอยู่ — เพราะ go mod download ยัง CACHED
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
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETPLATFORM   # เช่น linux/arm64  — เครื่องปลายทาง
ARG TARGETOS
ARG TARGETARCH
RUN echo "build บน $BUILDPLATFORM เพื่อไปรันบน $TARGETPLATFORM"
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/api ./cmd/api
```

เทคนิคขั้นสูง: ให้ stage `builder` รันบน `$BUILDPLATFORM` เสมอ (เร็ว ไม่ต้องจำลองด้วย QEMU) แล้วสั่ง Go
**cross-compile** ไปเป็น arch ปลายทางด้วย `GOOS`/`GOARCH` — วิธีนี้ตัด QEMU ออกจาก build ทั้งหมด เร็วกว่าเดิมมาก
Go ทำ cross-compile ได้ในตัวโดยไม่ต้องติดตั้งอะไรเพิ่ม (ต่างจาก Node ที่ผลลัพธ์เป็น JS ตีความตอนรัน จึงไม่มีแนวคิด cross-compile แบบนี้ — แต่ก็ไม่มีปัญหา arch เพราะ JS ไม่ผูกกับ CPU)
`TARGETOS`/`TARGETARCH` เป็นตัวแปรที่ BuildKit generate ให้อัตโนมัติจาก `TARGETPLATFORM` ไม่ต้อง parse เอง

---

## X3. ARG มีขอบเขตแค่ไหน — จุดที่คนพลาดบ่อย

```dockerfile
ARG GO_VERSION=1.25            # ← ก่อน FROM = global แต่ใช้ได้เฉพาะในบรรทัด FROM
FROM golang:${GO_VERSION}-alpine AS builder
RUN echo $GO_VERSION           # ← ว่าง! ต้องประกาศซ้ำในแต่ละ stage

FROM alpine:3.20 AS runner
ARG GO_VERSION                 # ← ประกาศซ้ำ (ไม่ต้องใส่ค่า) ถึงจะใช้ได้
RUN echo $GO_VERSION           # 1.25
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

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] `docker images` เทียบขนาด `devops-todo-api:local` กับ `golang:1.25-alpine` เปล่า ๆ — ต่างกันเท่าไร
- [ ] ลบบรรทัด `CGO_ENABLED=0` ออกแล้ว build ใหม่ ดูว่า binary ใหญ่ขึ้น/เปลี่ยนไปยังไง (`docker history`)
- [ ] แก้ `internal/todos/handler.go` แล้ว build ซ้ำ 2 ครั้งติดกัน — layer ไหน CACHED บ้าง
- [ ] ลองสลับลำดับ `COPY go.mod go.sum` กับ `COPY cmd ./cmd` ดูว่า cache พังไหม
- [ ] `docker run --rm -it --entrypoint sh devops-todo-api:local` เข้าไปดูว่าใน image มีไฟล์อะไรบ้าง (ไม่มี source code ติดไปเลย)

➡️ ต่อไป: [04 — Docker Compose](04-docker-compose.md)
