# 09 — แบบฝึกหัด

ทำไล่ระดับ แต่ละข้อมีคำใบ้ให้ ไม่ต้องรีบดู

## ระดับ 1 — Docker

**1.1 ทำ image ให้เล็กลง**
เช็คขนาดด้วย `docker images` แล้ว `docker history` ดูว่า layer ไหนกินที่
> ใบ้: `npm ci --omit=dev` แล้ว `npm cache clean --force` ใน layer เดียวกัน / ลองเทียบ `node:22-alpine` กับ `node:22-slim`

**1.2 พิสูจน์ layer cache**
สลับให้ `COPY src ./src` มาก่อน `npm ci` แล้ว build สองครั้ง จับเวลาเทียบกับของเดิม

**1.3 build หลาย architecture**
ทำให้ image รันได้ทั้ง amd64 และ arm64
> ใบ้: `docker buildx build --platform linux/amd64,linux/arm64` และใน workflow ใส่ `platforms:` ให้ `build-push-action`

## ระดับ 2 — Compose & Nginx

**2.1 เพิ่ม Adminer**
เพิ่ม service `adminer` (image `adminer`) ที่ port 8081 ไว้ดู DB ผ่านเว็บ

**2.2 ทำ path routing**
ให้ `/admin` ใน nginx proxy ไปที่ adminer ส่วน `/api` ยังไปที่ api เหมือนเดิม
> ใบ้: เพิ่ม `location /admin/` + `upstream` ใหม่ อาจต้องใช้ `rewrite`

**2.3 เปิด HTTPS ด้วย self-signed cert**
```bash
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout nginx/certs/key.pem -out nginx/certs/cert.pem -subj "/CN=localhost"
```
แล้วเพิ่ม `server { listen 443 ssl; ... }` + redirect 80 → 443

**2.4 จูน rate limit**
ลองเทียบ `burst=5` กับ `burst=50 nodelay` ว่าพฤติกรรมต่างกันยังไง แล้วทำ zone แยกให้ POST เข้มกว่า GET

## ระดับ 3 — CI

**3.1 เพิ่ม test จริง** ⭐ แนะนำให้ทำข้อนี้
```bash
npm i -D jest ts-jest @types/jest supertest @types/supertest
```
เขียน `src/__tests__/todos.test.ts` ยิง `createApp()` ผ่าน supertest แล้วเรียกใน CI แทน smoke test แบบ curl
> ใบ้: export `createApp()` แยกจาก `listen()` ไว้แล้วใน `src/app.ts` เพื่อการนี้โดยเฉพาะ

**3.2 เพิ่ม ESLint + Prettier**
เพิ่ม job `lint` ที่รันขนานกับ `build-and-test`

**3.3 สแกนช่องโหว่**
เพิ่ม `npm audit --audit-level=high` และ Trivy สแกน image
```yaml
- uses: aquasecurity/trivy-action@master
  with:
    image-ref: devops-todo-api:test
    severity: HIGH,CRITICAL
    exit-code: '1'
```

**3.4 Matrix build**
รัน test บน Node 20 และ 22 พร้อมกัน
> ใบ้: `strategy: { matrix: { node: [20, 22] } }` แล้วใช้ `${{ matrix.node }}`

**3.5 อัปโหลด coverage เป็น artifact**
> ใบ้: `actions/upload-artifact@v4`

## ระดับ 4 — CD

**4.1 ทำ semantic version**
push tag `v1.0.0` แล้วดูว่า `metadata-action` สร้าง tag `1.0.0`, `1.0`, `latest` ให้ครบไหม

**4.2 แยก environment staging**
สร้าง overlay `k8s/overlays/staging` + job deploy แยก โดยให้ `main` → staging และ tag `v*` → production

**4.3 ใส่ approval gate**
Settings → Environments → production → Required reviewers แล้วลอง merge ดู

**4.4 แจ้งเตือนเข้า Discord/Slack**
เพิ่ม step ที่ส่ง webhook เมื่อ deploy สำเร็จ/ล้มเหลว (ใช้ `if: success()` / `if: failure()`)

**4.5 ทดสอบ rollback อัตโนมัติ**
จงใจ deploy image พัง (เช่นแก้ CMD ให้ผิด) แล้วดูว่า `rollout status --timeout` จับได้และ `rollout undo` ทำงานไหม

## ระดับ 5 — Kubernetes

**5.1 ทำให้ pod พังแล้วดูว่าเกิดอะไร**
```bash
kubectl -n todo-app exec -it <pod> -- kill 1
kubectl -n todo-app get pods -w
kubectl -n todo-app describe pod <pod>   # ดู RESTARTS และ Events
```

**5.2 ทดสอบ zero-downtime**
ยิง request ต่อเนื่องระหว่าง rolling update แล้วนับว่าพลาดกี่ตัว
```bash
while true; do curl -s -o /dev/null -w "%{http_code}\n" http://todo.local/healthz; sleep 0.2; done
```
> ถ้าเห็น 502/503 แปลว่า probe หรือ graceful shutdown ยังไม่ดีพอ

**5.3 ทดสอบ HPA**
ยิงโหลดจนเห็น pod เพิ่ม แล้วหยุดโหลด จับเวลาว่ากี่นาที pod ถึงลด (เฉลย: ~2 นาที ตาม `stabilizationWindowSeconds`)

**5.4 เขียน Helm chart**
แปลง `k8s/base` เป็น Helm chart แล้วเทียบกับ kustomize ว่าชอบแบบไหน

**5.5 NetworkPolicy**
ทำให้เฉพาะ pod ที่ label `app: todo-api` เท่านั้นที่ต่อ postgres ได้
> ใบ้: ต้องมี CNI ที่รองรับ เช่น Calico (minikube: `--cni=calico`)

**5.6 ลอง GitOps ด้วย ArgoCD**
ให้ ArgoCD sync จาก `k8s/overlays/production` แทนการ `kubectl apply` ใน workflow
นี่คือทิศทางที่ทีมส่วนใหญ่ไปกัน — CD ไม่ push เข้าคลัสเตอร์ แต่คลัสเตอร์ pull เอง

## ระดับ 6 — Observability (หัวข้อถัดไปที่ควรฝึก)

**6.1** เพิ่ม `/metrics` ด้วย `prom-client` แล้วต่อ Prometheus + Grafana
**6.2** เปลี่ยน `console.log` เป็น structured logging ด้วย `pino` แล้วส่งเข้า Loki
**6.3** ใส่ OpenTelemetry ทำ distributed tracing

➡️ เจอปัญหาระหว่างทาง: [10 — Troubleshooting](10-troubleshooting.md)
