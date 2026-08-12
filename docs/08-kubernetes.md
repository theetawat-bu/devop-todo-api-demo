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

| Object | หน้าที่ | ไฟล์ |
|---|---|---|
| **Namespace** | แบ่งห้องให้ทรัพยากร | `base/namespace.yaml` |
| **ConfigMap** | ค่าตั้งค่าที่ไม่ลับ | `base/configmap.yaml` |
| **Secret** | ค่าลับ (base64 ไม่ใช่การเข้ารหัส!) | `base/secret.yaml` |
| **Deployment** | ดูแล pod ที่ไม่มี state — scale/rolling update/rollback | `base/api.yaml` |
| **StatefulSet** | ดูแล pod ที่มี state — ชื่อคงที่ + ดิสก์ส่วนตัว | `base/postgres.yaml` |
| **Service** | ชื่อ + IP คงที่หน้ากลุ่ม pod, load balance ให้ | `base/api.yaml` |
| **Ingress** | ประตูจากภายนอกเข้าคลัสเตอร์ (L7) | `base/ingress.yaml` |
| **HPA** | เพิ่ม/ลด pod ตามโหลด | `base/hpa.yaml` |
| **PDB** | กันไม่ให้ pod ถูกไล่ออกพร้อมกันหมด | `base/api.yaml` |

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
livenessProbe:   { httpGet: { path: /healthz, port: http }, periodSeconds: 20 }
readinessProbe:  { httpGet: { path: /readyz,  port: http }, periodSeconds: 10 }
startupProbe:    { httpGet: { path: /healthz, port: http }, periodSeconds: 5, failureThreshold: 12 }
```

| Probe | ไม่ผ่านแล้วเกิดอะไร | ใช้เช็คอะไร |
|---|---|---|
| liveness | **ฆ่า pod ทิ้งแล้วสร้างใหม่** | แค่ว่า process ไม่ค้าง — **ห้ามเช็ค DB** |
| readiness | **ถอดออกจาก Service** ไม่ฆ่า | dependency เช่น DB |
| startup | รอจนกว่าจะผ่าน ค่อยเริ่มนับ liveness | แอปที่ boot ช้า |

พลาดบ่อยสุด: เอา liveness ไปเช็ค DB → DB ล่มทีเดียว pod restart ทั้งคลัสเตอร์ ยิ่งทำให้แย่ลง

### initContainer สำหรับ migration

```yaml
initContainers:
  - name: migrate
    image: <same image>
    command: ["sh", "-c", "npx prisma migrate deploy"]
```

initContainer รันจนจบก่อน container หลักจะเริ่ม → migration รันครั้งเดียวก่อน ไม่ใช่รันพร้อมกันทุก pod
(ถ้าใส่ไว้ใน CMD ของแอป pod 3 ตัวจะแย่งกันรัน migration พร้อมกัน)

ทางเลือกที่ดีกว่าสำหรับ production คือแยกเป็น `Job` หรือ Helm hook

### Resources — ต้องใส่เสมอ

```yaml
resources:
  requests: { cpu: 100m, memory: 128Mi }   # จองขั้นต่ำ — scheduler ใช้เลือก node
  limits:   { cpu: 500m, memory: 256Mi }   # เพดาน — เกิน memory = OOMKilled
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
ฝั่งแอปต้องรองรับด้วย — ดู `src/index.ts` ที่ดัก `SIGTERM` แล้วปิด server อย่างสุภาพ ไม่ตัด request ที่ค้างอยู่

```ts
process.on('SIGTERM', () => shutdown('SIGTERM'));
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

ทำงานคู่กับ `USER node` ใน Dockerfile ถ้า image รันเป็น root อยู่ pod จะสตาร์ทไม่ขึ้นเลย

### HPA

```yaml
minReplicas: 2
maxReplicas: 6
metrics:
  - type: Resource
    resource: { name: cpu, target: { type: Utilization, averageUtilization: 70 } }
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

➡️ ต่อไป: [09 — แบบฝึกหัด](09-exercises.md)
