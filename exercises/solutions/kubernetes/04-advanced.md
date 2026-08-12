# เฉลย — ☸️ Kubernetes ระดับ 4

⬅️ [กลับไปที่โจทย์](../../kubernetes/04-advanced.md)

---

## K4.1 ติดตั้ง Argo CD

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
kubectl -n argocd port-forward svc/argocd-server 8080:443
```

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: todo-api-dev
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/<you>/devops-todo-api.git
    targetRevision: main
    path: k8s/overlays/dev
  destination:
    server: https://kubernetes.default.svc
    namespace: todo-app
  syncPolicy:
    automated: { prune: true, selfHeal: true }
    syncOptions: [CreateNamespace=true]
```

**ทดสอบวงจร:** แก้ replicas ใน git → commit → push → รอไม่เกิน 3 นาที (หรือกด Refresh) → คลัสเตอร์เปลี่ยนตาม

**ทำไมต้องรอถึง 3 นาที:** Argo poll git ทุก 3 นาทีโดยค่าเริ่มต้น
ถ้าอยากได้ทันทีให้ตั้ง webhook จาก GitHub → Argo (แต่ถ้า Argo อยู่หลัง VPN GitHub ยิงเข้ามาไม่ได้ ดู [docs/11](../../../docs/11-enterprise-gitops.md))

---

## K4.2 selfHeal

```bash
kubectl -n todo-app scale deployment/todo-api --replicas=10
kubectl -n todo-app get deploy -w
# 10 → กลับเป็นค่าใน git ภายในไม่กี่วินาที
```

**ทำไมนี่คือหัวใจของ GitOps:** ถ้าไม่มี selfHeal คนแก้ด้วยมือได้แล้วไม่มีใครรู้
พอผ่านไปสามเดือน คลัสเตอร์กับ git จะต่างกันเยอะจนไม่มีใครกล้า `kubectl apply` เพราะกลัวทับของที่จำเป็น
(อาการนี้เรียกว่า **configuration drift** และเป็นสาเหตุอันดับต้น ๆ ของ incident ตอน deploy)

selfHeal ทำให้ประโยคที่ว่า "git คือความจริง" เป็นสิ่งที่**บังคับใช้จริง** ไม่ใช่แค่ข้อตกลงที่คนลืมได้

⚠️ ผลข้างเคียงที่ต้องรู้: ตอนแก้ปัญหาฉุกเฉิน ถ้าใครทำ `kubectl rollout undo`
Argo จะดึงกลับไปเวอร์ชันที่พังทันที → **ต้องหยุด auto-sync ก่อน หรือ revert ที่ git แทน**

---

## K4.3 sync wave

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: todo-migrate
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: BeforeHookCreation
```

| annotation | ผล |
| --- | --- |
| `hook: PreSync` | รันก่อน resource อื่นทั้งหมด และ**รอจนจบ** |
| `hook: PostSync` | รันหลัง sync เสร็จ (เหมาะกับ smoke test) |
| `sync-wave: "-1"` | เรียงลำดับภายใน phase เดียวกัน (น้อยมาก่อน) |
| `hook-delete-policy: BeforeHookCreation` | ลบ Job เก่าก่อนสร้างใหม่ (ไม่งั้น Job ชื่อซ้ำจะสร้างไม่ได้) |

**ทดสอบว่าทำงานจริง:** ทำให้ migration พัง (เช่นใส่ SQL ผิด) แล้ว sync
→ Argo หยุดที่ PreSync, Deployment **ไม่ถูกแตะเลย**, ของเก่ายังรันอยู่ปกติ

นี่คือพฤติกรรมที่ต้องการ — **migration พังไม่ควรทำให้ระบบที่รันอยู่ล่ม**

---

## K4.4 canary

**ทางเลือก A — Ingress annotation (ง่ายกว่า):**

```yaml
# Ingress ตัวที่สอง ชี้ไป service ใหม่
metadata:
  annotations:
    nginx.ingress.kubernetes.io/canary: "true"
    nginx.ingress.kubernetes.io/canary-weight: "10"
```

**ทางเลือก B — Argo Rollouts (ทำได้มากกว่า):**

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
spec:
  strategy:
    canary:
      steps:
        - setWeight: 10
        - pause: { duration: 5m }
        - setWeight: 50
        - pause: { duration: 5m }
        - setWeight: 100
      analysis:
        templates: [{ templateName: success-rate }]
```

| | Rolling update | Canary |
| --- | --- | --- |
| แบ่ง traffic | ❌ ตามสัดส่วน pod เท่านั้น | ✅ กำหนด % ได้อิสระ |
| หยุดกลางคันเพื่อดูผล | ❌ | ✅ |
| ย้อนกลับอัตโนมัติจาก metric | ❌ | ✅ (Argo Rollouts) |
| ความซับซ้อน | ต่ำ | สูงกว่า |

**ความต่างที่สำคัญที่สุด:** rolling update วัดแค่ "pod ready ไหม" ส่วน canary วัด **"เวอร์ชันใหม่ทำงานดีจริงไหม"** จาก metric ของธุรกิจ
pod ที่ ready แต่ตอบ 500 ให้ 30% ของ request จะผ่าน rolling update ฉลุย แต่ canary จับได้

---

## K4.5 Prometheus

```ts
// src/metrics.ts
import client from "prom-client";
export const registry = new client.Registry();
client.collectDefaultMetrics({ register: registry });

export const httpDuration = new client.Histogram({
  name: "http_request_duration_seconds",
  help: "ระยะเวลาของ request",
  labelNames: ["method", "route", "status"],
  buckets: [0.01, 0.05, 0.1, 0.3, 0.5, 1, 3],
  registers: [registry],
});
```

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: todo-api
spec:
  selector: { matchLabels: { app: todo-api } }
  endpoints: [{ port: http, path: /metrics, interval: 15s }]
```

**คำเตือนสำคัญเรื่อง label:** อย่าใช้ค่าที่มีความหลากหลายสูงเป็น label (user id, request id, path ที่มี id อยู่ข้างใน)
เพราะ Prometheus สร้าง time series แยกต่อทุกชุด label → **cardinality explosion** ทำให้ Prometheus กินแรมจนตาย

```
❌ route="/api/todos/12345"      → หนึ่ง series ต่อหนึ่ง todo
✅ route="/api/todos/:id"        → หนึ่ง series
```

นี่คือความผิดพลาดที่ทำให้ระบบ monitoring ล่มบ่อยที่สุด

---

## K4.6 HPA จาก custom metric

```yaml
metrics:
  - type: Pods
    pods:
      metric: { name: http_requests_per_second }
      target: { type: AverageValue, averageValue: "100" }
```

ต้องมี `prometheus-adapter` แปลง metric ของ Prometheus ให้เป็น custom metrics API ของ k8s

**ทำไม rps ดีกว่า CPU สำหรับ API ที่รอ I/O:**

แอปที่ใช้เวลาส่วนใหญ่รอ DB จะมี CPU ต่ำมาก (10-20%) แม้จะมี request ค้างเป็นร้อย
→ HPA ที่ดู CPU จะไม่ scale เลย ทั้งที่ latency พุ่งไปแล้ว

| metric | เหมาะกับ |
| --- | --- |
| CPU | งานที่ใช้ CPU จริง (encode, คำนวณ) |
| rps / concurrent requests | **API ที่รอ I/O เป็นหลัก** |
| queue length | worker ที่ดึงงานจาก queue — **ตัวชี้วัดที่ตรงที่สุดสำหรับงานแบบนี้** |

---

## K4.7 Loki

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm install loki grafana/loki-stack --set promtail.enabled=true
```

ค้นด้วย LogQL:

```logql
{namespace="todo-app"} |= "abc-123-request-id" | json
{namespace="todo-app"} | json | level="error"
```

**ทำไม structured logging (JSON) ถึงสำคัญตรงนี้:** Loki parse JSON ได้ทันที ทำให้ filter ตาม field ได้
ถ้า log เป็นข้อความอิสระ ต้องเขียน regex ทุกครั้ง — ซึ่งจะพังทันทีที่มีคนแก้ข้อความ log

**ต่อยอดสำคัญ:** ส่ง `X-Request-Id` จาก nginx เข้าไปใน log ของแอปด้วย → ค้น id เดียวแล้วเห็นทั้งเส้นทาง

---

## K4.8 node ดับ

```bash
minikube start --nodes=3
minikube node stop minikube-m02
kubectl get nodes -w
```

**ลำดับเวลาที่เกิดขึ้น (นานกว่าที่คนคาดเสมอ):**

```
t=0      node หยุดส่ง heartbeat
t=40s    node-monitor-grace-period หมด → node เป็น NotReady
t=40s    taint node.kubernetes.io/unreachable ถูกใส่
t=5m40s  tolerationSeconds (300 วิ) หมด → pod ถูกไล่ออก
t=5m45s  pod ใหม่ถูก schedule ไป node อื่น
```

**รวมประมาณ 6 นาที** ที่ pod บน node นั้นยังถูกนับว่า "มีอยู่" ทั้งที่เข้าถึงไม่ได้

**ทำไมนานขนาดนั้น:** k8s แยกไม่ออกระหว่าง "node ตายจริง" กับ "network สะดุดชั่วคราว"
ถ้ารีบไล่ pod ทุกครั้งที่ network กระตุก ระบบจะสั่นตลอดเวลา — จึงเลือกความระมัดระวังไว้ก่อน

**สิ่งที่ป้องกันผู้ใช้ได้จริงระหว่าง 6 นาทีนั้นคือ readinessProbe** ที่ถอด pod ออกจาก endpoints ภายใน ~30 วินาที
ไม่ใช่กลไกการไล่ pod

ลดเวลาได้ด้วย `tolerationSeconds` ที่สั้นลง แต่ต้องแลกกับความเสี่ยงที่จะย้าย pod เพราะ network กระตุกชั่วคราว

---

## K4.9 Helm vs Kustomize

```bash
helm create todo-api
# ย้าย manifest จาก k8s/base ไป templates/ แล้วแทนค่าด้วย {{ .Values.xxx }}
helm install todo-api ./todo-api -n todo-app -f values-dev.yaml
```

| | Kustomize | Helm |
| --- | --- | --- |
| แนวคิด | patch YAML จริง | template + ตัวแปร |
| อ่านง่าย | ✅ YAML ยังเป็น YAML | ❌ เต็มไปด้วย `{{ }}` |
| ตรรกะเงื่อนไข | ❌ | ✅ if/range/function |
| แจกจ่ายให้คนอื่น | ยาก | ✅ chart repository |
| ติดตั้งเพิ่ม | ❌ มีใน kubectl แล้ว | ต้องลง helm |
| จัดการ lifecycle (install/upgrade/rollback) | ❌ | ✅ |

**คำตอบสำหรับโปรเจกต์นี้: kustomize เหมาะกว่า** เพราะเป็นแอปของเราเอง ไม่ได้แจกให้คนอื่น และไม่ต้องการตรรกะซับซ้อน

**Helm เหมาะเมื่อ:** ต้องแจกจ่าย chart ให้ทีมอื่น/ลูกค้า หรือ config ต่างกันมากระหว่าง environment จนต้องมี if/else

**ในทางปฏิบัติหลายทีมใช้ทั้งคู่:** Helm สำหรับ third-party (Prometheus, ingress-nginx) + Kustomize สำหรับแอปตัวเอง
และ Argo CD รองรับทั้งสองอย่างในคลัสเตอร์เดียวกันได้

---

## 🎯 ต่อยอด

- ลอง ApplicationSet สร้าง Application ให้ทุก overlay อัตโนมัติ
- ตั้ง Argo CD notifications ส่งเข้า Slack เมื่อ sync ล้มเหลว
- วัด cardinality ของ metric ด้วย `count({__name__=~".+"}) by (__name__)` แล้วดูว่าตัวไหนระเบิด
