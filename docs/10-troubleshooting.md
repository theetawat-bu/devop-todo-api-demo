# 10 — แก้ปัญหาที่เจอบ่อย

## Node / Prisma

**`@prisma/client did not initialize yet`**
ยังไม่ได้ generate → `npx prisma generate`
ถ้าเจอใน Docker แปลว่า Dockerfile ไม่ได้ copy `node_modules/.prisma` มาจาก stage builder

**`Can't reach database server at localhost:5432`**
ต่อ DB ผิด host — ใน container `localhost` คือตัว container เอง
ใช้ `db` (compose) หรือ `postgres` (k8s) เป็น hostname

**`DATABASE_URL is not set`**
ยังไม่มี `.env` → `cp .env.example .env`
บน k8s แปลว่า Secret ไม่ถูก mount ตรวจ `kubectl -n todo-app describe pod <pod>`

**`P3009: migrate found failed migrations`**
migration เก่าค้างสถานะ failed → `npx prisma migrate resolve --rolled-back <ชื่อ migration>`

**ติดตั้งบนเครื่องแล้วรันไม่ได้ / binary ผิด platform**
`rm -rf node_modules package-lock.json && npm install` (เกิดตอนย้าย `node_modules` ข้าม OS)

## Docker

**build ช้าทุกครั้ง ไม่ยอม cache**
เรียงคำสั่งผิด — `COPY package*.json` ต้องมาก่อน `COPY src` ดู [03](03-docker.md)
เช็คว่ามี `.dockerignore` กัน `node_modules` ไว้แล้ว

**`port is already allocated`**
```bash
lsof -i :8080          # หาว่าใครใช้อยู่
docker compose down
```

**container ขึ้นแล้วดับทันที**
```bash
docker compose logs api          # อ่าน error
docker compose ps -a             # ดู exit code
```
exit 1 = แอปโยน error เอง, exit 137 = โดนฆ่าเพราะ memory เต็ม

**แก้ไฟล์แล้วไม่มีอะไรเปลี่ยน**
image ยังเป็นตัวเก่า → `docker compose up -d --build`
ถ้าอยากล้างจริง ๆ: `docker compose build --no-cache`

**`permission denied` ในไฟล์ที่ mount**
เกิดจาก `USER node` (uid 1000) ไม่มีสิทธิ์ในไฟล์ของ host — dev override ใช้ stage `builder` ที่ยังเป็น root อยู่จึงไม่เจอ

## Compose

**api ขึ้นก่อน db พร้อม**
ต้องมี `condition: service_healthy` ไม่ใช่แค่ `depends_on: [db]`

**ข้อมูล DB หายหลัง restart**
เผลอ `docker compose down -v` (`-v` ลบ volume) หรือลืมประกาศ volume

**เปลี่ยนค่าใน `.env` แล้วไม่มีผล**
ต้อง recreate container: `docker compose up -d --force-recreate`
เช็คค่าที่ถูกแทนจริงด้วย `docker compose config`

## Nginx

**502 Bad Gateway**
nginx ต่อ upstream ไม่ได้ — เช็คว่า api ขึ้นจริงไหม (`docker compose ps`) และชื่อใน `upstream` ตรงกับชื่อ service

**504 Gateway Timeout**
แอปตอบช้าเกิน `proxy_read_timeout` → เพิ่มเวลา หรือไปแก้ที่แอปให้เร็วขึ้น

**เจอ 429 ตลอด**
rate limit เข้มเกิน → เพิ่ม `rate` หรือ `burst` ใน `nginx.conf`

**แก้ config แล้วไม่มีผล**
```bash
docker compose exec nginx nginx -t     # ตรวจ syntax ก่อนเสมอ
docker compose restart nginx
```
mount เป็น `:ro` ต้อง restart container ไม่ใช่แค่เซฟไฟล์

**แอปเห็น IP เป็น 172.x ทุก request**
ขาด `proxy_set_header X-Forwarded-For` หรือฝั่ง Express ไม่ได้ตั้ง `app.set('trust proxy', true)`

## GitHub Actions

**`Error: Cannot find module` ใน CI แต่ในเครื่องปกติ**
ลืม commit `package-lock.json` หรือ dependency ไปอยู่ใน devDependencies

**`denied: permission_denied` ตอน push GHCR**
- ยังไม่ได้ใส่ `permissions: packages: write`
- Settings → Actions → General → Workflow permissions ยังเป็น read-only
- ชื่อ image ต้อง **ตัวพิมพ์เล็กทั้งหมด**

**workflow ไม่รันเลย**
- ไฟล์ต้องอยู่ที่ `.github/workflows/*.yml` เป๊ะ ๆ
- branch ใน `on:` ตรงกับ branch จริงไหม (`main` vs `master`)
- YAML ผิด indent → GitHub จะขึ้น error ที่แท็บ Actions

**cache ไม่ทำงาน**
`actions/setup-node` ต้องมี `cache: 'npm'` และต้องมี lockfile อยู่จริง

**secret เป็นค่าว่าง**
secret ไม่ถูกส่งให้ workflow ที่มาจาก fork PR — เป็นพฤติกรรมด้านความปลอดภัยที่ตั้งใจ

## Kubernetes

> เริ่มจาก `kubectl -n todo-app describe pod <pod>` แล้วอ่าน **Events** ท้ายสุดเสมอ

| อาการ | สาเหตุที่พบบ่อย | ตรวจยังไง |
|---|---|---|
| `ImagePullBackOff` | ชื่อ image ผิด / private ไม่มี imagePullSecret | `describe pod` → Events |
| `CrashLoopBackOff` | แอปตายซ้ำ ๆ ตอน boot | `logs <pod> --previous` |
| `Pending` | ทรัพยากรไม่พอ / ไม่มี PV ให้ผูก | `describe pod` → Events |
| `OOMKilled` | ใช้ memory เกิน `limits` | `describe pod` → Last State |
| `0/1 Running` ไม่ ready สักที | readinessProbe ไม่ผ่าน | `logs` + ลอง curl endpoint ใน pod |
| `Init:0/1` ค้าง | initContainer (migration) พัง | `logs <pod> -c migrate` |

**ดึง image จาก GHCR ที่เป็น private ไม่ได้**
```bash
kubectl -n todo-app create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io --docker-username=<user> --docker-password=<PAT>
```
แล้วเพิ่ม `imagePullSecrets: [{ name: ghcr-secret }]` ใน pod spec
(หรือง่ายกว่า: ตั้ง package ให้เป็น public ที่หน้า Packages)

**ingress ยิงไม่ได้ 404**
- ติดตั้ง controller แล้วหรือยัง: `kubectl get pods -n ingress-nginx`
- `ingressClassName: nginx` ตรงกับ controller ไหม
- ใส่ `/etc/hosts` ชี้ `todo.local` ไป IP ของ ingress แล้วหรือยัง

**HPA ขึ้น `<unknown>` ในคอลัมน์ TARGETS**
ไม่มี metrics-server → `minikube addons enable metrics-server` แล้วรอสักครู่
หรือ Deployment ไม่ได้ตั้ง `resources.requests` (HPA คำนวณจากค่านี้)

**`kubectl apply -k` ฟ้อง field ไม่รู้จัก**
เวอร์ชัน kubectl เก่าเกิน — `kubectl version` แล้วอัปเดต

**PVC ค้างที่ Pending**
คลัสเตอร์ไม่มี default StorageClass → `kubectl get storageclass`

## เทคนิคดีบักทั่วไป

1. **อ่าน error ให้จบ** — บรรทัดสุดท้ายมักบอกสาเหตุจริง
2. **แยกให้แคบลง** — พังที่แอป, ที่ network, หรือที่ config? ลองยิงจากในสุดออกมาทีละชั้น
3. **เทียบกับที่ที่มันเวิร์ก** — รันในเครื่องได้แต่ใน docker ไม่ได้ = ปัญหาอยู่ที่ environment ไม่ใช่โค้ด
4. **เปลี่ยนทีละอย่าง** — เปลี่ยนสามที่พร้อมกันแล้วหาย จะไม่รู้ว่าอะไรแก้
5. **`describe` / `logs` / `exec` คือเพื่อนที่ดีที่สุด**
