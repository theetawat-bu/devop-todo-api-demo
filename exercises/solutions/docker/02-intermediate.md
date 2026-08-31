# เฉลย — 🐳 Docker ระดับ 2

⬅️ [กลับไปที่โจทย์](../../docker/02-intermediate.md)

---

## D2.1 layer cache

```bash
docker builder prune -af
time docker build -t t .                    # ~30-60 วิ
time docker build -t t .                    # ~1 วิ  (CACHED ทุก layer)
echo "// x" >> cmd/api/main.go
time docker build -t t .                    # ~5-15 วิ
```

ในกรณีที่ 3:

| layer | สถานะ |
| --- | --- |
| `COPY go.mod go.sum` | CACHED |
| `RUN go mod download` | **CACHED** ← ตัวที่ประหยัดเวลาที่สุด |
| `COPY cmd ./cmd` | ทำใหม่ (ไฟล์เปลี่ยน) |
| `COPY internal ./internal` | CACHED (ไม่เปลี่ยน) |
| `RUN go build ...` | ทำใหม่ |

**ทำไมถึงเป็นแบบนี้:** Docker คำนวณ hash จากคำสั่ง + ไฟล์ที่คำสั่งนั้นแตะ ถ้า hash เดิม → ใช้ layer เดิม
และ **พอ layer หนึ่งพัง cache ทุก layer หลังจากนั้นพังหมด** จึงต้องเรียงจาก "เปลี่ยนน้อย" ไป "เปลี่ยนบ่อย"

---

## D2.2 ทำลาย cache

ย้าย `COPY cmd ./cmd` / `COPY internal ./internal` ขึ้นไปก่อน `RUN go mod download` แล้ว build หลังแก้โค้ด → กลับไปช้าเท่า build ครั้งแรก (~30-60 วิ)

**ทำไม:** `COPY cmd` พัง cache → `go mod download` ที่อยู่ถัดไปพัง cache ตาม → ต้องดาวน์โหลด module ใหม่หมดจาก module proxy
คิดเป็นเวลาที่เสียของทั้งทีม: build วันละ 20 ครั้ง × 45 วิ × 5 คน = เสียเวลาไปเกือบ 1.5 ชั่วโมงต่อวัน

---

## D2.3 `.dockerignore`

```bash
go mod download
mv .dockerignore .dockerignore.bak
docker build -t t .    # => transferring context: ใหญ่กว่าเดิมชัดเจน (รวม .git, bin เก่า ฯลฯ)
mv .dockerignore.bak .dockerignore
docker build -t t .    # => transferring context: ~เล็กมาก (แค่ cmd/, internal/, go.mod, go.sum, migrations/)
```

**ผลเสีย 2 ข้อ:**

1. **ช้า** — ต้องส่งไฟล์เข้า daemon ทุกครั้งที่ build; module cache ของ Go เก็บอยู่ที่ `$GOPATH/pkg/mod` นอก working directory อยู่แล้วจึงไม่ค่อยเป็นปัญหาเท่าฝั่ง Node แต่ `.git` และ binary ที่ build ค้างไว้ในเครื่องยังโดนส่งเข้าไปถ้าไม่กัน
2. **ความลับหลุด** — `.env` และ `.git` (ที่มี history ทั้งหมด) จะติดเข้าไปใน build context และอาจถูก `COPY . .` เข้า image

ข้อ 2 คือเหตุผลที่ควรมี `.dockerignore` แม้จะไม่สนใจเรื่องความเร็ว

---

## D2.4 ลดขนาด image

```dockerfile
# ใน stage builder
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api
```

**สำคัญ:** `-ldflags="-s -w"` ตัด debug symbol table (`-s`) และ DWARF debug info (`-w`) ออกจาก binary — ลดขนาดได้มักจะ 20-30% แลกกับการ debug ด้วย `dlv`/`gdb` ทำไม่ได้อีกต่อไป (ปกติไม่จำเป็นใน production เพราะ Go binary debug ยากอยู่แล้วโดยธรรมชาติ)

วิธีอื่นที่ได้ผล:

- ไม่ copy `migrate` binary เข้า runner ถ้าย้าย migration ไปรันเป็น initContainer แยกต่างหากบน k8s แทน (ประหยัดได้เพราะตัด `migrate` CLI ทั้งก้อนออกจาก image ของ API)
- เทียบ `alpine:3.20` กับ `gcr.io/distroless/static` — distroless เล็กกว่าและไม่มี shell/package manager เลย (ดู D4.4 เรื่อง trade-off)
- ตรวจว่า `CGO_ENABLED=0` ถูกตั้งไว้จริง — ถ้าไม่ตั้ง Go จะ link กับ glibc ของระบบแบบ dynamic ทำให้ binary ใหญ่ขึ้นและพกไปรันข้าม base image ไม่ได้

---

## D2.5 ARG vs ENV

```bash
docker build --build-arg APP_VERSION=1.2.3 -t t .
docker run --rm -p 3000:3000 -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" t
curl localhost:3000/healthz
# {"status":"ok","version":"1.2.3",...}
```

ใน Dockerfile ต้องมีทั้งสองบรรทัด:

```dockerfile
ARG APP_VERSION=dev        # มีอยู่เฉพาะตอน build
ENV APP_VERSION=$APP_VERSION   # ส่งต่อให้ตอน runtime
```

**ถ้ามีแค่ `ARG`:** ค่าจะหายไปทันทีที่ build เสร็จ — `os.Getenv("APP_VERSION")` จะได้ค่าว่างตอนรัน

| | ARG | ENV |
| --- | --- | --- |
| มีผลตอน | build | build + runtime |
| เห็นใน `docker inspect` | ❌ | ✅ |
| override ตอน run ได้ | ❌ | ✅ ด้วย `-e` |
| ใส่ความลับได้ไหม | ❌ (ติดใน history) | ❌ (เห็นใน inspect) |

---

## D2.6 HEALTHCHECK

```bash
docker run -d --name hc -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" devops-todo-api:local
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
docker run --rm devops-todo-api:local sh -c "echo hi"
# hi
```

**ทำไมได้:** `CMD` เป็นแค่ค่าเริ่มต้น ถ้าใส่คำสั่งต่อท้าย `docker run` มันจะ**ถูกแทนที่ทั้งหมด** — ที่ใช้ `sh -c` ได้เพราะ base image เป็น `alpine` ซึ่งมี shell ติดมาด้วย (ถ้าเป็น distroless แบบ D4.4 จะทำแบบนี้ไม่ได้เพราะไม่มี shell)

ถ้าเปลี่ยนเป็น `ENTRYPOINT ["./api"]` แล้ว `docker run image sh -c "..."` จะกลายเป็นการรัน `./api` พร้อม argument `sh -c "..."` ต่อท้าย (ซึ่งไม่ตรงกับที่ `./api` คาดหวัง) เพราะสิ่งที่พิมพ์ต่อท้ายจะถูก**ต่อท้าย** ENTRYPOINT ไม่ใช่แทนที่

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

/app $ ./api                     # รันเองเพื่อดู error เต็ม ๆ
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
