# เฉลย — ☸️ Kubernetes ระดับ 2

⬅️ [กลับไปที่โจทย์](../../kubernetes/02-intermediate.md)

---

## K2.1 readiness ไม่ผ่าน

```bash
kubectl -n todo-app scale statefulset/postgres --replicas=0
kubectl -n todo-app get pods -w
# todo-api-xxx   0/1   Running   0   5m      ← Running แต่ไม่ ready, RESTARTS = 0
kubectl -n todo-app get endpoints todo-api
# <none>
```

**สิ่งที่ต้องสังเกต 3 อย่าง:**

| | ค่า | ความหมาย |
| --- | --- | --- |
| STATUS | `Running` | container ยังทำงานอยู่ ไม่ถูกฆ่า |
| READY | `0/1` | ไม่พร้อมรับ traffic |
| RESTARTS | `0` | **ไม่มีการ restart เลย** |

**ทำไมพฤติกรรมนี้ถูกต้อง:** DB ล่มไม่ใช่ความผิดของ pod นี้ การ restart ไม่ทำให้ DB กลับมา
สิ่งที่ควรทำคือ "หยุดส่งงานให้ชั่วคราว" แล้วรอ — พอ DB กลับมา readiness ผ่านเอง pod กลับเข้า pool เอง ไม่ต้องมีใครแตะ

```bash
kubectl -n todo-app scale statefulset/postgres --replicas=1
# รอสักครู่ → READY 1/1 เอง
```

---

## K2.2 liveness ไม่ผ่าน

```yaml
livenessProbe:
  httpGet: { path: /ไม่มีจริง, port: http }
```

```bash
kubectl -n todo-app get pods -w
# todo-api-xxx  1/1  Running            0   30s
# todo-api-xxx  1/1  Running            1   60s     ← RESTARTS เพิ่ม
# todo-api-xxx  0/1  CrashLoopBackOff   3   3m
```

```bash
kubectl -n todo-app describe pod <pod> | grep -A3 "Liveness"
# Warning  Unhealthy  Liveness probe failed: HTTP probe failed with statuscode: 404
# Normal   Killing    Container api failed liveness probe, will be restarted
```

**เทียบกับ K2.1 ให้ชัด:**

| | readiness ล้มเหลว | liveness ล้มเหลว |
| --- | --- | --- |
| k8s ทำอะไร | ถอดออกจาก Service | **ฆ่าแล้วสร้างใหม่** |
| RESTARTS | 0 | เพิ่มเรื่อย ๆ |
| ฟื้นเองได้ไหม | ✅ พอเงื่อนไขกลับมาปกติ | เฉพาะถ้า restart ช่วยได้จริง |
| ความรุนแรง | ต่ำ | **สูง** |

`CrashLoopBackOff` คือ k8s ถ่วงเวลาการ restart เพิ่มขึ้นเรื่อย ๆ (10s → 20s → 40s → … สูงสุด 5 นาที)
เพื่อไม่ให้เปลืองทรัพยากรกับการ restart สิ่งที่ไม่มีวันสำเร็จ

---

## K2.3 anti-pattern

```yaml
livenessProbe:
  httpGet: { path: /readyz, port: http } # ❌ เช็ค DB ใน liveness
```

```bash
kubectl -n todo-app scale statefulset/postgres --replicas=0
kubectl -n todo-app get pods -w
# ทุก pod เข้า CrashLoopBackOff พร้อมกันภายในไม่กี่นาที
```

**ตอบคำถาม: ตอน DB กลับมา ระบบจะฟื้นช้าลง เพราะ**

1. pod ทั้งหมดอยู่ใน backoff — ต้องรอถึง 5 นาทีกว่าจะลอง restart รอบถัดไป
2. พอ restart พร้อมกันทั้งหมด ทุกตัวเปิด connection pool ใหม่พร้อมกัน → **thundering herd** ถล่ม DB ที่เพิ่งฟื้น
3. DB ที่โดนถล่มอาจตอบช้าจน probe ไม่ผ่านอีก → วนซ้ำ

**เทียบกับกรณีที่ทำถูก (K2.1):** pod ไม่เคยตาย connection pool ยังอยู่ พอ DB กลับมา readiness ผ่านทันทีในรอบถัดไป (10 วินาที) และ traffic กลับมาไหลทันที

> นี่คือเหตุผลที่โปรเจกต์นี้แยก `/healthz` (ไม่แตะ DB) ออกจาก `/readyz` (แตะ DB) ตั้งแต่บรรทัดแรกของโค้ด

---

## K2.4 rolling update

```bash
# terminal 1
while true; do curl -s -o /dev/null -w "%{http_code}\n" http://todo.local/healthz; sleep 0.2; done | sort | uniq -c
# terminal 2
kubectl -n todo-app set image deployment/todo-api api=<image ใหม่>
kubectl -n todo-app rollout status deployment/todo-api
```

ผลที่ต้องได้: **200 ทั้งหมด**

**องค์ประกอบ 4 อย่างที่ทำให้ไม่มี downtime — ขาดข้อใดข้อหนึ่งก็เจอ error:**

| องค์ประกอบ | ถ้าไม่มีจะเจอ |
| --- | --- |
| `maxUnavailable: 0` | ความจุลดลงระหว่าง deploy |
| `readinessProbe` ที่ตรวจจริง | traffic เข้า pod ที่ยังไม่พร้อม → 502 |
| **แอปดัก SIGTERM** | request ที่ค้างถูกตัดกลางคัน |
| `terminationGracePeriodSeconds` พอ | โดน SIGKILL ก่อนปิดงานเสร็จ |

**ถ้ายังเจอ 502 อยู่** มักเป็นเพราะช่องว่างเล็ก ๆ นี้: k8s ส่ง SIGTERM กับการถอด pod ออกจาก endpoints เกิด**พร้อมกัน** ไม่ได้เรียงกัน
ทำให้มี traffic ที่ถูกส่งมาหลัง SIGTERM เล็กน้อย ทางแก้มาตรฐาน:

```yaml
lifecycle:
  preStop:
    exec:
      command: ["sh", "-c", "sleep 5"] # รอให้ endpoints อัปเดตทั่วถึงก่อนค่อยเริ่มปิด
```

---

## K2.5 maxUnavailable

| config | พฤติกรรม | ความจุระหว่าง deploy |
| --- | --- | --- |
| `maxSurge: 1, maxUnavailable: 0` | สร้างใหม่ก่อน แล้วค่อยฆ่าเก่า | **100%+** |
| `maxSurge: 0, maxUnavailable: 1` | ฆ่าเก่าก่อน แล้วค่อยสร้างใหม่ | **ลดลงเหลือ 50%** (ถ้ามี 2 replica) |

**สิ่งที่แลกกัน:**

- `maxSurge` สูง = ต้องมีทรัพยากรว่างเพิ่ม (ชั่วคราว pod มากกว่าปกติ) แต่ deploy เร็วและไม่เสียความจุ
- `maxUnavailable` สูง = ไม่ต้องใช้ทรัพยากรเพิ่ม แต่ความจุลดลงระหว่าง deploy → ถ้า deploy ตอนโหลดสูงจะเจอ latency พุ่ง

**คำแนะนำ:** production ใช้ `maxSurge: 1, maxUnavailable: 0` เสมอ ถ้าทรัพยากรไม่พอสำหรับ pod เพิ่ม 1 ตัว แปลว่าคลัสเตอร์คับเกินไปตั้งแต่แรกแล้ว

---

## K2.6 rollback

```bash
kubectl -n todo-app set image deployment/todo-api api=ghcr.io/ไม่มีจริง:v99
kubectl -n todo-app rollout status deployment/todo-api --timeout=60s   # ค้าง แล้ว timeout

kubectl -n todo-app rollout history deployment/todo-api
kubectl -n todo-app rollout undo deployment/todo-api
kubectl -n todo-app rollout status deployment/todo-api                 # กลับมาปกติ
```

**สังเกตสิ่งที่ดี:** ระหว่างที่ image ใหม่ pull ไม่ได้ **pod เก่ายังทำงานอยู่ครบ** — ผู้ใช้ไม่ได้รับผลกระทบเลย
เพราะ `maxUnavailable: 0` บอกว่าห้ามฆ่าตัวเก่าจนกว่าตัวใหม่จะพร้อม (ซึ่งไม่มีวันพร้อม) → deploy ค้างแต่ระบบไม่ล่ม

**นี่คือเหตุผลที่ `rollout status --timeout` สำคัญมากใน CI** — ถ้าไม่มี workflow จะขึ้นเขียวทั้งที่ deploy ไม่สำเร็จ

**`revisionHistoryLimit: 5`** = เก็บ ReplicaSet เก่าไว้ 5 ตัว → ย้อนได้ 5 เวอร์ชัน
ตั้งสูงเกินไปจะมี ReplicaSet เก่ารกคลัสเตอร์ ตั้งต่ำเกินไปย้อนได้ไม่ไกลพอ

---

## K2.7 overlay ใหม่

```yaml
# k8s/overlays/staging/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: todo-staging
resources:
  - ../../base
patches:
  - target: { kind: Deployment, name: todo-api }
    patch: |-
      - op: replace
        path: /spec/replicas
        value: 2
  - target: { kind: Ingress, name: todo-ingress }
    patch: |-
      - op: replace
        path: /spec/rules/0/host
        value: todo-staging.local
  - target: { kind: Namespace, name: todo-app }
    patch: |-
      - op: replace
        path: /metadata/name
        value: todo-staging
```

```bash
kubectl kustomize k8s/overlays/staging     # ตรวจก่อน
kubectl apply -k k8s/overlays/staging
```

**หลักการที่ต้องยึด:** overlay ควรมีแค่ **ส่วนต่าง** ถ้าต้องก็อป YAML ทั้งไฟล์มาแก้ = ใช้ kustomize ผิดวิธี
เพราะสุดท้าย base กับ overlay จะหลุดจากกัน แล้วเกิดอาการ "prod พังเพราะ config ต่างจาก staging"

---

## K2.8 diff

```bash
kubectl diff -k k8s/overlays/dev
# - replicas: 1
# + replicas: 3
```

**ทำไมสำคัญใน production:**

- เห็นว่าจะเปลี่ยนอะไร**ก่อน**ที่จะเปลี่ยนจริง
- จับได้ว่ามีใครแก้ด้วยมือแล้วยังไม่ได้ commit (diff จะโชว์สิ่งที่ไม่คาดคิด)
- ใส่ใน CI เป็น step ให้ reviewer เห็นใน PR ได้

**ควรทำให้เป็นนิสัย:** `kubectl diff` ก่อน `kubectl apply` ทุกครั้งใน production เหมือนที่ `nginx -t` ก่อน reload

---

## K2.9 ข้อมูลไม่หาย

```bash
curl -X POST http://todo.local/api/todos -H 'Content-Type: application/json' -d '{"title":"ทดสอบ"}'
kubectl -n todo-app delete pod postgres-0
kubectl -n todo-app get pods -w              # postgres-0 กลับมาชื่อเดิม
curl http://todo.local/api/todos             # ข้อมูลยังอยู่
```

**ทำไมชื่อเหมือนเดิม:** StatefulSet ให้ชื่อแบบมีลำดับคงที่ (`postgres-0`, `postgres-1`) ต่างจาก Deployment ที่สุ่มท้ายชื่อ
และ PVC ผูกกับ**ชื่อ pod** ไม่ใช่กับตัว pod → pod ใหม่ชื่อเดิมได้ดิสก์ก้อนเดิมกลับมา

```bash
kubectl -n todo-app get pvc
# data-postgres-0   Bound   pvc-xxx   2Gi   RWO
```

**ถ้าลบ StatefulSet ทิ้ง PVC จะหายไหม → ไม่หาย** (เป็นการออกแบบที่ตั้งใจ เพื่อกันข้อมูลหายจากอุบัติเหตุ)

```bash
kubectl -n todo-app delete statefulset postgres
kubectl -n todo-app get pvc                  # ยังอยู่!
kubectl -n todo-app delete pvc data-postgres-0   # ต้องลบเองถึงจะหาย
```

**บทเรียน:** ถ้าเจอปัญหา "ลบแล้วสร้างใหม่แต่ข้อมูลเก่ายังอยู่/รหัสผ่านไม่เปลี่ยน" — ต้นเหตุคือ PVC เก่าที่ยังค้างอยู่

---

## 🎯 ต่อยอด

- ลองใส่ `preStop` hook แล้ววัดว่าจำนวน 502 ระหว่าง deploy ลดลงจริงไหม
- ลอง `kubectl rollout pause` / `resume` — ใช้ตอนอยากหยุด deploy กลางคันเพื่อดูผล
- อ่าน `kubectl -n todo-app get rs` แล้วดูว่า ReplicaSet เก่าถูกเก็บไว้กี่ตัว
