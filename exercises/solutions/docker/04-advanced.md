# เฉลย — 🐳 Docker ระดับ 4

⬅️ [กลับไปที่โจทย์](../../docker/04-advanced.md)

---

## D4.1 multi-arch

```bash
docker buildx create --name multi --use
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/<you>/todo:multi --push .
docker manifest inspect ghcr.io/<you>/todo:multi
```

**ทำไมต้อง `--push` (ใส่ `--load` ไม่ได้):** local image store ของ Docker เก็บได้แค่ architecture เดียว
multi-arch ต้องอยู่ในรูป **manifest list** ซึ่งเป็นแนวคิดของ registry — เก็บใน local ไม่ได้

**เกิดอะไรตอน pull:** client ส่ง architecture ของตัวเองไป registry เลือก image ที่ตรงให้อัตโนมัติ
นี่คือเหตุผลที่ `docker pull node:22-alpine` บน Mac M-series กับบน server Intel ได้คนละ binary แต่ใช้คำสั่งเดียวกัน

ใน workflow:

```yaml
- uses: docker/setup-qemu-action@v3 # จำเป็นถ้าจะ build arch ที่ต่างจาก runner
- uses: docker/build-push-action@v6
  with:
    platforms: linux/amd64,linux/arm64
```

⚠️ build ข้าม arch ด้วย QEMU **ช้ากว่ามาก** (3-10 เท่า) — ถ้าเป็นไปได้ให้ใช้ native runner ของแต่ละ arch แล้วรวม manifest ทีหลัง

---

## D4.2 cache mount

```dockerfile
RUN --mount=type=cache,target=/root/.npm npm ci
```

| | layer cache | cache mount |
| --- | --- | --- |
| ทำงานยังไง | ข้าม step ทั้ง step ถ้า input เหมือนเดิม | step ยังรัน แต่มีโฟลเดอร์ที่คงอยู่ข้าม build ให้ใช้ |
| พอ lockfile เปลี่ยน | **พังทั้ง step** ต้องโหลดใหม่หมด | ยังใช้ของเดิมที่โหลดไว้แล้วได้ ดาวน์โหลดเฉพาะที่เปลี่ยน |
| อยู่ใน image สุดท้ายไหม | อยู่ (เป็น layer) | **ไม่อยู่** — ไม่ทำให้ image ใหญ่ขึ้น |

ทั้งสองอย่างใช้ร่วมกันได้และควรใช้ร่วมกัน

⚠️ ใน GitHub Actions cache mount **ไม่คงอยู่ข้าม run** โดยอัตโนมัติ ต้องใช้ `cache-from/cache-to: type=gha` ช่วย

---

## D4.3 secret mount

```dockerfile
RUN --mount=type=secret,id=npmtoken \
    NPM_TOKEN=$(cat /run/secrets/npmtoken) npm ci
```

```bash
docker build --secret id=npmtoken,src=./token.txt .
docker history --no-trunc t | grep -i token   # ไม่เจอ
```

**ทำไมปลอดภัย:** secret ถูก mount เป็น tmpfs เฉพาะช่วงที่ `RUN` นั้นทำงาน พอจบ step มันหายไปโดยไม่เคยถูกเขียนลง layer เลย

เทียบกับวิธีที่**ผิด** 2 แบบ:

```dockerfile
ARG NPM_TOKEN          # ❌ ติดใน build history
ENV NPM_TOKEN=xxx      # ❌ เห็นได้จาก docker inspect ทุกคน
```

ใน GitHub Actions:

```yaml
- uses: docker/build-push-action@v6
  with:
    secrets: |
      npmtoken=${{ secrets.NPM_TOKEN }}
```

---

## D4.4 distroless

```dockerfile
FROM gcr.io/distroless/nodejs22-debian12 AS runner
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=builder /app/dist ./dist
USER nonroot
CMD ["dist/index.js"]     # ไม่มี "node" นำหน้า — entrypoint ของ image เป็น node อยู่แล้ว
```

**สิ่งที่ต้องเปลี่ยนตาม:**

| เดิม | ทำไมใช้ไม่ได้ | ทางแก้ |
| --- | --- | --- |
| `CMD ["sh","-c","prisma migrate deploy && node …"]` | ไม่มี shell | ย้าย migration ไป initContainer / Job |
| `HEALTHCHECK CMD curl …` | ไม่มี curl | ใช้ probe ของ k8s แทน |
| `docker exec -it sh` | ไม่มี shell | ใช้ `kubectl debug` แนบ ephemeral container |

**ได้อะไร:** พื้นที่โจมตีเล็กลงมาก — ไม่มี shell, package manager, หรือ utility ให้ผู้โจมตีใช้ต่อ
**เสียอะไร:** debug ยากขึ้นชัดเจน

**ควรใช้เมื่อไร:** production ที่มีความเสี่ยงสูงและมี observability ดีพอจนไม่ต้อง exec เข้าไปดูบ่อย ๆ
ถ้าทีมยังต้อง exec เข้า container สัปดาห์ละหลายครั้ง = ยังไม่พร้อม

---

## D4.5 base image ของทีม

```dockerfile
# company-base/Dockerfile
FROM node:22-alpine@sha256:…
RUN apk add --no-cache curl tini ca-certificates
RUN addgroup -g 1000 app && adduser -u 1000 -G app -D app
```

```dockerfile
# ในโปรเจกต์
FROM harbor.company.internal/base-images/node:22 AS builder
```

**ได้:** มาตรฐานเดียวกันทั้งองค์กร, แก้ CVE ที่เดียวมีผลทุกโปรเจกต์, build เร็วขึ้นเพราะ layer ร่วมกัน
**เสีย:** ต้องมีเจ้าของและรอบการอัปเดตที่ชัดเจน — **base image ที่ไม่มีคนดูแลคือแหล่งสะสม CVE ที่แย่กว่าไม่มีเลย**

---

## D4.6 reproducible build

```bash
export SOURCE_DATE_EPOCH=$(git log -1 --pretty=%ct)
docker buildx build --output type=image,rewrite-timestamp=true -t t .
```

สิ่งที่มักทำให้ digest ไม่ตรง:

| สาเหตุ | แก้ยังไง |
| --- | --- |
| `FROM` ใช้ tag ไม่ใช่ digest | pin digest |
| timestamp ของไฟล์ที่ copy | ตั้ง `SOURCE_DATE_EPOCH` |
| `npm ci` ดึงเวอร์ชันต่างกัน | lockfile + `--ignore-scripts` |
| build id / uuid ที่ถูกฝังตอน build | ตัดออกหรือทำให้ deterministic |

**ทำไมสำคัญ:** ถ้า build จากโค้ดเดิมแล้วได้ digest เดิม แปลว่า **พิสูจน์ได้ว่า image ที่รันใน production มาจากโค้ดชุดนั้นจริง**
นี่คือรากฐานของ supply chain security ทั้งหมด

---

## D4.7 dive

```bash
docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock \
  wagoodman/dive devops-todo-api:local
```

ดูค่า **Wasted Space** — คือไฟล์ที่ถูกเพิ่มใน layer หนึ่งแล้วถูกลบ/ทับใน layer ถัดไป แต่ยังกินที่อยู่

ตัวอย่างที่มักเจอ: npm cache ที่ถูกลบคนละ `RUN` กับตอนสร้าง

รันใน CI แบบไม่ interactive:

```bash
CI=true dive devops-todo-api:local --lowestEfficiency=0.9
```

---

## D4.8 gate อัตโนมัติ

```bash
#!/usr/bin/env bash
set -euo pipefail
IMAGE=$1
fail() { echo "❌ $1"; exit 1; }

# 1) ต้องไม่รันเป็น root
[ "$(docker inspect -f '{{.Config.User}}' "$IMAGE")" != "" ] || fail "รันเป็น root"

# 2) ห้ามใช้ latest ใน FROM
grep -qE '^FROM .*:latest' Dockerfile && fail "ใช้ tag latest"

# 3) ขนาดต้องไม่เกิน 300MB
SIZE=$(docker inspect -f '{{.Size}}' "$IMAGE")
[ "$SIZE" -lt 314572800 ] || fail "image ใหญ่เกิน 300MB ($((SIZE/1024/1024))MB)"

# 4) ห้ามมี HIGH/CRITICAL
trivy image --exit-code 1 --severity HIGH,CRITICAL "$IMAGE" || fail "มีช่องโหว่ระดับสูง"

echo "✅ ผ่านทุกเกณฑ์"
```

**ทำไมต้องเป็นสคริปต์ ไม่ใช่ข้อตกลงในเอกสาร:** กฎที่ไม่มีระบบบังคับ = กฎที่จะถูกลืมภายในเดือนแรก
`set -euo pipefail` สำคัญมาก — ถ้าไม่ใส่ คำสั่งที่พังจะถูกข้ามไปเงียบ ๆ แล้วสคริปต์ขึ้นเขียวหลอก

---

## 🎯 ต่อยอด

- เทียบขนาด/CVE ระหว่าง alpine / slim / distroless แล้วทำตารางสรุปของตัวเอง
- ลอง `docker buildx bake` เพื่อ build หลาย target พร้อมกันจากไฟล์เดียว
- วัดว่า cache mount ประหยัดเวลาได้จริงกี่ % ใน CI ของคุณ
