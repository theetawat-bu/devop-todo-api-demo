# เฉลย — ☸️ Kubernetes ระดับ 1

⬅️ [กลับไปที่โจทย์](../../kubernetes/01-beginner.md)

---

## K1.1 deploy ครั้งแรก

```bash
kubectl apply -k k8s/overlays/dev
kubectl -n todo-app get pods -w
kubectl -n todo-app get all
```

ถ้า pod ไม่ขึ้น ให้ไล่ตามนี้เสมอ:

```bash
kubectl -n todo-app describe pod <pod>     # ดู Events ท้ายสุด — ตอบได้เกือบทุกเคส
kubectl -n todo-app logs <pod> -c migrate  # log ของ initContainer
```

**เคสที่เจอบ่อยที่สุด:** `ImagePullBackOff` เพราะยังไม่ได้แก้ `ghcr.io/OWNER/REPO` ใน `k8s/base/api.yaml`
ถ้าอยากใช้ image ที่ build ในเครื่องกับ minikube:

```bash
eval $(minikube docker-env)          # ชี้ docker client ไปที่ daemon ของ minikube
docker build -t devops-todo-api:local .
# แล้วตั้ง imagePullPolicy: IfNotPresent (ตั้งไว้แล้วในโปรเจกต์)
```

---

## K1.2 เข้าใช้งาน

```bash
# วิธีที่ 1
kubectl -n todo-app port-forward svc/todo-api 8080:80
curl localhost:8080/healthz

# วิธีที่ 2
echo "$(minikube ip) todo.local" | sudo tee -a /etc/hosts
curl http://todo.local/healthz
```

**เส้นทางต่างกันยังไง:**

```
port-forward:  เครื่องคุณ → API server ของ k8s → tunnel ตรงเข้า pod
               (ข้าม Ingress และข้าม Service load balancing ไปเลย)

ingress:       เครื่องคุณ → ingress controller → Service → pod
               (เส้นทางเดียวกับผู้ใช้จริง)
```

**บทเรียนสำคัญ:** port-forward ใช้ debug ได้ดี **แต่ไม่ใช่การทดสอบ**
ถ้า port-forward ได้แต่ ingress ไม่ได้ = ปัญหาอยู่ที่ ingress/Service ไม่ใช่ที่แอป — ข้อมูลนี้มีค่ามาก

---

## K1.3 describe

```bash
kubectl -n todo-app describe pod <pod>
```

| หา | อยู่ตรงไหน |
| --- | --- |
| image | ส่วน `Containers:` → `Image:` |
| node | ส่วนบนสุด `Node:` |
| probe | ส่วน `Containers:` → `Liveness:` / `Readiness:` |
| **Events** | **ท้ายสุดของ output** |

**ทำไม Events ต้องดูก่อนเสมอ:** มันคือบันทึกว่า k8s พยายามทำอะไรและเจอปัญหาอะไร เรียงตามเวลา

```
Normal   Scheduled  assigned todo-app/todo-api-xxx to minikube
Normal   Pulling    Pulling image "ghcr.io/..."
Warning  Failed     Failed to pull image: unauthorized     ← สาเหตุอยู่ตรงนี้
Warning  BackOff    Back-off pulling image
```

⚠️ Events เก็บไว้แค่ 1 ชั่วโมงโดยค่าเริ่มต้น — ถ้าปัญหาเกิดเมื่อวานจะไม่เห็นแล้ว

---

## K1.4 log

```bash
kubectl -n todo-app logs <pod>                 # container หลัก
kubectl -n todo-app logs <pod> -c migrate      # initContainer
kubectl -n todo-app logs <pod> --previous      # container ที่ตายไปแล้ว ⭐
kubectl -n todo-app logs -l app=todo-api --tail=50 --prefix   # ทุก pod พร้อมกัน
```

**`--previous` คือคำสั่งที่ช่วยชีวิต:** ตอน `CrashLoopBackOff` container ปัจจุบันอาจยังไม่ทันพ่น log อะไร
log ที่บอกสาเหตุจริงอยู่ในรอบก่อนหน้าที่ตายไปแล้ว

---

## K1.5 ลบ pod

```bash
kubectl -n todo-app delete pod <pod>
kubectl -n todo-app get pods       # pod ใหม่ขึ้นทันที ชื่อไม่เหมือนเดิม
```

**ใครสร้างให้:**

```
Deployment (บอกว่าอยากได้ 2 replica)
    └─ ReplicaSet (คอยนับว่ามีครบ 2 ไหม)
         └─ Pod, Pod
```

ReplicaSet คือตัวที่นั่งเฝ้าจำนวน pod ตลอดเวลา พอเหลือ 1 มันสร้างเพิ่มทันที
ดูความสัมพันธ์ได้จาก:

```bash
kubectl -n todo-app get rs
kubectl -n todo-app get pod <pod> -o jsonpath='{.metadata.ownerReferences}'
```

**นี่คือแนวคิดหลักของ k8s: เราบอก "สภาพที่ต้องการ" แล้ว controller ทำให้เป็นจริงเอง** ไม่ใช่สั่งเป็นขั้นตอน

---

## K1.6 scale

```bash
kubectl -n todo-app scale deployment/todo-api --replicas=5
kubectl -n todo-app get pods
kubectl -n todo-app scale deployment/todo-api --replicas=2
```

**ถ้ารัน `kubectl apply -k` ซ้ำ:** ค่าจะกลับไปเป็นค่าใน git (2 หรือ 1 ตาม overlay) เพราะ apply บอกว่า "สภาพที่ต้องการคือแบบนี้"

**นี่คือเหตุผลที่ห้าม scale ด้วยมือใน production** — deploy ครั้งถัดไปจะทับค่าที่แก้ไว้ โดยที่มักไม่มีใครจำได้ว่าเคยแก้
(และถ้าใช้ Argo CD ที่เปิด `selfHeal` มันจะดึงกลับภายในไม่กี่วินาทีด้วยซ้ำ)

⚠️ ถ้ามี HPA อยู่ อย่าใส่ `replicas` ใน manifest — สองตัวจะแย่งกันคุมค่าเดียวกัน

---

## K1.7 ConfigMap vs Secret

```bash
kubectl -n todo-app get configmap todo-config -o yaml
kubectl -n todo-app get secret todo-secret -o jsonpath='{.data.DATABASE_URL}' | base64 -d
# postgresql://app:app_password@postgres:5432/tododb?sslmode=disable
```

**เห็นแล้วใช่ไหมว่าถอดง่ายแค่ไหน** — base64 ไม่ใช่การเข้ารหัส มันคือการเข้ารหัสอักขระเฉย ๆ

| | ConfigMap | Secret |
| --- | --- | --- |
| เก็บยังไง | plain text | base64 (ถอดได้ทันที) |
| เข้ารหัสใน etcd | ❌ | ❌ **ไม่ได้เข้ารหัสโดยค่าเริ่มต้น** ต้องเปิด encryption at rest เอง |
| RBAC แยกได้ | ✅ | ✅ (นี่คือความต่างที่แท้จริง) |
| commit ลง git ได้ | ✅ | ❌ **ห้ามเด็ดขาด** |

**ความต่างจริง ๆ ไม่ใช่ base64 แต่คือ:** Secret แยกสิทธิ์ RBAC ได้, ไม่โผล่ใน `kubectl describe`, และมี integration กับ Vault/CSI driver

---

## K1.8 Service กับ Endpoint

```bash
kubectl -n todo-app get svc todo-api
kubectl -n todo-app get endpoints todo-api
# NAME       ENDPOINTS
# todo-api   10.244.0.5:3000,10.244.0.6:3000
```

ทดลองให้ pod หนึ่งไม่ ready:

```bash
kubectl -n todo-app scale statefulset/postgres --replicas=0    # ทำให้ readyz ล้มเหลว
kubectl -n todo-app get pods                                    # READY 0/1
kubectl -n todo-app get endpoints todo-api                      # <none> — หายไปหมด
kubectl -n todo-app scale statefulset/postgres --replicas=1     # กลับมา
```

**นี่คือกลไกที่ทำให้ readinessProbe มีความหมาย:**

```
readinessProbe ไม่ผ่าน → kubelet รายงาน pod not ready
                       → endpoints controller ถอด IP ออกจาก Endpoints
                       → kube-proxy อัปเดตกฎ routing
                       → traffic ไม่ถูกส่งไป pod นั้นอีก
```

Service เป็นแค่ชื่อกับ IP คงที่ — **ตัวที่รู้ว่าต้องส่งไป pod ไหนจริง ๆ คือ Endpoints**
เวลา debug ว่า "ทำไมยิง Service แล้วไม่ถึง pod" ให้ดู `get endpoints` เป็นอันดับแรก ถ้าเป็น `<none>` แปลว่า selector ผิดหรือไม่มี pod ไหน ready เลย

---

## 🎯 ต่อยอด

- `kubectl explain deployment.spec.strategy` — เอกสารในตัวที่หลายคนไม่รู้ว่ามี
- `kubectl get pod <pod> -o yaml` แล้วเทียบกับที่เราเขียน จะเห็นค่า default ที่ k8s เติมให้เพียบ
- ลอง `kubectl -n todo-app exec -it <pod> -- wget -qO- http://todo-api/healthz` (เรียก Service จากใน pod)
