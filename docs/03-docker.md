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

### `COPY go.mod go.sum ./` — อ่านทีละ parameter

รูปแบบทั่วไปของ `COPY` คือ `COPY <src...> <dest>` — พารามิเตอร์ตัวสุดท้ายเสมอคือปลายทาง ตัวก่อนหน้าคือไฟล์ต้นทาง (จะมีกี่ตัวก็ได้):

```dockerfile
COPY go.mod go.sum ./
#    ^^^^^^ ^^^^^^ ^^
#    src 1  src 2  dest
```

| ตำแหน่ง | ค่า | ความหมาย |
| --- | --- | --- |
| src 1 | `go.mod` | ไฟล์ประกาศชื่อ module + list dependency ตรง ๆ ที่โค้ดใช้ — อยู่ที่ root ของ build context (โฟลเดอร์ที่รัน `docker build .`) บนเครื่อง host |
| src 2 | `go.sum` | ไฟล์ checksum ของทุก dependency (รวม indirect) ใช้ยืนยันว่าโหลดมาไม่ถูกแก้ไข — คู่กับ `go.mod` เสมอ |
| dest | `./` | ปลายทางในตัว container นี้ ตรงกับ `WORKDIR /app` ที่ประกาศไว้ด้านบน (บรรทัดที่ 3) ดังนั้น `./` ที่นี่คือ `/app` — ผลคือได้ `/app/go.mod` และ `/app/go.sum` |

ถ้าอยากส่งไฟล์เดียวก็เขียนแค่ `COPY go.mod ./`; ถ้าจะรับทั้งโฟลเดอร์เข้ามาแบบ `COPY internal ./internal` — src เป็นโฟลเดอร์ก็ copy ทั้งโฟลเดอร์นั้นเข้าไปในปลายทาง

**ทำไม copy `go.mod go.sum` ก่อน แล้วค่อย copy `cmd`/`internal`?**

นี่คือหัวใจของ layer caching Docker จะ cache layer ไว้และใช้ซ้ำถ้า input ไม่เปลี่ยน (Docker เช็คว่า input เปลี่ยนไหมโดยเทียบ hash ของไฟล์ที่ `COPY` เข้ามา — ถ้า `go.mod`/`go.sum` ไบต์ต่อไบต์เหมือนเดิม layer นั้นและทุก layer ถัดไปที่ยังไม่เจอ input ใหม่จะถูก cache ไว้)

ถ้าเรา `COPY . .` ทีเดียวตั้งแต่แรก → แก้โค้ด 1 บรรทัดในไฟล์ไหนก็ตาม = hash ของ input เปลี่ยน = ทุก layer หลังจากนั้นพัง cache = `go mod download` ใหม่ทุกครั้ง (ช้ามาก เพราะต้องโหลด dependency ทั้งหมดซ้ำทั้งที่ dependency ไม่ได้เปลี่ยนเลย)
แต่แยก copy แบบนี้ → แก้โค้ดใน `cmd`/`internal` ไม่กระทบไฟล์ `go.mod`/`go.sum` → layer `COPY go.mod go.sum ./` และ `RUN go mod download` ยังคง hash เดิม → ใช้ cache เดิม → build เร็วขึ้นหลายเท่า (ดู proof จริงที่หัวข้อ "ลองพิสูจน์เรื่อง cache" ด้านล่าง)

**หลักจำง่าย: อะไรที่เปลี่ยนน้อย ให้ไว้บน อะไรที่เปลี่ยนบ่อย ให้ไว้ล่าง**

### `RUN CGO_ENABLED=0 go build ...` คืออะไร

`CGO_ENABLED` เป็น environment variable ที่ Go toolchain อ่านตอน build (ใส่หน้าคำสั่งแบบนี้ = ตั้งค่าเฉพาะ command นั้นบรรทัดเดียว ไม่ persist ไปบรรทัดอื่น)

- **cgo คืออะไร:** กลไกที่ให้โค้ด Go เรียกไลบรารี C ได้ (เช่นบาง driver ฐานข้อมูล หรือ `net` package บางฟังก์ชันที่ใช้ resolver ของ OS) ถ้าเปิดไว้ (`CGO_ENABLED=1`, เป็นค่า default บนเครื่องที่มี C compiler) binary ที่ได้จะ **dynamically link** กับ `libc` ของระบบที่ build อยู่
- **`CGO_ENABLED=0` ปิดกลไกนี้ทั้งหมด** → Go compiler ใช้ implementation ล้วน ๆ ที่เป็น Go เอง (pure Go) แทนทุกจุดที่เคยพึ่ง C → ผลลัพธ์คือ **static binary** ตัวเดียว ที่มีทุกอย่างรวมอยู่ในไฟล์ (runtime, garbage collector, dependency ที่ import ทั้งหมด) ไม่ต้องพึ่ง shared library ใด ๆ จากระบบที่จะเอาไปรัน

**ทำไมเรื่องนี้ถึงสำคัญกับ multi-stage build:** stage `builder` ใช้ `golang:1.25-alpine` (มี toolchain เต็ม, หนัก) แต่ stage `runner` ใช้ `alpine:3.20` เปล่า ๆ (ไม่มี Go, และ libc ของ alpine คือ `musl` ไม่ใช่ `glibc` แบบ distro อื่น) ถ้า binary ที่ build มาดัน dynamic-link กับ `glibc` ของ builder image เอาไปวางใน `alpine` runner ที่ไม่มี `glibc` จะรันไม่ขึ้นทันที (`exec format error` หรือ error หา shared library ไม่เจอ) การปิด cgo ตัดปัญหานี้ทิ้งไปเลย — เอาไฟล์เดียวไปวางบน base image ไหนก็รันได้ ถึงขั้นใช้ `scratch` (image เปล่าสนิท ไม่มี OS อะไรเลย) ก็ยังรันได้

ผลข้างเคียงที่เห็นชัด: ลองดูด้วยตาได้จาก 🪛 Playground ด้านล่าง (ลบ `CGO_ENABLED=0` ออกแล้วเทียบขนาด binary ด้วย `docker history`)

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

**`COPY --from=builder` คืออะไร ทำไมต้องมี**

`COPY` ปกติ (ที่เห็นใน stage 1) ก็อปไฟล์จาก **build context บนเครื่อง host** (โฟลเดอร์ที่รัน `docker build .`) เข้ามาใน image ที่กำลังสร้าง
แต่ `COPY --from=builder` เปลี่ยนต้นทางเป็น **filesystem ของ stage อื่นที่ build เสร็จไปแล้ว** — ในที่นี้คือ stage ที่ตั้งชื่อว่า `builder` (มาจาก `FROM golang:1.25-alpine AS builder` บรรทัดที่ 2 ของ Dockerfile) นี่คือ flag ที่ทำให้ **multi-stage build** เป็นไปได้: มีหลาย `FROM` ในไฟล์เดียว แต่หยิบผลลัพธ์บางส่วนจาก stage ก่อนหน้าข้ามมาโดยไม่ต้องเอาทั้ง stage นั้นติดไปด้วย

แยกพารามิเตอร์ของบรรทัด `COPY --from=builder /out/api ./api`:

| ตำแหน่ง | ค่า | ความหมาย |
| --- | --- | --- |
| flag | `--from=builder` | บอกว่า src ต่อไปนี้ไม่ได้มาจาก host แต่มาจาก filesystem ของ stage ชื่อ `builder` (จะใช้เลข index อย่าง `--from=0` ก็ได้ แต่ตั้งชื่อด้วย `AS` แล้วอ้างชื่อจะอ่านง่ายกว่า) |
| src | `/out/api` | path **ภายใน stage builder** — ตรงกับที่ `RUN go build -o /out/api ./cmd/api` เขียนไฟล์ binary ไว้ (บรรทัดที่ 11 ของ Dockerfile) |
| dest | `./api` | path ปลายทางใน stage `runner` ปัจจุบัน เทียบกับ `WORKDIR /app` จึงกลายเป็น `/app/api` |

บรรทัดถัดมา `COPY --from=builder /go/bin/migrate /usr/local/bin/migrate` หลักการเดียวกัน: `/go/bin/migrate` คือที่ที่ `go install` (บรรทัดที่ 14) วางไบนารี `migrate` ไว้ในสมัยที่ยังอยู่ stage `builder` (ค่า default ของ `$GOPATH/bin` ใน image `golang:*`) แล้วก็อปมาวางไว้ที่ `/usr/local/bin/migrate` ใน stage `runner` (อยู่ใน `$PATH` อยู่แล้ว เรียก `migrate` เฉย ๆ จากตรงไหนก็ได้โดยไม่ต้องพิมพ์ path เต็ม)

ส่วน `COPY migrations ./migrations` ไม่มี `--from` เพราะโฟลเดอร์ `migrations/` เป็นไฟล์ `.sql` ที่เขียนเองอยู่ใน source code ตรง ๆ — ไม่เคยผ่านการ build เลย เลยก็อปจาก host (build context) ตามปกติเหมือน stage แรก

**ทำไมต้องแยกเป็น 2 stage แล้วก๊อปข้ามแบบนี้ แทนที่จะ build ใน image เดียวจบ:**

หยิบเฉพาะของที่ต้องใช้จริงตอนรัน: binary ที่ compile แล้ว + เครื่องมือ migrate + ไฟล์ migration
**Go toolchain (compiler, ตัว `go` เอง), source code, module cache ไม่ติดไปด้วยเลย** เพราะสิ่งเหล่านี้อยู่ใน stage `builder` ซึ่งไม่ใช่ stage สุดท้าย — Docker เก็บ image สุดท้ายจากเฉพาะ stage ที่ชื่อ/ตำแหน่งท้ายสุด (`runner`) ส่วน stage `builder` ทั้งก้อนจะถูกทิ้งไป เหลือแค่ไฟล์ที่ถูก `COPY --from=builder` หยิบออกมาเท่านั้น

ผลคือ image สุดท้ายเล็กลงมาก (ไม่มี Go toolchain ที่หนักหลายร้อย MB) และมี attack surface น้อยลง (ไม่มี compiler/source code ให้คนที่เจาะเข้ามาใช้ประโยชน์) — ต่างจาก Node ตรงที่ **ไม่มี "production dependencies" ให้แยก stage** เพราะ `go build` รวมทุกอย่างเป็นไบนารีเดียวไปแล้วตั้งแต่ stage แรก (Go จึงใช้ multi-stage แค่ 2 stage ไม่ใช่ 3 แบบที่ Node ต้องมี stage `deps` แยก)

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
