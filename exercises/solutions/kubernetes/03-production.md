# เฉลย — ☸️ Kubernetes ระดับ 3

⬅️ [กลับไปที่โจทย์](../../kubernetes/03-production.md)

---

## K3.1 HPA

```bash
kubectl -n todo-app get hpa -w
kubectl -n todo-app run load --rm -it --image=busybox --restart=Never -- \
  sh -c "while true; do wget -q -O- http://todo-api/api/todos; done"
```

```
NAME           TARGETS         MINPODS  MAXPODS  REPLICAS
todo-api-hpa   15%/70%         2        6        2
todo-api-hpa   180%/70%        2        6        2      ← โหลดพุ่ง
todo-api-hpa   180%/70%        2        6        4      ← scale up (เร็ว)
todo-api-hpa   62%/70%         2        6        5
[หยุดโหลด]
todo-api-hpa   5%/70%          2        6        5      ← ยังไม่ลด
todo-api-hpa   5%/70%          2        6        2      ← ลดหลัง ~2 นาที
```

**สูตรที่ HPA ใช้:**

```
replicas ใหม่ = ceil( replicas ปัจจุบัน × (metric ปัจจุบัน / metric เป้าหมาย) )
```

**ทำไมขาขึ้นเร็วแต่ขาลงช้า:** เป็นความตั้งใจของการออกแบบ

- scale ขึ้นช้า = ผู้ใช้เจอ error → ต้องรีบ
- scale ลงเร็ว = ถ้าโหลดกลับมาอีกจะไม่ทัน (pod ใช้เวลา start) → ต้องรอให้แน่ใจ

`stabilizationWindowSeconds: 120` = ดูค่าย้อนหลัง 2 นาทีแล้วใช้ค่า**สูงสุด**ในการตัดสินใจ ป้องกัน flapping

**ถ้า TARGETS ขึ้น `<unknown>`:** ไม่มี metrics-server หรือ Deployment ไม่ได้ตั้ง `resources.requests`

---

## K3.2 OOMKilled

```yaml
resources:
  limits: { memory: 64Mi }
```

```bash
kubectl -n todo-app describe pod <pod> | grep -A5 "Last State"
# Last State:  Terminated
#   Reason:    OOMKilled
#   Exit Code: 137
```

| | เกิน CPU limit | เกิน memory limit |
| --- | --- | --- |
| เกิดอะไร | throttle (ช้าลง) | **ถูกฆ่าทันที** |
| แอปรู้ตัวไหม | ไม่รู้ แค่ช้า | ไม่รู้เลย SIGKILL ดักไม่ได้ |
| graceful shutdown | ไม่เกี่ยว | **ไม่มี** request ที่ค้างหายหมด |
| เห็นใน | metric CPU throttling | Events + Last State |

**ทำไม memory ต้องฆ่า ไม่ throttle:** CPU แบ่งเวลากันใช้ได้ แต่ memory ที่จองไปแล้วจะคืนไม่ได้
kernel ไม่มีทางเลือกอื่นนอกจากฆ่า process ที่ขอเกิน

**ข้อควรระวังเฉพาะ Go:** Go runtime เองก็ไม่รู้จัก container memory limit โดยอัตโนมัติเช่นกัน (แก้ไขบางส่วนตั้งแต่ Go 1.19 ด้วย `GOMEMLIMIT` แต่ยังไม่ผูกกับ cgroup limit อัตโนมัติ)
ถ้าตั้ง limit 256Mi แต่ GC ยังคิดว่ามี RAM ทั้งเครื่องให้ใช้ heap อาจโตจนโดนฆ่าก่อนที่ GC จะไล่เก็บทัน
ทางแก้: ตั้ง `GOMEMLIMIT=200MiB` (ประมาณ 75-80% ของ limit) เพื่อบอก GC ให้ทำงานถี่ขึ้นก่อนถึงเพดานจริง

---

## K3.3 PodDisruptionBudget

```bash
kubectl -n todo-app get pdb
# NAME           MIN AVAILABLE   ALLOWED DISRUPTIONS
# todo-api-pdb   1               1
kubectl drain <node> --ignore-daemonsets --delete-emptydir-data
```

**PDB คุมได้เฉพาะ voluntary disruption:**

| เหตุการณ์ | PDB ช่วยไหม |
| --- | --- |
| `kubectl drain` (อัปเกรด node) | ✅ |
| cluster autoscaler ลด node | ✅ |
| อัปเดต node pool | ✅ |
| **node ดับกะทันหัน** | ❌ |
| **pod ถูกฆ่าเพราะ OOM** | ❌ |
| **kubectl delete pod** | ❌ |

**สิ่งที่คนเข้าใจผิด:** คิดว่า PDB คือการรับประกันว่าจะมี pod พร้อมใช้เสมอ — **ไม่ใช่**
มันแค่บอกกับ "ตัวที่ขออนุญาตก่อนไล่ pod" ว่าห้ามไล่เกินเท่านี้ ส่วนเหตุการณ์ที่ไม่ขออนุญาต (ฮาร์ดแวร์พัง) มันช่วยไม่ได้

⚠️ **กับดัก:** ถ้าตั้ง `minAvailable: 1` แต่มี replica เดียว → `kubectl drain` จะค้างตลอดกาล เพราะไล่ pod ไม่ได้เลย
ตั้ง PDB ต้องดูจำนวน replica เสมอ (`minAvailable` ต้องน้อยกว่า replicas)

---

## K3.4 NetworkPolicy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: postgres-allow-api-only
  namespace: todo-app
spec:
  podSelector: { matchLabels: { app: postgres } }
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector: { matchLabels: { app: todo-api } }
      ports:
        - { protocol: TCP, port: 5432 }
```

ทดสอบ:

```bash
kubectl -n todo-app run test --rm -it --image=busybox --restart=Never -- \
  nc -zv postgres 5432        # ควรค้างแล้ว timeout
kubectl -n todo-app exec deploy/todo-api -- nc -zv postgres 5432   # ควรต่อได้
```

**สิ่งที่ต้องรู้ 3 ข้อ:**

1. NetworkPolicy **ต้องมี CNI ที่รองรับ** (Calico, Cilium) — ถ้า CNI ไม่รองรับ manifest จะ apply ผ่านแต่**ไม่มีผลอะไรเลย** (อันตรายมาก เพราะคิดว่าปลอดภัยแล้ว)
2. เป็น **allowlist** — พอมี policy ที่ select pod นั้นแล้ว ทุกอย่างที่ไม่ได้ allow จะถูกปฏิเสธ
3. ค่าเริ่มต้นของ k8s คือ **ทุก pod คุยกันได้หมดทั้งคลัสเตอร์** ซึ่งไม่ปลอดภัยเลยในสภาพแวดล้อมที่มีหลายทีม

**แนวทาง production:** ตั้ง default-deny ทั้ง namespace ก่อน แล้วค่อยเปิดเฉพาะที่จำเป็น

---

## K3.5 migration เป็น Job

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: todo-migrate
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: BeforeHookCreation
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ghcr.io/OWNER/REPO:tag
          command: ["sh", "-c", "migrate -path ./migrations -database \"$DATABASE_URL\" up"]
          env:
            - name: DATABASE_URL
              valueFrom: { secretKeyRef: { name: todo-secret, key: DATABASE_URL } }
```

แล้วลบ `initContainers` ออกจาก Deployment

**ทำไมดีกว่าเมื่อมีหลาย replica:**

| | initContainer | Job |
| --- | --- | --- |
| รันกี่ครั้ง | **ทุก pod** (3 replica = 3 ครั้ง) | 1 ครั้ง |
| รันพร้อมกันไหม | ใช่ — เสี่ยง race condition | ไม่ |
| เห็น log แยกไหม | ปนกับ pod | ✅ แยกชัด |
| ถ้า migration พัง | pod ทุกตัวค้างที่ Init | Job แดง ชัดเจน |

> golang-migrate มี advisory lock (ผ่าน Postgres `pg_advisory_lock`) กัน migration ชนกันอยู่แล้ว แต่การพึ่ง lock ของเครื่องมือ ไม่ดีเท่าการออกแบบให้มันรันครั้งเดียวตั้งแต่แรก

**ทางเลือกที่ดีที่สุด** สำหรับ GitOps คือ Job + `PreSync` hook เพราะ Argo จะรอให้ migration จบก่อนค่อย apply Deployment
และถ้า migration พัง Deployment จะไม่ถูกแตะเลย — ของเก่ายังรันอยู่ปกติ

---

## K3.6 Sealed Secrets

```bash
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.27.0/controller.yaml
brew install kubeseal

kubectl -n todo-app create secret generic todo-secret \
  --from-literal=DATABASE_URL='postgresql://...' \
  --dry-run=client -o yaml | kubeseal --format yaml > k8s/base/sealed-secret.yaml
```

พิสูจน์ว่าถอดเองไม่ได้:

```bash
cat k8s/base/sealed-secret.yaml     # เห็นแต่ ciphertext ยาว ๆ
# ลอง base64 -d → ได้ขยะ ไม่ใช่ข้อมูลเดิม
```

**หลักการทำงาน:** `kubeseal` เข้ารหัสด้วย **public key** ของ controller ในคลัสเตอร์
private key อยู่ในคลัสเตอร์เท่านั้นและไม่เคยออกไปไหน → ใครได้ไฟล์ไปก็ถอดไม่ได้ ถอดได้เฉพาะคลัสเตอร์นั้น

⚠️ **สิ่งที่ต้องสำรอง:** private key ของ controller — ถ้าคลัสเตอร์พังแล้วไม่มี key สำรอง จะถอด SealedSecret ทั้งหมดไม่ได้เลย

```bash
kubectl -n kube-system get secret -l sealedsecrets.bitnami.com/sealed-secrets-key -o yaml > backup-key.yaml
# แล้วเก็บไฟล์นี้ในที่ปลอดภัย (ไม่ใช่ git!)
```

---

## K3.7 security context

```bash
kubectl -n todo-app exec deploy/todo-api -- id
# uid=1000 gid=1000
kubectl -n todo-app exec deploy/todo-api -- touch /test
# touch: /test: Read-only file system
```

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities: { drop: ["ALL"] }
volumeMounts:
  - { name: tmp, mountPath: /tmp }
volumes:
  - name: tmp
    emptyDir: {}
```

**ต้องมีทั้งสองฝั่ง:** `USER app` ใน Dockerfile (user ที่สร้างเองด้วย `adduser`) ทำให้ image รันเป็น non-root
ส่วน `runAsNonRoot: true` ใน k8s คือการ**บังคับ** — ถ้า image ดันรันเป็น root pod จะไม่สตาร์ทเลย

**`allowPrivilegeEscalation: false`** กัน setuid binary ยกระดับสิทธิ์ตัวเอง
**`capabilities: drop ALL`** ตัดสิทธิ์พิเศษของ Linux ทั้งหมด (แอปเว็บไม่ต้องใช้สักตัว)

---

## K3.8 กระจาย pod

```yaml
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: ScheduleAnyway # หรือ DoNotSchedule
    labelSelector:
      matchLabels: { app: todo-api }
```

| ค่า | ผล |
| --- | --- |
| `ScheduleAnyway` | พยายามกระจาย แต่ถ้าทำไม่ได้ก็ยอมวางกระจุก |
| `DoNotSchedule` | ถ้ากระจายไม่ได้ → **pod ค้างที่ Pending** |

**ถ้ามี node เดียว + `DoNotSchedule`:** pod ที่ 2 จะ Pending ตลอดไป

```bash
kubectl -n todo-app describe pod <pending-pod>
# 0/1 nodes are available: 1 node(s) didn't match pod topology spread constraints
```

**คำแนะนำ:** ใช้ `ScheduleAnyway` เว้นแต่การกระจายเป็นข้อกำหนดที่ยอมไม่ได้จริง ๆ
เพราะ `DoNotSchedule` ทำให้ระบบ scale ไม่ได้ในวันที่ node ไม่พอ — ซึ่งมักเป็นวันที่ต้องการ scale มากที่สุด

---

## K3.9 graceful shutdown บนคลัสเตอร์

```bash
curl http://todo.local/api/slow &     # endpoint ที่ใช้ 5 วิ
sleep 1
kubectl -n todo-app delete pod <pod ที่รับ request นั้น>
wait                                   # request ได้คำตอบครบ
```

**ลำดับเหตุการณ์ที่แท้จริง (สองอย่างนี้เกิดพร้อมกัน ไม่ได้เรียงกัน):**

```
t=0    kubectl delete pod
       ├─ [เส้นทาง A] pod ถูกลบออกจาก Endpoints → kube-proxy อัปเดตกฎ (ใช้เวลาเป็นวินาที)
       └─ [เส้นทาง B] kubelet ส่ง SIGTERM ให้ container ทันที

t=0    แอปได้ SIGTERM → srv.Shutdown(ctx) → ไม่รับ connection ใหม่ แต่ทำงานค้างต่อ
t=5    request เดิมเสร็จ → ปิด DB pool (pgxpool.Close()) → os.Exit(0)
t=30   (ถ้ายังไม่ตาย) SIGKILL
```

**ช่องโหว่ที่ทำให้ยังเจอ 502:** ระหว่าง t=0 ถึงตอนที่ kube-proxy ทุก node อัปเดตเสร็จ
ยังมี traffic ใหม่ถูกส่งมาที่ pod ที่ปิดรับ connection ไปแล้ว

ทางแก้มาตรฐาน:

```yaml
lifecycle:
  preStop:
    exec:
      command: ["sh", "-c", "sleep 5"]
```

`preStop` ทำงาน**ก่อน** SIGTERM → หน่วง 5 วินาทีให้ endpoints อัปเดตทั่วถึงก่อน แล้วค่อยเริ่มปิด
(อย่าลืมว่า `terminationGracePeriodSeconds` ต้องมากกว่า preStop + เวลาปิดงานจริง)

---

## 🎯 ต่อยอด

- ลอง Vertical Pod Autoscaler ในโหมด `recommendation` เพื่อดูว่ามันแนะนำ requests เท่าไร
- ตั้ง `default-deny` NetworkPolicy ทั้ง namespace แล้วไล่เปิดทีละอย่าง
- ลอง `kubectl debug` แนบ ephemeral container เข้า pod ที่ไม่มี shell
