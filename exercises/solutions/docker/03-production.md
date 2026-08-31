# เฉลย — 🐳 Docker ระดับ 3

⬅️ [กลับไปที่โจทย์](../../docker/03-production.md)

---

## D3.1 non-root

```bash
docker exec -it <c> sh
/app $ touch /test
touch: /test: Permission denied
```

**ทำไมสำคัญ:** ถ้าแอปมีช่องโหว่ที่ทำให้รันคำสั่งได้ (RCE) การรันเป็น root แปลว่าผู้โจมตีได้ root ใน container ทันที
จากนั้นถ้าเจอช่องโหว่ของ container runtime ต่อ (container escape) ก็อาจได้ root บน host ต่อไปอีก
`USER app` ตัดขั้นแรกของบันไดนี้ทิ้ง

---

## D3.2 read-only root filesystem

```bash
docker run --rm --read-only --tmpfs /tmp \
  -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" -p 3000:3000 devops-todo-api:local
```

**ทำไมต้องมี `--tmpfs /tmp`:** แม้ Go เองจะเขียนไฟล์ชั่วคราวลง `/tmp` น้อยกว่า runtime อื่นมาก แต่ library บางตัวและเครื่องมือ debug ก็ยังใช้ `/tmp` อยู่ ถ้าเขียนไม่ได้จะพัง
`tmpfs` คือ filesystem ในหน่วยความจำ — เขียนได้แต่หายเมื่อ container ดับ ซึ่งเป็นสิ่งที่เราต้องการพอดี

ใน k8s:

```yaml
securityContext:
  readOnlyRootFilesystem: true
volumeMounts:
  - name: tmp
    mountPath: /tmp
volumes:
  - name: tmp
    emptyDir: {}
```

**ได้อะไร:** ผู้โจมตีเขียน webshell หรือ malware ลง filesystem ไม่ได้ ต่อให้เจาะเข้ามาได้แล้วก็ตาม

---

## D3.3 OOMKilled

```bash
docker run -d --name oom --memory=64m -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" devops-todo-api:local
# ยิงโหลดหรือรอให้ process ใช้ memory เกิน limit
docker inspect oom --format '{{.State.ExitCode}} {{.State.OOMKilled}}'
# 137 true
```

**137 มาจากไหน:** `128 + signal` โดย SIGKILL = 9 → `128 + 9 = 137`

**ทำไมไม่มี graceful shutdown:** SIGKILL ถูกดักไม่ได้เลย — kernel ฆ่า process ทันที
ต่างจาก SIGTERM (15, exit code 143) ที่แอปดักได้และปิดงานให้เรียบร้อยก่อน

**บทเรียน:** ตั้ง memory limit ต่ำเกินไป = แอปตายกลางคัน request หาย ผู้ใช้เห็น error โดยไม่มีสัญญาณเตือนล่วงหน้า
ให้ดูค่าจริงจาก monitoring แล้วตั้งเผื่อ ไม่ใช่เดา

---

## D3.4 CPU throttling

```bash
docker run -d --name slow --cpus=0.2 -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" -p 3001:3000 devops-todo-api:local
time curl -s localhost:3001/api/todos > /dev/null    # ช้ากว่าชัดเจน
docker stats slow --no-stream
```

| | เกิน CPU limit | เกิน memory limit |
| --- | --- | --- |
| เกิดอะไร | **throttle** — ทำงานช้าลง | **OOMKilled** — ตายทันที |
| ผู้ใช้เห็น | ช้า | error / connection reset |
| กู้เองได้ไหม | ได้ พอโหลดลดก็กลับปกติ | ต้อง restart |

**ทำไมต่างกัน:** CPU เป็นทรัพยากรที่ "แบ่งเวลากันใช้" ได้ แต่ memory เป็นทรัพยากรที่ "มีหรือไม่มี" — จะแบ่งครึ่งไม่ได้
นี่คือเหตุผลที่หลายทีมเลือกตั้ง memory limit แต่**ไม่ตั้ง** CPU limit (ตั้งแค่ requests) เพื่อให้ใช้ CPU ว่างของ node ได้เต็มที่

---

## D3.5 สแกนช่องโหว่

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy image --severity HIGH,CRITICAL devops-todo-api:local
```

แยกที่มาให้ออก — วิธีแก้คนละแบบ:

| ที่มา | ตัวอย่าง | วิธีแก้ |
| --- | --- | --- |
| **OS package** (base image) | `openssl`, `busybox`, `zlib` | อัปเดต base image เป็น tag ล่าสุด — แก้ได้ทีเดียวหลายรายการ |
| **Go module** | dependency ใน `go.mod` | `go get -u <module>` แล้ว `go mod tidy`, หรือดู `govulncheck` |
| **แก้ไม่ได้** | ยังไม่มี patch จาก upstream | บันทึกไว้ว่ารับความเสี่ยง + ตั้ง `.trivyignore` พร้อมวันหมดอายุ |

**สิ่งที่คนมักทำผิด:** เห็น CVE เยอะแล้วปิดการสแกนทิ้ง — ที่ถูกคือตั้ง threshold ให้สมเหตุสมผล (HIGH ขึ้นไป) แล้วจัดการทีละรายการ

---

## D3.6 pin digest

```bash
docker pull golang:1.25-alpine
docker inspect --format='{{index .RepoDigests 0}}' golang:1.25-alpine
# golang@sha256:abc123…
```

```dockerfile
FROM golang:1.25-alpine@sha256:abc123… AS builder
```

**ทำไม tag ไม่ปลอดภัย:** `golang:1.25-alpine` วันนี้กับพรุ่งนี้อาจเป็นคนละ image — เจ้าของ repo push ทับ tag เดิมได้ตลอด
ผลคือ build เดิม โค้ดเดิม แต่ได้ image ไม่เหมือนเดิม = debug ไม่ได้

**ข้อเสียของการ pin:** ไม่ได้รับ security patch อัตโนมัติอีกต่อไป → **ต้องมี Renovate/Dependabot คู่กันเสมอ**
pin โดยไม่มีระบบอัปเดตอัตโนมัติ = แลกปัญหาหนึ่งกับอีกปัญหาหนึ่ง

---

## D3.7 graceful shutdown

```bash
docker run -d --name g -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" devops-todo-api:local
time docker stop g       # ~0.5 วิ

# คอมเมนต์ signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) ใน cmd/api/main.go แล้ว build ใหม่
time docker stop g2      # ~10 วิ (ครบ timeout แล้วโดน SIGKILL)
```

ลำดับที่เกิดขึ้นจริง:

```
docker stop → SIGTERM → แอปดักได้ผ่าน signal.Notify → srv.Shutdown(ctx) → รอ request ที่ค้างจนจบ
           → ปิด DB pool (pgxpool.Close()) → os.Exit(0)   ← จบใน 0.5 วิ

ถ้าไม่ดัก SIGTERM → รอครบ 10 วิ → SIGKILL → request ที่ค้างขาดกลางคัน
```

**เชื่อมกับ k8s:** `terminationGracePeriodSeconds: 30` คือเวลาที่ k8s รอก่อนส่ง SIGKILL
ถ้าแอปไม่ดัก SIGTERM ทุกครั้งที่ deploy จะมีผู้ใช้บางคนเจอ connection ขาด — เป็น downtime ที่มองไม่เห็นใน dashboard

---

## D3.8 ความลับใน image

```bash
docker history --no-trunc devops-todo-api:local | grep -i -E "password|secret|token|key"
trivy image --scanners secret devops-todo-api:local
```

**ทำไม `RUN rm` ไม่ช่วย:**

```dockerfile
COPY .env /app/.env      # layer 5 — ไฟล์อยู่ในนี้ตลอดกาล
RUN rm /app/.env         # layer 6 — แค่ทำเครื่องหมายว่าลบ
```

image เป็น layer ซ้อนกัน layer 5 ยังอยู่ครบ ใครก็ตามที่ `docker save` แล้วแตก tar ออกมาก็อ่านไฟล์นั้นได้
**ความลับที่เคยเข้า layer แล้ว ถือว่าหลุดถาวร** — ทางแก้เดียวคืออย่าให้เข้าไปตั้งแต่แรก (ใช้ `--mount=type=secret` หรือส่งตอน runtime)

---

## D3.9 structured logging

โปรเจกต์นี้ log เป็น JSON อยู่แล้วใน `internal/app/app.go` (ผ่าน middleware ของ Gin ที่ config ให้ log แบบ structured):

```bash
docker logs <c> | jq 'select(.level=="info") | .path'
```

**ทำไมห้ามเขียน log ลงไฟล์ใน container:**

- container ถูกลบเมื่อไร log หายหมด
- filesystem ของ container ควรเป็น read-only
- ระบบรวบรวม log (Loki, Fluent Bit, CloudWatch) อ่านจาก stdout ของ container เป็นมาตรฐาน
- ถ้าเขียนไฟล์ต้องมาจัดการ log rotation เองอีก

**หลัก 12-factor: log คือ event stream — แอปแค่พ่นออก stdout ส่วนการเก็บเป็นหน้าที่ของ platform**

---

## 🎯 ต่อยอด

- ลอง `--security-opt no-new-privileges` และ `--cap-drop ALL`
- ลอง `docker run --user 1001` แล้วดูว่าพังตรงไหน (ไฟล์ที่ chown ให้ uid 1000)
- ตั้ง `.trivyignore` พร้อมคอมเมนต์เหตุผลและวันที่ทบทวน
