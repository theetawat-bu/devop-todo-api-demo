# เฉลย — ☸️ Kubernetes ระดับ 5

⬅️ [กลับไปที่โจทย์](../../kubernetes/05-expert.md)

> ระดับนี้เป็นการออกแบบและตัดสินใจ — ด้านล่างคือแนวคำตอบและเกณฑ์ประเมิน

---

## K5.1 SLO และ error budget

**นิยาม SLO:**

```
SLI:  สัดส่วน request ที่สำเร็จ (ไม่ใช่ 5xx) และตอบภายใน 300ms
SLO:  99.5% ใน 30 วัน
Error budget: 0.5% = ประมาณ 3 ชั่วโมง 39 นาทีต่อเดือน
```

**alert แบบ multi-window burn rate (วิธีที่ Google SRE แนะนำ):**

| ความเร็วการเผา | หน้าต่างเวลา | ใช้ budget หมดใน | ระดับ |
| --- | --- | --- | --- |
| 14.4× | 1 ชม. | 2 วัน | 🔴 ปลุกคนตอนตีสาม |
| 6× | 6 ชม. | 5 วัน | 🟠 ปลุกคนตอนตีสาม |
| 3× | 1 วัน | 10 วัน | 🟡 แค่แจ้งเตือน |
| 1× | 3 วัน | 30 วัน | 🟢 บันทึกไว้ |

```promql
(1 - (sum(rate(http_requests_total{status!~"5.."}[1h])) / sum(rate(http_requests_total[1h])))) > 14.4 * 0.005
```

**ทำไม alert ที่ดังทุกครั้งที่มี 5xx ถึงไร้ประโยชน์ภายในสองสัปดาห์:**

1. ระบบจริงมี 5xx เล็กน้อยตลอดเวลา (bot, client แปลก ๆ, timeout ชั่วคราว) → alert ดังทุกวัน
2. คนเริ่มเมิน → alert fatigue
3. วันที่ดังเพราะเรื่องจริง ไม่มีใครสนใจแล้ว

**หลักการ: alert ต้องดังเมื่อ "ต้องมีคนลงมือทำอะไรบางอย่างตอนนี้" เท่านั้น** ถ้าดังแล้วไม่ต้องทำอะไร มันไม่ควรเป็น alert

**เกณฑ์ผ่าน:** มี dashboard แสดง budget ที่เหลือ + alert ที่อ้างอิง burn rate ไม่ใช่จำนวน error ดิบ

---

## K5.2 chaos engineering

```yaml
# chaos-mesh: ฆ่า pod สุ่ม
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
spec:
  action: pod-kill
  mode: one
  selector: { namespaces: [todo-app], labelSelectors: { app: todo-api } }
  scheduler: { cron: "@every 5m" }
```

```yaml
# หน่วง network 500ms
kind: NetworkChaos
spec:
  action: delay
  delay: { latency: "500ms" }
  selector: { namespaces: [todo-app] }
```

**สิ่งที่มักค้นพบ (และมีค่าที่สุด):**

| การทดลอง | สิ่งที่คนคิด | สิ่งที่มักเกิดจริง |
| --- | --- | --- |
| ฆ่า pod สุ่ม | "ไม่เป็นไร มี replica" | request ที่ค้างขาด ถ้า preStop/SIGTERM ไม่ครบ |
| หน่วง network 500ms | "แค่ช้าลง" | connection pool เต็ม → **ระบบล่มทั้งหมด** ทั้งที่ไม่มีอะไรพัง |
| DB ช้า | "readiness ถอด pod ออก" | ถ้า probe timeout สั้นเกิน → pod ทั้งหมดถูกถอดพร้อมกัน = ล่ม 100% |

**การค้นพบข้อ 2 คือเหตุผลที่ chaos engineering มีอยู่** — ความช้าอันตรายกว่าความพัง
เพราะระบบมีกลไกรับมือ "พัง" (health check, retry) แต่ไม่มีกลไกรับมือ "ช้าแต่ยังตอบ"

**กฎการทำ chaos ที่ปลอดภัย:** ตั้งสมมติฐานก่อน → ทดลอง → เทียบผลกับสมมติฐาน
ถ้าไม่ตั้งสมมติฐานล่วงหน้า มันไม่ใช่การทดลอง แต่คือการทำระบบพังเล่น

---

## K5.3 กู้คืนฐานข้อมูล

```bash
# สำรอง
kubectl -n todo-app exec postgres-0 -- pg_dump -U app tododb > backup.sql

# ทำลาย
kubectl -n todo-app delete statefulset postgres
kubectl -n todo-app delete pvc data-postgres-0

# กู้
kubectl apply -k k8s/overlays/dev
kubectl -n todo-app exec -i postgres-0 -- psql -U app tododb < backup.sql
```

| ตัวชี้วัด | ความหมาย | ตัวอย่างที่ควรได้ |
| --- | --- | --- |
| **RTO** | ใช้เวลากู้เท่าไรจนใช้งานได้ | 12 นาที |
| **RPO** | ข้อมูลหายย้อนหลังกี่นาที | 24 ชม. (ถ้า backup วันละครั้ง) |

**คำถามที่ต้องตอบ: ธุรกิจรับ RPO 24 ชั่วโมงได้ไหม**
สำหรับระบบที่มีธุรกรรมทางการเงิน คำตอบคือ **ไม่ได้** → ต้องมี WAL archiving หรือ streaming replication เพื่อให้ RPO เหลือระดับวินาที

**ข้อที่ทีมส่วนใหญ่พลาด:** มี backup แต่**ไม่เคยลองกู้** จนวันที่ต้องใช้จริงถึงรู้ว่าไฟล์เสีย หรือกู้แล้วข้อมูลไม่ครบ
**backup ที่ไม่เคยทดสอบกู้ ไม่นับว่ามี backup**

---

## K5.4 policy

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: baseline-security
spec:
  validationFailureAction: Enforce
  rules:
    - name: require-resources
      match: { any: [{ resources: { kinds: [Pod] } }] }
      validate:
        message: "ทุก container ต้องระบุ resources.requests และ limits — ดู wiki/resource-guide"
        pattern:
          spec:
            containers:
              - resources:
                  requests: { memory: "?*", cpu: "?*" }
                  limits: { memory: "?*" }
    - name: disallow-latest
      match: { any: [{ resources: { kinds: [Pod] } }] }
      validate:
        message: "ห้ามใช้ tag :latest — ใช้ digest หรือเวอร์ชันที่ระบุได้"
        pattern:
          spec:
            containers:
              - image: "!*:latest"
    - name: allowed-registries
      match: { any: [{ resources: { kinds: [Pod] } }] }
      validate:
        message: "ดึง image ได้เฉพาะจาก harbor.company.internal"
        pattern:
          spec:
            containers:
              - image: "harbor.company.internal/*"
```

**ลำดับการเปิดใช้ที่ปลอดภัย:**

```
1. validationFailureAction: Audit  →  ดู PolicyReport ว่ามีอะไรผิดกฎบ้าง
2. แจ้งทีมที่เกี่ยวข้อง พร้อมกำหนดเส้นตาย
3. ยกเว้นชั่วคราวสำหรับ workload เก่าที่แก้ไม่ทัน (พร้อมวันหมดอายุ)
4. เปลี่ยนเป็น Enforce
```

**เปิด Enforce วันแรกเลย = ทีมอื่น deploy ไม่ได้กะทันหัน** แล้วงานความปลอดภัยจะกลายเป็นศัตรูของทุกคน
ซึ่งจบลงด้วยการที่ใครสักคนไปปิด policy ทิ้งทั้งหมด — แย่กว่าไม่เคยมีเลย

---

## K5.5 อัปเกรดคลัสเตอร์

```bash
# 1) หา API ที่ถูก deprecated ก่อน
pluto detect-files -d ./k8s
kubectl-convert -f manifest.yaml --output-version apps/v1

# 2) control plane ก่อนเสมอ
# 3) node ทีละตัว
kubectl drain node-1 --ignore-daemonsets --delete-emptydir-data
# อัปเกรด node
kubectl uncordon node-1
```

**ลำดับที่ห้ามสลับ:** control plane ต้องขึ้นก่อน node เสมอ
k8s รองรับ kubelet ที่เก่ากว่า API server ได้ (version skew) แต่ไม่รองรับ kubelet ที่ใหม่กว่า

**เช็กลิสต์ที่ต้องมีในแผน:**

- [ ] ตรวจ API ที่หายไปในเวอร์ชันใหม่ (pluto/kubent)
- [ ] ตรวจว่า CNI, CSI, ingress controller รองรับเวอร์ชันใหม่หรือยัง
- [ ] ทุก workload มี PDB (ไม่งั้น drain จะทำให้ downtime)
- [ ] มีความจุเหลือพอให้ pod ย้ายตอน drain
- [ ] ทดสอบบนคลัสเตอร์ทดสอบก่อน
- [ ] **เกณฑ์ยกเลิก** — เงื่อนไขอะไรที่จะทำให้หยุดแล้วถอย
- [ ] ทำนอกเวลาที่มีคนใช้เยอะ + แจ้งล่วงหน้า

**ข้อที่คนลืมบ่อยที่สุดคือ PDB** — drain node ที่ไม่มี PDB จะไล่ pod ทั้งหมดพร้อมกันทันที

---

## K5.6 มัลติเทนแนนซี

```yaml
apiVersion: v1
kind: ResourceQuota
metadata: { name: team-a-quota, namespace: team-a }
spec:
  hard:
    requests.cpu: "10"
    requests.memory: 20Gi
    limits.cpu: "20"
    limits.memory: 40Gi
    persistentvolumeclaims: "10"
    count/pods: "50"
---
apiVersion: v1
kind: LimitRange
metadata: { name: team-a-limits, namespace: team-a }
spec:
  limits:
    - type: Container
      default: { cpu: 500m, memory: 512Mi }
      defaultRequest: { cpu: 100m, memory: 128Mi }
      max: { cpu: "4", memory: 8Gi }
```

**ทำไมต้องมีทั้งสองอย่าง:**

| | ควบคุมอะไร |
| --- | --- |
| ResourceQuota | เพดาน**รวม**ของทั้ง namespace |
| LimitRange | ค่า**ต่อ container** + เติมค่า default ให้ pod ที่ไม่ได้ระบุ |

LimitRange สำคัญเพราะ **ResourceQuota จะปฏิเสธ pod ที่ไม่ระบุ resources เลย** — LimitRange เติม default ให้ทำให้ไม่ต้องบังคับทุกทีมแก้ manifest พร้อมกัน

**ชั้นที่ต้องมีให้ครบ:**

| ชั้น | เครื่องมือ | กันอะไร |
| --- | --- | --- |
| ทรัพยากร | ResourceQuota + LimitRange | ทีมหนึ่งกินหมด |
| เครือข่าย | NetworkPolicy (default-deny) | ทีมหนึ่งยิงเข้า service อีกทีม |
| สิทธิ์ | RBAC ต่อ namespace | แก้ของทีมอื่น |
| GitOps | Argo AppProject | deploy ข้าม namespace |
| node | taint/toleration | workload สำคัญโดนแย่ง node |

**ความจริงที่ต้องยอมรับ:** k8s namespace เป็น **soft multi-tenancy** — ไม่ได้แยก kernel
ถ้าต้องการแยกจริงในระดับความปลอดภัยสูง (เช่นรันโค้ดของลูกค้า) ต้องใช้คลัสเตอร์แยก หรือ sandboxed runtime (gVisor, Kata)

---

## K5.7 วิเคราะห์ต้นทุน

```promql
# ใช้จริง p95 ย้อนหลัง 7 วัน
quantile_over_time(0.95, container_memory_working_set_bytes{pod=~"todo-api.*"}[7d])
# เทียบกับที่จอง
kube_pod_container_resource_requests{resource="memory"}
```

**หลักการตั้งค่า:**

| ค่า | ตั้งเท่าไร | เพราะ |
| --- | --- | --- |
| `requests.cpu` | ~p50 ของการใช้จริง | scheduler ใช้ตัวนี้จองที่ ตั้งสูงเกินคือจองที่ทิ้งไว้เปล่า ๆ |
| `requests.memory` | ~p95 | memory คืนยาก ต้องเผื่อ |
| `limits.memory` | p99 + 30% | เกินแล้วตาย ต้องเผื่อมาก |
| `limits.cpu` | มักไม่ตั้ง | ให้ใช้ CPU ว่างของ node ได้ |

**ทำไมใช้ p95 ไม่ใช่ค่าเฉลี่ย:** ค่าเฉลี่ยถูกกดโดยช่วงกลางคืนที่ไม่มีคนใช้
ตั้งตามค่าเฉลี่ยแล้วจะไม่พอในช่วงพีค — ซึ่งเป็นช่วงที่พังแล้วเจ็บที่สุด

**อันตรายทั้งสองทาง:**

| ตั้งต่ำเกิน | ตั้งสูงเกิน |
| --- | --- |
| pod ถูก schedule ลง node ที่ทรัพยากรไม่พอจริง | จองที่ไว้แต่ไม่ใช้ = จ่ายค่า node ฟรี |
| ตอนโหลดสูงแย่งทรัพยากรกับ pod อื่น | node เต็มเร็ว ต้องซื้อเพิ่มโดยไม่จำเป็น |
| QoS class ต่ำ → ถูกไล่ออกก่อนใครตอน node ขาดแคลน | HPA คำนวณผิด (scale ช้าเพราะ % ดูต่ำ) |

**เกร็ด QoS:** pod ที่ `requests == limits` ทุก resource จะได้ QoS `Guaranteed` = **ถูกไล่ออกเป็นลำดับสุดท้าย**
ใช้กับ workload ที่สำคัญที่สุดได้

---

## K5.8 runbook

โครงที่ผ่านเกณฑ์:

```markdown
# Runbook: API ตอบ 5xx เกิน 5%

## อาการ: alert `TodoApiHighErrorRate` ดัง
## ผลกระทบ: ผู้ใช้บางส่วนใช้งานไม่ได้

## ขั้นตอนตรวจสอบ (ทำตามลำดับ)
1. ดูภาพรวม: [ลิงก์ Grafana dashboard]
2. error มาจาก pod เดียวหรือทุก pod?
   kubectl -n todo-app logs -l app=todo-api --tail=100 | grep -i error
   - pod เดียว → ไปข้อ 3
   - ทุก pod  → ไปข้อ 4
3. [pod เดียว] ลบ pod นั้นทิ้ง: kubectl delete pod <pod>  → จบ
4. เพิ่ง deploy ไปหรือเปล่า?
   kubectl -n todo-app rollout history deployment/todo-api
   - ใช่ → rollback (ดู runbook-rollback) → จบ
   - ไม่ → ไปข้อ 5
5. DB ปกติไหม? curl /readyz ; kubectl -n todo-app logs postgres-0 --tail=50
   - DB มีปัญหา → ไป runbook-database
6. ยังไม่พบสาเหตุ → escalate ไป [ทีม/ช่องทาง] พร้อมข้อมูลจากข้อ 1-5

## สิ่งที่ห้ามทำ
- ห้าม kubectl edit แก้ค่าโดยตรง (Argo จะดึงกลับ + ทำให้ git ไม่ตรงคลัสเตอร์)
- ห้าม scale เพิ่มโดยไม่รู้สาเหตุ (ถ้าปัญหาอยู่ที่ DB จะยิ่งแย่)

## หลังเหตุการณ์สงบ: เขียน postmortem ภายใน 48 ชม.
```

**เกณฑ์ผ่านมีข้อเดียว: ให้คนที่ไม่ได้เขียนทำตามแล้วแก้ปัญหาได้จริง**

ลักษณะของ runbook ที่ใช้ไม่ได้:

- มีแต่ทฤษฎี ไม่มีคำสั่งที่ก็อปวางได้
- ไม่มีเงื่อนไขแตกแขนง (ถ้า A ทำแบบนี้ ถ้า B ทำแบบนั้น)
- ไม่บอกว่าเมื่อไรควรขอความช่วยเหลือ
- ไม่มีส่วน "ห้ามทำ" — ซึ่งมักสำคัญกว่าส่วน "ให้ทำ" ตอนตีสาม

---

## K5.9 postmortem

```markdown
# Postmortem: API ล่ม 23 นาที (2026-08-10)

## สรุป
migration ที่ล็อกตารางนานทำให้ readiness ไม่ผ่านทุก pod → traffic ถูกปฏิเสธ 23 นาที

## ผลกระทบ
- ผู้ใช้ 4,200 คนได้ 503
- request ล้มเหลว 18,000 รายการ
- error budget ของเดือนถูกใช้ไป 34%

## Timeline
14:02  deploy v1.4.0 พร้อม migration เพิ่ม index
14:03  migration ล็อกตาราง todos
14:04  readiness ล้มเหลวทุก pod → endpoints ว่าง
14:05  alert ดัง
14:11  คนแรกรับเรื่อง (6 นาทีหลัง alert)
14:19  ระบุสาเหตุได้จาก log ของ postgres
14:25  ยกเลิก migration
14:26  ระบบกลับมาปกติ

## สาเหตุราก (5 whys)
ทำไม 503? → pod ทั้งหมด not ready
ทำไม not ready? → /readyz ค้างเพราะ query ถูกล็อก
ทำไมถูกล็อก? → CREATE INDEX แบบไม่มี CONCURRENTLY
ทำไมไม่มี CONCURRENTLY? → ไม่มีการ review migration ก่อน merge
ทำไมไม่มี review? → **ไม่มีขั้นตอนที่บังคับ review migration แยกจาก code review**

## สิ่งที่ทำงานได้ดี
- alert ดังภายใน 1 นาที
- rollback ใช้เวลาแค่ 1 นาทีหลังระบุสาเหตุได้

## สิ่งที่ควรปรับปรุง
- ใช้เวลา 6 นาทีกว่าจะมีคนรับเรื่อง
- ใช้เวลา 8 นาทีในการหาสาเหตุ เพราะ log ของ postgres ไม่ได้อยู่ใน dashboard เดียวกัน

## Action items
| ทำอะไร | ใคร | เมื่อไร |
|---|---|---|
| เพิ่ม CI check บังคับ CONCURRENTLY ใน CREATE INDEX | @a | 8/20 |
| ย้าย migration ไป PreSync hook แยกจาก readiness path | @b | 8/25 |
| เพิ่ม lock monitoring ของ postgres เข้า dashboard หลัก | @c | 8/22 |
| ทบทวนเส้นทาง on-call ให้รับเรื่องได้ใน 3 นาที | @d | 9/1 |
```

**เกณฑ์ผ่าน:** ทุก action item เป็นการ**แก้ระบบ** ไม่ใช่ "ต้องระวังมากขึ้น" หรือ "อบรมทีม"
เพราะความระมัดระวังของมนุษย์ไม่ scale และจะหายไปภายในหนึ่งเดือน

**หลักการไม่โทษคน:** สมมติว่าทุกคนทำดีที่สุดแล้วด้วยข้อมูลที่มีในตอนนั้น
ถ้าคนทำผิดพลาดได้ง่าย แปลว่า**ระบบออกแบบให้ผิดพลาดได้ง่าย** — นั่นคือสิ่งที่ต้องแก้

---

## 🎯 ประเมินตัวเอง

- [ ] alert ทุกตัวมีเหตุผลของ threshold และมี runbook คู่กัน
- [ ] เคยทำระบบพังเองแบบมีการควบคุม และเรียนรู้อะไรที่ไม่คาดคิด
- [ ] กู้ข้อมูลได้จริงพร้อมตัวเลข RTO/RPO
- [ ] policy ที่ตั้งไว้ Enforce จริง ไม่ใช่แค่ Audit
- [ ] runbook ผ่านการทดสอบโดยคนอื่นแล้ว
