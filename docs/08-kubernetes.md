# 08 — Kubernetes

## เตรียมคลัสเตอร์ในเครื่อง

เลือกอย่างใดอย่างหนึ่ง:

```bash
# minikube
brew install minikube
minikube start --cpus=2 --memory=4096
minikube addons enable ingress
minikube addons enable metrics-server

# หรือ kind
brew install kind
kind create cluster --name todo

# หรือง่ายสุด: Docker Desktop → Settings → Kubernetes → Enable
```

## Object ที่ต้องรู้จัก

| Object          | หน้าที่                                                 | ไฟล์                  |
| --------------- | ------------------------------------------------------- | --------------------- |
| **Namespace**   | แบ่งห้องให้ทรัพยากร                                     | `base/namespace.yaml` |
| **ConfigMap**   | ค่าตั้งค่าที่ไม่ลับ                                     | `base/configmap.yaml` |
| **Secret**      | ค่าลับ (base64 ไม่ใช่การเข้ารหัส!)                      | `base/secret.yaml`    |
| **Deployment**  | ดูแล pod ที่ไม่มี state — scale/rolling update/rollback | `base/api.yaml`       |
| **StatefulSet** | ดูแล pod ที่มี state — ชื่อคงที่ + ดิสก์ส่วนตัว         | `base/postgres.yaml`  |
| **Service**     | ชื่อ + IP คงที่หน้ากลุ่ม pod, load balance ให้          | `base/api.yaml`       |
| **Ingress**     | ประตูจากภายนอกเข้าคลัสเตอร์ (L7)                        | `base/ingress.yaml`   |
| **HPA**         | เพิ่ม/ลด pod ตามโหลด                                    | `base/hpa.yaml`       |
| **PDB**         | กันไม่ให้ pod ถูกไล่ออกพร้อมกันหมด                      | `base/api.yaml`       |

**Deployment vs StatefulSet:** DB ต้องใช้ StatefulSet เพราะแต่ละ pod ต้องมีดิสก์ของตัวเองและชื่อคงที่
API ไม่มี state → Deployment พอ (สร้าง/ฆ่าสลับกันได้อิสระ)

## Deploy

```bash
kubectl apply -k k8s/overlays/dev

kubectl -n todo-app get all
kubectl -n todo-app get pods -w         # ดู pod ค่อย ๆ พร้อม
```

เข้าใช้งาน:

```bash
# วิธีที่ 1: port-forward (ง่ายสุด ข้าม ingress)
kubectl -n todo-app port-forward svc/todo-api 8080:80
curl localhost:8080/healthz

# วิธีที่ 2: ผ่าน ingress
echo "$(minikube ip) todo.local" | sudo tee -a /etc/hosts
curl http://todo.local/api/todos
```

## เจาะ manifest ที่สำคัญ

### Probes — สามตัว สามหน้าที่

```yaml
livenessProbe: { httpGet: { path: /healthz, port: http }, periodSeconds: 20 }
readinessProbe: { httpGet: { path: /readyz, port: http }, periodSeconds: 10 }
startupProbe:
  {
    httpGet: { path: /healthz, port: http },
    periodSeconds: 5,
    failureThreshold: 12,
  }
```

| Probe         | ไม่ผ่านแล้วเกิดอะไร                                     | ใช้เช็คอะไร                                                        | **ห้าม**เช็คอะไร                                                        |
| ------------- | ------------------------------------------------------- | ------------------------------------------------------------------ | ----------------------------------------------------------------------- |
| **liveness**  | **ฆ่า container แล้วสร้างใหม่** (restart counter +1)    | แค่ว่า event loop ยังตอบได้ ไม่ deadlock                           | DB, Redis, service อื่น — ทุกอย่างที่ restart แล้วไม่ช่วย               |
| **readiness** | **ถอด pod ออกจาก Service** ไม่ฆ่า พอผ่านก็กลับเข้ามาเอง | dependency ที่จำเป็นต่อการรับงาน (DB, cache, warm-up เสร็จหรือยัง) | สิ่งที่ล่มแล้วแอปยังทำงานส่วนใหญ่ได้ (เช่น service เสริมที่มี fallback) |
| **startup**   | รอจนผ่าน แล้วค่อยเริ่มนับ liveness/readiness            | แอปที่ boot ช้า (JVM, โหลด model, migration)                       | —                                                                       |

### ตัดสินใจว่าจะใช้ probe ตัวไหน

| ถ้าอาการคือ...                      | ควรทำ                                                  | เหตุผล                                                                      |
| ----------------------------------- | ------------------------------------------------------ | --------------------------------------------------------------------------- |
| DB ล่มชั่วคราว                      | **readiness เท่านั้น**                                 | restart pod ไม่ทำให้ DB กลับมา แค่ถอดออกจาก pool แล้วรอ                     |
| แอป deadlock / memory leak จนค้าง   | **liveness**                                           | restart คือทางแก้จริง                                                       |
| แอป boot 60 วินาที                  | **startup** (`failureThreshold × periodSeconds ≥ 60s`) | ดีกว่าใส่ `initialDelaySeconds` สูง ๆ ที่ทำให้ตรวจเจอปัญหาช้าลงตลอดอายุ pod |
| กำลังโหลด cache ตอนเริ่มต้น         | **readiness**                                          | อย่าเพิ่งรับ traffic จนกว่าจะพร้อม                                          |
| ต้องหยุดรับงานชั่วคราวเพื่อระบายคิว | **readiness** ที่ควบคุมจากในแอปได้                     | ถอดตัวเองออกจาก LB ได้โดยไม่ต้องตาย                                         |
| แค่ "แอปพังบ่อยแก้ไม่ได้"           | liveness **แต่รู้ว่านี่คือพลาสเตอร์ปิดแผล**            | restart อัตโนมัติซื้อเวลา แต่ไม่ใช่การแก้                                   |

**กฎที่จำง่ายที่สุด:** ถ้า restart แล้วไม่ช่วย → มันไม่ใช่งานของ liveness

**anti-pattern ที่เจอบ่อยที่สุด:** ใช้ endpoint เดียวกันทั้ง liveness และ readiness แล้ว endpoint นั้นเช็ค DB
ผลคือวันที่ DB ช้า → readiness ไม่ผ่าน (ถูกต้อง) **แต่ liveness ก็ไม่ผ่านด้วย** → k8s ฆ่า pod ทั้งหมดพร้อมกัน →
pod ใหม่รุมต่อ DB ที่ช้าอยู่แล้ว → ช้ากว่าเดิม → ฆ่าอีกรอบ = **restart storm** ที่เกิดจาก probe ล้วน ๆ
นี่คือเหตุผลที่โปรเจกต์นี้แยก `/healthz` (ไม่แตะ DB) ออกจาก `/readyz` (แตะ DB) ตั้งแต่แรก

### เลือก probe handler แบบไหน

```yaml
httpGet: { path: /healthz, port: http } # ← ใช้ตัวนี้ถ้าเป็น HTTP service
exec: { command: ["sh", "-c", "pg_isready"] }
tcpSocket: { port: 5432 }
grpc: { port: 50051 }
```

| Handler       | ค่าใช้จ่าย                                | ความแม่นยำ                                 | ใช้เมื่อ                                                         |
| ------------- | ----------------------------------------- | ------------------------------------------ | ---------------------------------------------------------------- |
| **httpGet**   | ต่ำ (kubelet ยิงเอง ไม่ต้อง fork process) | สูง — ให้แอปตอบเองว่าพร้อมจริงไหม          | **HTTP service ทุกตัว = ตัวเลือกแรกเสมอ**                        |
| **grpc**      | ต่ำ                                       | สูง                                        | gRPC service (k8s 1.24+)                                         |
| **tcpSocket** | ต่ำมาก                                    | **ต่ำ** — port เปิดอยู่ ≠ แอปทำงานได้      | service ที่ไม่ใช่ HTTP เช่น DB, message queue                    |
| **exec**      | **สูงสุด** — fork process ใหม่ทุกครั้ง    | สูง แต่ถ้ายิงถี่จะกินทรัพยากรจนกระทบแอปเอง | ไม่มีทางอื่นจริง ๆ เช่น `pg_isready` ใน StatefulSet ของ postgres |

`tcpSocket` เป็นกับดัก — แอป Node ที่ event loop ค้างสนิทก็ยังทำให้ port เปิดอยู่ได้ probe เลยผ่านทั้งที่ตอบ request ไม่ได้แล้ว

### จูนตัวเลขยังไงให้ไม่พัง

```yaml
readinessProbe:
  periodSeconds: 10 # ยิงทุก 10 วิ
  timeoutSeconds: 3 # ต้องน้อยกว่า periodSeconds เสมอ
  failureThreshold: 3 # พลาด 3 ครั้งติดถึงจะถือว่าไม่พร้อม
  successThreshold: 1
```

**เวลาที่ระบบทนได้ก่อนลงมือ = `periodSeconds × failureThreshold`**
ค่าข้างบน = 30 วินาที เหมาะกับ readiness (ยอมให้สะดุดสั้น ๆ ได้ ไม่ต้องรีบถอด)

- ตั้งถี่/ไวเกิน → pod เด้งเข้าออก pool ตลอด (flapping) ตอนโหลดสูง
- ตั้งช้าเกิน → ผู้ใช้เจอ error หลายสิบวินาทีกว่า k8s จะรู้ตัว
- `timeoutSeconds` ต้อง **น้อยกว่า** `periodSeconds` ไม่งั้น probe ซ้อนกันเอง
- liveness ควรตั้ง "ใจเย็นกว่า" readiness เสมอ (period ยาวกว่า, threshold สูงกว่า) เพราะโทษหนักกว่ามาก

**ข้อควรระวังสำคัญ:** อย่าให้ probe ไปเรียก service อื่นต่อเป็นทอด ๆ
ถ้า `/readyz` ของ A เรียก `/readyz` ของ B ที่เรียกของ C — วันที่ C ล่ม ทั้งสามระบบจะ not-ready พร้อมกันทันที (cascading failure)
ให้เช็คเฉพาะ dependency ที่ **ตัวเองต้องใช้โดยตรง** เท่านั้น

### initContainer สำหรับ migration

```yaml
initContainers:
  - name: migrate
    image: <same image>
    command: ["sh", "-c", "migrate -path ./migrations -database \"$DATABASE_URL\" up"]
```

initContainer รันจนจบก่อน container หลักจะเริ่ม → migration รันครั้งเดียวก่อน ไม่ใช่รันพร้อมกันทุก pod
(ถ้าใส่ไว้ใน CMD ของแอป pod 3 ตัวจะแย่งกันรัน migration พร้อมกัน)

ทางเลือกที่ดีกว่าสำหรับ production คือแยกเป็น `Job` หรือ Helm hook

### Resources — ต้องใส่เสมอ

```yaml
resources:
  requests: { cpu: 100m, memory: 128Mi } # จองขั้นต่ำ — scheduler ใช้เลือก node
  limits: { cpu: 500m, memory: 256Mi } # เพดาน — เกิน memory = OOMKilled
```

- ไม่ใส่ `requests` → scheduler วาง pod มั่ว node ล้นได้
- ไม่ใส่ `limits` → pod เดียวกินหมดทั้ง node
- `100m` = 0.1 core
- CPU เกิน limit จะถูก throttle (ช้าลง) แต่ **memory เกิน limit จะถูกฆ่าเลย (OOMKilled)**

HPA คำนวณจาก `requests` — ถ้าไม่ใส่ HPA ทำงานไม่ได้

### Rolling update แบบไม่มี downtime

```yaml
strategy:
  rollingUpdate: { maxSurge: 1, maxUnavailable: 0 }
terminationGracePeriodSeconds: 30
```

`maxUnavailable: 0` = pod ใหม่ต้อง ready ก่อน ถึงจะฆ่าตัวเก่า
ฝั่งแอปต้องรองรับด้วย — ดู `cmd/api/main.go` ที่ดัก `SIGTERM` แล้วปิด server อย่างสุภาพ ไม่ตัด request ที่ค้างอยู่

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
sig := <-quit
// ... srv.Shutdown(shutdownCtx) รอ request ที่ค้างอยู่ให้จบก่อนค่อยปิดจริง
```

ถ้าแอปไม่ดัก SIGTERM → k8s รอ 30 วิแล้ว SIGKILL → request ที่กำลังทำอยู่ขาดกลางคัน

### Security context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  capabilities: { drop: ["ALL"] }
```

ทำงานคู่กับ user ที่ไม่ใช่ root ที่ตั้งไว้ใน Dockerfile (ดู [03 — ทำไมต้องสร้าง user เอง](03-docker.md)) ถ้า image รันเป็น root อยู่ pod จะสตาร์ทไม่ขึ้นเลย

### HPA

```yaml
minReplicas: 2
maxReplicas: 6
metrics:
  - type: Resource
    resource:
      { name: cpu, target: { type: Utilization, averageUtilization: 70 } }
behavior:
  scaleDown: { stabilizationWindowSeconds: 120 }
```

`stabilizationWindowSeconds` กัน **flapping** (ขึ้น ๆ ลง ๆ ถี่เกินไป) — ขาลงรอให้แน่ใจ 2 นาทีก่อน

ทดลอง:

```bash
kubectl -n todo-app get hpa -w
# อีก terminal ยิงโหลด
kubectl -n todo-app run load --rm -it --image=busybox --restart=Never -- \
  sh -c "while true; do wget -q -O- http://todo-api/api/todos; done"
```

## Load balancing บน Kubernetes — ทำที่ชั้นไหนดี

บน k8s มีจุดที่กระจาย traffic ได้หลายชั้น ซ้อนกันอยู่จริง ๆ:

```
Internet
   │
   ▼  ① Cloud LB (L4)        ← ของ cloud provider, กระจายไป node
┌──────────────────┐
│ Ingress          │  ② L7   ← path/host routing, sticky cookie, TLS
└──────────────────┘
   │
   ▼  ③ Service (kube-proxy, L4)  ← กระจายไป pod
┌──────────────────┐
│ Pods             │  ④ Service Mesh sidecar (L7) ← ถ้าติดตั้ง
└──────────────────┘
```

| ชั้น                             | ทำงานที่ระดับ | กระจายต่ออะไร      | ทำอะไรได้                                                     | ไม่เหมาะกับ                                                      |
| -------------------------------- | ------------- | ------------------ | ------------------------------------------------------------- | ---------------------------------------------------------------- |
| **Service (ClusterIP)**          | L4 (TCP)      | ต่อ **connection** | round-robin/random แบบง่าย, ฟรี, มาพร้อม k8s                  | HTTP/2, gRPC, connection ที่ keep-alive ยาว                      |
| **Ingress**                      | L7 (HTTP)     | ต่อ **request**    | path/host routing, sticky cookie, rewrite, TLS, rate limit    | traffic ภายในระหว่าง service (ปกติใช้กับ traffic ขาเข้าเท่านั้น) |
| **Service Mesh** (Istio/Linkerd) | L7            | ต่อ **request**    | retry, circuit breaker, outlier detection, mTLS, canary ตาม % | ทีมเล็ก — ค่าดูแลสูงและ debug ยากขึ้นมาก                         |
| **Headless Service + client LB** | ในแอป         | ต่อ request        | ควบคุมเองได้เต็มที่                                           | ต้องเขียนโค้ดเพิ่ม ผูกกับภาษา                                    |

### ⚠️ กับดักที่ทุกคนต้องเจอสักครั้ง: Service กระจาย gRPC ไม่เป็น

`kube-proxy` ทำงานที่ **L4** — มันเลือก pod ตอน **เปิด connection** แล้วจบ ทุก request หลังจากนั้นวิ่งไป pod เดิมตลอด

- **REST ธรรมดา** → มักเปิด connection ใหม่บ่อย ๆ ก็เลยดูเหมือนกระจายดี
- **gRPC / HTTP2 / connection pool ที่ keep-alive ยาว** → เปิด connection ครั้งเดียวใช้ยาว = **traffic ทั้งหมดกระจุกที่ pod เดียว** ต่อให้มี 10 pod ก็ตาม

อาการ: scale pod เพิ่มแล้วประสิทธิภาพไม่ดีขึ้นเลย, `kubectl top pods` เห็น pod เดียว CPU พุ่งที่เหลือว่าง

ทางแก้เรียงตามความง่าย:

1. ให้ client ปิด connection เป็นระยะ (ตั้ง max connection age)
2. ใช้ **Ingress / proxy L7** คั่น เพราะมันกระจายต่อ request
3. ใช้ **Service Mesh** — ตรงเป้าที่สุดสำหรับ traffic ภายในที่เป็น gRPC
4. **Headless Service** (`clusterIP: None`) แล้วให้ client ทำ LB เอง (gRPC มี `round_robin` policy ในตัว)

### ตัวเลือกอื่นที่ควรรู้จัก

```yaml
# sticky session ระดับ L4 — IP เดิมไป pod เดิม
spec:
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP: { timeoutSeconds: 10800 }
```

| ตัวเลือก                                       | ผลที่ได้                                                             | เมื่อไรควรใช้                                                              |
| ---------------------------------------------- | -------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| `sessionAffinity: ClientIP`                    | sticky ตาม IP (แบบเดียวกับ `ip_hash` ของ nginx)                      | แอปเก่าที่เก็บ session ในเครื่อง — ทางที่ดีกว่าคือย้าย session ออกไป Redis |
| Ingress cookie affinity                        | sticky ตาม cookie แม่นกว่า IP                                        | ต้องการ sticky จริง ๆ บน HTTP                                              |
| `externalTrafficPolicy: Local`                 | เห็น source IP จริง แต่กระจายไม่สม่ำเสมอ (node ที่ไม่มี pod จะ drop) | ต้องใช้ IP จริงในการ log/limit                                             |
| `externalTrafficPolicy: Cluster` (ค่าเริ่มต้น) | กระจายสม่ำเสมอ แต่ source IP ถูก SNAT ทับ                            | เคสทั่วไป                                                                  |
| `topologyAwareHints`                           | ให้ traffic วิ่งใน zone เดียวกันก่อน                                 | คลัสเตอร์หลาย AZ ที่อยากลดค่า network ข้าม zone                            |

**สรุปสำหรับโปรเจกต์นี้:** REST API ธรรมดา + traffic ขาเข้าจากภายนอก → **Ingress + Service ก็พอ ไม่ต้องมี mesh**
mesh คุ้มค่าเมื่อมี service หลายสิบตัวคุยกันเอง และต้องการ mTLS / tracing / canary เป็นมาตรฐานทั้งองค์กร

## Rate limit บน Kubernetes

```yaml
annotations:
  nginx.ingress.kubernetes.io/limit-rps: "10"
  nginx.ingress.kubernetes.io/limit-burst-multiplier: "3"
  nginx.ingress.kubernetes.io/limit-connections: "20"
```

**สิ่งที่ต้องรู้ก่อนใช้:** ค่านี้เป็น **ต่อ pod ของ ingress controller** ไม่ใช่ทั้งคลัสเตอร์
ingress-nginx 3 replica + `limit-rps: 10` → ผู้ใช้คนเดียวยิงได้จริงถึง 30 rps
และถ้า controller มี HPA ของตัวเอง limit จริงก็จะขยับตามโดยไม่มีใครรู้

| วิธี                      | นับรวมทั้งคลัสเตอร์ | แยกตาม user/API key | ค่าดูแล | เหมาะกับ                              |
| ------------------------- | ------------------- | ------------------- | ------- | ------------------------------------- |
| Ingress annotation        | ❌ ต่อ replica      | ❌                  | ต่ำมาก  | กัน bot / traffic ขยะแบบหยาบ ๆ        |
| Istio + Envoy global RLS  | ✅ (มี Redis กลาง)  | ✅                  | สูง     | องค์กรที่มี mesh อยู่แล้ว             |
| API Gateway (Kong/APISIX) | ✅                  | ✅                  | กลาง    | ขาย API ให้ลูกค้าภายนอก มี plan/quota |
| ในแอป + Redis             | ✅                  | ✅ ละเอียดสุด       | ต่ำ     | quota ตาม business rule               |
| CDN/WAF (Cloudflare)      | ✅                  | ⚠️                  | ต่ำ     | กัน DDoS ก่อนถึงคลัสเตอร์             |

**คำแนะนำ:** ตั้งที่ Ingress หลวม ๆ เพื่อกันของหยาบ + ตั้งใน quota จริงที่แอป — อย่าพยายามให้ Ingress ทำหน้าที่ quota แม่น ๆ เพราะมันทำไม่ได้โดยธรรมชาติ
(ดูตารางเต็มข้ามทุกชั้นที่ [09 — ตั้งค่าที่ชั้นไหนดี](09-where-to-configure.md))

## Kustomize — base + overlays

```
k8s/
├── base/                    ← manifest กลาง
└── overlays/
    ├── dev/                 ← replicas 1
    └── production/          ← replicas 3, host จริง
```

ไม่ต้องก็อป YAML ทั้งชุดต่อ environment แต่เขียนเฉพาะ "ส่วนต่าง"

```bash
kubectl kustomize k8s/overlays/production   # ดูผลลัพธ์ก่อน apply
kubectl apply -k k8s/overlays/production
kubectl diff -k k8s/overlays/production     # เทียบกับของที่รันอยู่จริง
```

`kubectl diff` ก่อน apply เป็นนิสัยที่ดีมาก

## Rollback

```bash
kubectl -n todo-app rollout history deployment/todo-api
kubectl -n todo-app rollout undo deployment/todo-api            # ถอยหนึ่งเวอร์ชัน
kubectl -n todo-app rollout undo deployment/todo-api --to-revision=3
kubectl -n todo-app rollout status deployment/todo-api
```

`revisionHistoryLimit: 5` ใน manifest = เก็บประวัติไว้ 5 เวอร์ชันให้ย้อนได้

## Secret — ต้องรู้ก่อนใช้จริง

`base/secret.yaml` ที่ commit ไว้เป็น **ตัวอย่างสำหรับฝึกเท่านั้น**
Secret ของ k8s เก็บเป็น base64 ซึ่ง **ถอดกลับได้ทันที ไม่ใช่การเข้ารหัส**

```bash
kubectl -n todo-app get secret todo-secret -o jsonpath='{.data.DATABASE_URL}' | base64 -d
```

ของจริงเลือกทางใดทางหนึ่ง:

- สร้างด้วยมือ ไม่ commit: `kubectl create secret generic ... --from-literal=...`
- **Sealed Secrets** — เข้ารหัสแล้ว commit ลง git ได้ ถอดได้เฉพาะในคลัสเตอร์
- **External Secrets Operator** — ดึงจาก Vault / AWS Secrets Manager
- **SOPS** + age/KMS

## คำสั่งดีบักที่ใช้ตลอดชีวิต

```bash
kubectl -n todo-app get pods
kubectl -n todo-app describe pod <pod>        # ⭐ ดู Events ท้ายสุด — บอกสาเหตุเกือบทุกครั้ง
kubectl -n todo-app logs <pod>
kubectl -n todo-app logs <pod> -c migrate     # log ของ initContainer
kubectl -n todo-app logs <pod> --previous     # log ของ container ที่ตายไปแล้ว ⭐
kubectl -n todo-app exec -it <pod> -- sh
kubectl -n todo-app get events --sort-by=.lastTimestamp
kubectl -n todo-app top pods                  # ต้องมี metrics-server
```

**`describe pod` แล้วเลื่อนไปดู Events** คือคำสั่งที่ต้องทำเป็นอย่างแรกเสมอเวลา pod ไม่ขึ้น

---

# ภาคลึก — คำสั่งและการ setup ที่ใช้ตอน deploy จริง

ส่วนนี้คือความรู้ที่ workflow `deploy-k8s.yml` ใน [10](10-deploy-free-cloud.md) ใช้จริงทุกบรรทัด

## D1. kubeconfig — อ่านให้ออกว่าข้างในมีอะไร

```yaml
apiVersion: v1
clusters:
  - name: default
    cluster:
      server: https://127.0.0.1:6443 # ← จุดที่ทำให้ CI ต่อไม่ได้
      certificate-authority-data: LS0tLS1CRUd… # CA ที่ใช้ตรวจ certificate ของ server
users:
  - name: default
    user:
      client-certificate-data: LS0tLS1CRUd… # ตัวตนของเรา
      client-key-data: LS0tLS1CRUd… # ← นี่คือ "รหัสผ่าน" ห้ามหลุด
contexts:
  - name: default
    context: { cluster: default, user: default, namespace: todo-app }
current-context: default
```

**3 ส่วนที่ต้องแยกให้ออก:**

| ส่วน       | ตอบคำถาม                                         |
| ---------- | ------------------------------------------------ |
| `clusters` | จะไปคุยกับเครื่องไหน และเชื่อ certificate ของใคร |
| `users`    | เราเป็นใคร (ยืนยันด้วย client certificate)       |
| `contexts` | จับคู่ cluster + user + namespace ที่จะใช้       |

**ทำไม k3s ให้ `127.0.0.1` มา:** เพราะ kubeconfig ที่ k3s สร้างตั้งใจให้ใช้บนเครื่องนั้นเอง
พอเอาไปใช้จากที่อื่น `127.0.0.1` จะหมายถึงเครื่องนั้น ๆ แทน → ต้องแก้เป็น IP จริง

**ทำไมต้อง `--tls-san <IP>` ตอนติดตั้ง:** certificate ของ API server ระบุไว้ว่าใช้กับชื่อ/IP อะไรได้บ้าง
ถ้าไม่ใส่ IP สาธารณะเข้าไป จะเจอ:

```
x509: certificate is valid for 127.0.0.1, 10.43.0.1, not 203.0.113.10
```

**ตรวจ kubeconfig ก่อนเอาไปใช้เสมอ:**

```bash
kubectl --kubeconfig=./kubeconfig config view --minify        # ดูสรุป context ปัจจุบัน
kubectl --kubeconfig=./kubeconfig cluster-info                 # ต่อได้จริงไหม
kubectl --kubeconfig=./kubeconfig auth can-i create deployment -n todo-app
```

`auth can-i` คือคำสั่งที่ควรใช้ก่อนเสมอเวลาสงสัยว่า "ไม่มีสิทธิ์" หรือ "ต่อไม่ติด"

⚠️ kubeconfig = สิทธิ์เต็มในคลัสเตอร์ **ต้องอยู่ใน Secret เท่านั้น** และควรสร้าง ServiceAccount ที่มีสิทธิ์จำกัดเฉพาะ namespace สำหรับ CI แทนการใช้ของ admin

---

## D2. `apply` vs `create` vs `replace` vs `patch`

| คำสั่ง    | ถ้ายังไม่มี | ถ้ามีอยู่แล้ว            | เหมาะกับ                |
| --------- | ----------- | ------------------------ | ----------------------- |
| `create`  | สร้าง       | ❌ **error**             | สคริปต์ที่รันครั้งเดียว |
| `apply`   | สร้าง       | รวมค่าที่เปลี่ยน (merge) | **CI/CD — ใช้ตัวนี้**   |
| `replace` | ❌ error    | เขียนทับทั้งก้อน         | แทบไม่ใช้               |
| `patch`   | ❌ error    | แก้เฉพาะ field           | แก้ทีละจุด              |

**สำนวนที่ใช้ใน `deploy-k8s.yml` — "สร้างหรืออัปเดตก็ได้":**

```bash
kubectl -n todo-app create secret generic todo-secret \
  --from-literal=DATABASE_URL='...' \
  --dry-run=client -o yaml | kubectl apply -f -
```

อ่านทีละท่อน:

| ท่อน                                 | ทำอะไร                                        |
| ------------------------------------ | --------------------------------------------- |
| `create secret ... --dry-run=client` | **ไม่ส่งไปที่ server** แค่ประกอบ YAML ให้ดู   |
| `-o yaml`                            | พิมพ์ YAML ที่ประกอบได้ออก stdout             |
| `\| kubectl apply -f -`              | ส่ง YAML นั้นเข้า apply (`-` = อ่านจาก stdin) |

**ทำไมต้องอ้อมแบบนี้:** `create` เขียน YAML ของ Secret ให้เราได้ถูกต้อง (encode base64 ให้เอง)
แต่ `create` ตรง ๆ จะพังถ้ามี Secret อยู่แล้ว ส่วน `apply` ต้องการ YAML ที่พร้อมแล้ว
เอาจุดแข็งของทั้งคู่มาต่อกัน = สคริปต์ที่รันซ้ำกี่รอบก็ได้ผลเหมือนเดิม (idempotent)

`--dry-run=client` vs `--dry-run=server`:

|          | ทำอะไร                                                            |
| -------- | ----------------------------------------------------------------- |
| `client` | ประกอบ YAML ในเครื่อง ไม่ติดต่อ server เลย                        |
| `server` | ส่งไปให้ server ตรวจจริง (ผ่าน validation + webhook) แต่ไม่บันทึก |

---

## D3. exit code ของ kubectl — เรื่องที่ทำให้ CI แดงโดยไม่ตั้งใจ

```bash
kubectl diff -k k8s/overlays/cloud || true
```

**`kubectl diff` คืน exit code 1 เมื่อ "มีความต่าง"** ซึ่งเป็นเรื่องปกติของทุก deploy
ถ้าไม่มี `|| true` ต่อท้าย ทุก deploy ที่มีอะไรเปลี่ยนจะทำให้ workflow แดงทันที

| คำสั่ง           | exit 0 เมื่อ         | exit ≠ 0 เมื่อ                     |
| ---------------- | -------------------- | ---------------------------------- |
| `apply`          | สำเร็จ               | error จริง                         |
| **`diff`**       | **ไม่มีความต่าง**    | **มีความต่าง (1)** หรือ error (>1) |
| `rollout status` | rollout เสร็จสมบูรณ์ | timeout / rollout ล้มเหลว          |
| `wait`           | เงื่อนไขเป็นจริง     | timeout                            |
| `get`            | เจอ                  | ไม่เจอ                             |

**`rollout status` คือบรรทัดเดียวที่ทำให้ CD ของเรามีความหมาย** — ถ้าไม่มี workflow จะเขียวทั้งที่ pod พัง

---

## D4. `set image` — จุดที่คนพลาดบ่อยที่สุด

```bash
kubectl -n todo-app set image deployment/todo-api api="$IMAGE" migrate="$IMAGE"
```

`api` คือ container หลัก ส่วน `migrate` คือ **initContainer**

`kubectl set image` เลือกเป้าหมายจาก**ชื่อ** โดยดูทั้ง `containers` และ `initContainers`

**ถ้าลืมใส่ `migrate=`** จะเกิดสถานการณ์ที่ debug ยากมาก:

```
container หลัก  → image ใหม่ (โค้ดเวอร์ชันใหม่)
initContainer   → image เก่า (migration เวอร์ชันเก่า)
```

ผลคือ schema ของฐานข้อมูลไม่ตรงกับโค้ด → บาง endpoint ใช้ได้ บางอันพัง
และอาการจะเปลี่ยนไปทุกครั้งที่ pod restart

**เปลี่ยนทุก container ในทีเดียวได้ด้วย `*`:**

```bash
kubectl set image deployment/todo-api '*'="$IMAGE"
```

แต่เขียนชื่อให้ครบชัดเจนกว่า เพราะถ้าวันหนึ่งมี sidecar เพิ่มเข้ามา `*` จะไปเปลี่ยน image ของ sidecar ด้วย

**บันทึกว่าใครสั่ง deploy:**

```bash
kubectl annotate deployment/todo-api \
  kubernetes.io/change-cause="commit $SHA โดย $ACTOR" --overwrite
```

ค่านี้จะโผล่ในคอลัมน์ CHANGE-CAUSE ของ `kubectl rollout history` — ตอนตี 3 ที่ต้อง rollback มันมีค่ามาก

---

## D5. `rollout status` ทำงานยังไงจริง ๆ

```bash
kubectl -n todo-app rollout status deployment/todo-api --timeout=300s
```

มันจะ **บล็อกรอ** จนกว่าเงื่อนไขนี้จะเป็นจริง:

```
updatedReplicas == replicas  &&  availableReplicas == replicas  &&  observedGeneration >= generation
```

แปลไทย: pod เวอร์ชันใหม่ครบจำนวน **และ** ทุกตัวผ่าน readinessProbe แล้ว

**ถ้า pod ใหม่ขึ้นไม่ได้:** เพราะ `maxUnavailable: 0` pod เก่ายังรันอยู่ครบ → ผู้ใช้ไม่กระทบ
แต่ `rollout status` จะรอจน timeout แล้วคืน exit code ไม่เป็น 0 → workflow แดง → rollback ทำงาน

**ตั้ง timeout เท่าไรดี:**

```
timeout ≥ (เวลา pull image) + (startupProbe: periodSeconds × failureThreshold) + (เวลา boot จริง) + เผื่อ
```

ของเรา: startupProbe = 5s × 12 = 60 วิ + pull ARM image ~60 วิ + boot ~10 วิ → **300 วิ กำลังพอดี**
ตั้งสั้นไปจะ rollback ทั้งที่ของกำลังจะขึ้นได้ ตั้งยาวไปจะรู้ผลช้า

**คำสั่งที่เกี่ยวข้อง:**

```bash
kubectl rollout status deploy/x --watch=false   # เช็คครั้งเดียวไม่รอ
kubectl rollout pause  deploy/x                  # หยุด rollout กลางคัน
kubectl rollout resume deploy/x
kubectl rollout history deploy/x --revision=3    # ดูรายละเอียดเวอร์ชันที่ 3
```

---

## D6. `kubectl wait` — รอให้ถูกวิธี

```bash
kubectl -n ingress-nginx wait --for=condition=available deploy/ingress-nginx-controller --timeout=180s
kubectl -n todo-app wait --for=condition=ready pod -l app=todo-api --timeout=120s
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded job/migrate --timeout=300s
```

**ทำไมดีกว่า `sleep`:** `sleep 30` บน runner ที่เร็วคือเสียเวลา 25 วิ ส่วนบน runner ที่ช้าคือไม่พอ
`wait` จบทันทีที่พร้อม และ**ล้มเหลวอย่างชัดเจน**เมื่อไม่พร้อมจริง

⚠️ `wait` จะ error ทันทีถ้า resource ยังไม่มีอยู่ — ถ้าเพิ่ง apply ไป ต้องรอให้ object ถูกสร้างก่อน:

```bash
kubectl wait --for=create pod -l app=todo-api --timeout=60s   # k8s 1.31+
```

---

## D7. อ่านค่าจากคลัสเตอร์ในสคริปต์

```bash
# ดึงค่าเดียว
kubectl -n todo-app get deploy todo-api -o jsonpath='{.spec.template.spec.containers[0].image}'

# ดึงหลายค่าเป็นตาราง
kubectl -n todo-app get pods -o custom-columns=NAME:.metadata.name,READY:.status.containerStatuses[0].ready

# นับ pod ที่ ready
kubectl -n todo-app get pods -l app=todo-api \
  -o jsonpath='{range .items[*]}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' | grep -c True

# หา image ที่รันอยู่จริง (digest ไม่ใช่ tag)
kubectl -n todo-app get pods -l app=todo-api -o jsonpath='{.items[0].status.containerStatuses[0].imageID}'
```

**บรรทัดสุดท้ายมีค่ามาก** — `imageID` บอก digest ของ image ที่**กำลังรันอยู่จริง**
ต่างจาก `.spec...image` ที่บอกแค่สิ่งที่เราขอไป ถ้าใช้ tag ลอย สองค่านี้จะไม่ตรงกัน

---

## D8. Field ownership — ทำไม HPA กับ `replicas` ถึงตีกัน

Kubernetes จำว่า **ใครเป็นคนตั้งค่าแต่ละ field** (server-side apply)

```bash
kubectl -n todo-app get deploy todo-api -o yaml | grep -A15 managedFields
```

**ปัญหาคลาสสิก:** manifest มี `replicas: 2` และมี HPA ที่ปรับเป็น 4
พอ `kubectl apply` ครั้งถัดไป → replicas ถูกดึงกลับเป็น 2 → HPA ปรับกลับเป็น 4 → **แกว่งไปมาตลอด**

**ทางแก้ 3 แบบ:**

| วิธี                              | ทำยังไง                       |
| --------------------------------- | ----------------------------- |
| ไม่ใส่ `replicas` ใน manifest เลย | ปล่อยให้ HPA คุมอย่างเดียว    |
| ใช้ `kubectl apply --server-side` | k8s จะเคารพเจ้าของ field เดิม |
| ใช้ Argo CD + `ignoreDifferences` | ระบุให้ข้าม `/spec/replicas`  |

โปรเจกต์นี้ใส่ `replicas` ไว้เพื่อการเรียนรู้ (จะได้เห็นค่าชัด ๆ) — ในระบบจริงที่เปิด HPA **ควรเอาออก**

---

## D9. ลำดับการ setup คลัสเตอร์ใหม่ตั้งแต่ศูนย์

เรียงตามลำดับที่ต้องทำ ข้ามไม่ได้:

```
1. ติดตั้ง k3s (--tls-san <IP>)         → ได้ API server
2. ดึง kubeconfig + แก้ server IP        → ต่อจากข้างนอกได้
3. ทดสอบ kubectl cluster-info            ← ห้ามข้าม ถ้าตรงนี้ไม่ผ่าน ที่เหลือไม่มีทางผ่าน
4. สร้าง namespace                        → ที่อยู่ของ resource
5. สร้าง Secret (DATABASE_URL)            ← ต้องมาก่อน apply เพราะ pod อ้างถึง
6. kubectl apply -k overlays/cloud        → สร้าง Deployment/Service/Ingress
7. rollout status                         → ยืนยันว่า pod ขึ้นจริง
8. ทดสอบผ่าน Ingress จากภายนอก            → ยืนยัน network ทั้งเส้น
```

**ข้อ 5 ต้องมาก่อนข้อ 6 เสมอ** — ถ้า apply ก่อนสร้าง Secret pod จะค้างที่
`CreateContainerConfigError` และรอไปเรื่อย ๆ จนกว่า Secret จะโผล่มา
(k8s ไม่ล้มเหลวทันที แต่รอ — ซึ่งทำให้ `rollout status` timeout แทนที่จะบอกสาเหตุจริง)

**ตรวจทีละชั้นเวลาเข้าไม่ได้** — ไล่จากในออกนอก:

```bash
kubectl -n todo-app get pods                                    # 1. pod ขึ้นไหม
kubectl -n todo-app exec deploy/todo-api -- wget -qO- localhost:3000/healthz  # 2. แอปตอบไหม
kubectl -n todo-app get endpoints todo-api                      # 3. Service เห็น pod ไหม
kubectl -n todo-app port-forward svc/todo-api 8080:80           # 4. Service ใช้ได้ไหม
kubectl get ingress -A                                          # 5. Ingress ถูกสร้างไหม
curl -H "Host: todo.x.nip.io" http://<IP>/healthz               # 6. เข้าจากนอกได้ไหม
```

**ชั้นแรกที่พังคือชั้นที่ต้องแก้** — วิธีนี้ใช้เวลาไม่กี่นาทีและตอบได้เสมอว่าปัญหาอยู่ตรงไหน

---

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] แก้ `readinessProbe` ให้ชี้ path ผิด (เช่น `/notexist`) แล้ว apply ดูว่า `rollout status` ค้างตรงไหนและ `describe pod` บอกอะไร
- [ ] `kubectl -n todo-app delete pod <pod>` ตัวหนึ่งระหว่างมี traffic แล้วดูว่า Service เปลี่ยนไปหา pod ที่เหลือเร็วแค่ไหน
- [ ] ลอง scale `kubectl -n todo-app scale deployment/todo-api --replicas=5` แล้วยิงโหลดดูว่า HPA ปรับกลับไหมถ้ามันตั้งเป็น `minReplicas: 2, maxReplicas: 6`
- [ ] เทียบผลลัพธ์ `kubectl kustomize k8s/overlays/dev` กับ `k8s/overlays/production` — ต่างกันตรงไหนบ้างจริง ๆ
- [ ] ลบ `initContainers` ของ migrate ออกชั่วคราวแล้ว apply ดูว่า pod ตัวหลักพังยังไงเมื่อ schema ไม่ตรง

➡️ ต่อไป: [แบบฝึกหัด](../exercises/README.md)
📊 อ่านคู่กัน: [09 — จะตั้ง LB / rate limit / health check ที่ชั้นไหนดี](09-where-to-configure.md)
🏋️ ฝึกมือ: [แบบฝึกหัด Kubernetes](../exercises/kubernetes/01-beginner.md)
