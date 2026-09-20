# 01 — ภาพรวม & สถาปัตยกรรม

## เป้าหมายของโปรเจกต์นี้

แอปตัวนี้ตั้งใจให้ "น่าเบื่อ" ที่สุด — Todo CRUD ธรรมดา ไม่มี business logic ซับซ้อน
เพราะสิ่งที่เราจะฝึกจริง ๆ คือ **ทุกอย่างที่อยู่รอบ ๆ โค้ด**:

```
เขียนโค้ด → build เป็น image → ทดสอบอัตโนมัติ → push ขึ้น registry → deploy → monitor → rollback
     ↑                                                                              │
     └──────────────────────────── feedback loop ←──────────────────────────────────┘
```

## Request วิ่งยังไง

### แบบ Docker Compose

```
Client :8080
   │
   ▼
┌─────────┐   proxy_pass    ┌──────────┐   DATABASE_URL   ┌────────────┐
│  nginx  │ ──────────────▶ │ api x2   │ ───────────────▶ │ postgres   │
│  :80    │  + rate limit   │ :3000    │                  │ :5432      │
└─────────┘  + load balance └──────────┘                  └────────────┘
                                                            volume: pgdata
```

Docker DNS ทำให้ชื่อ `api` คืน IP ของทุก replica — nginx เลยกระจาย request ให้เองโดยไม่ต้อง config อะไรเพิ่ม
และ `db` คือชื่อ service ที่ใช้เป็น hostname ใน `DATABASE_URL`

### แบบ Kubernetes

```
Client
   │
   ▼
┌──────────────────┐    ┌─────────────┐    ┌──────────────────┐    ┌──────────────┐
│ Ingress          │───▶│ Service     │───▶│ Deployment       │───▶│ StatefulSet  │
│ (ingress-nginx)  │    │ todo-api    │    │ todo-api (3 pod) │    │ postgres     │
│ host: todo.local │    │ ClusterIP   │    │ + HPA 2-6        │    │ + PVC        │
└──────────────────┘    └─────────────┘    └──────────────────┘    └──────────────┘
```

จะเห็นว่าหน้าที่เดิม ๆ ถูกย้ายไปคนละที่:

| หน้าที่                       | Compose                   | Kubernetes                 |
| ----------------------------- | ------------------------- | -------------------------- |
| รับ traffic เข้า              | nginx container           | Ingress                    |
| Load balance                  | nginx upstream            | Service (kube-proxy)       |
| Restart เมื่อแอปพัง           | `restart: unless-stopped` | livenessProbe              |
| ไม่ส่ง traffic ตอนยังไม่พร้อม | `depends_on: healthy`     | readinessProbe             |
| เก็บข้อมูล DB                 | named volume              | PVC / volumeClaimTemplates |
| ตั้งค่า / ความลับ             | `.env`                    | ConfigMap / Secret         |
| เพิ่มจำนวน instance           | `deploy.replicas`         | `spec.replicas` + HPA      |

เข้าใจตารางนี้ = เข้าใจ 70% ของการย้ายจาก Compose ไป K8s

## ทำไมต้องมี `/healthz` และ `/readyz` แยกกัน

ตรงนี้คนพลาดกันเยอะ:

- **`/healthz` (liveness)** — "process ยังไม่ค้างใช่ไหม" ถ้าไม่ผ่าน → k8s **ฆ่า pod แล้วสร้างใหม่**
  ห้ามเช็ค DB ที่นี่! ไม่งั้น DB ล่มทีเดียว pod ทั้งหมดจะ restart วนไม่จบ (restart ก็ไม่ช่วยอะไรอยู่ดี)
- **`/readyz` (readiness)** — "พร้อมรับงานไหม" ถ้าไม่ผ่าน → k8s **แค่ถอด pod ออกจาก Service** ไม่ฆ่า
  ตรงนี้เช็ค dependency ได้เต็มที่ พอ DB กลับมา pod ก็กลับเข้า pool เอง

## Roadmap ที่แนะนำ

1. รันในเครื่องให้ได้ก่อน → [02](02-local-development.md)
2. ห่อเป็น image → [03](03-docker.md)
3. ต่อหลาย service เข้าด้วยกัน → [04](04-docker-compose.md)
4. เอา proxy มาคั่นหน้า → [05](05-nginx.md)
5. ให้เครื่องอื่นตรวจโค้ดแทนเรา → [06](06-github-actions-ci.md)
6. ให้เครื่องอื่น build + ส่งของแทนเรา → [07](07-cd-ghcr.md)
7. ย้ายไป orchestrator ของจริง → [08](08-kubernetes.md)
8. ขึ้น server จริงแบบฟรี + scale ง่าย ๆ → [10](10-deploy-free-cloud.md)

ทำทีละขั้น อย่าข้าม — แต่ละขั้นตอบคำถามที่ขั้นก่อนหน้าทำให้เกิด

**อยากเห็นภาพรวมทั้งเส้นทางการเรียน ตั้งแต่ศูนย์ถึง deploy จริง?** ดู [00 — Roadmap การเรียนรู้](00-roadmap.md)

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] วาดตารางหน้า 57 ใหม่ด้วยลายมือตัวเอง (Compose ↔ K8s) โดยไม่เปิดดู — เช็คว่าจำได้กี่ช่อง
- [ ] เปิด [08](08-kubernetes.md) แล้วหา 3 อย่างที่ตารางนี้ยังไม่ได้พูดถึง
- [ ] อธิบายให้เพื่อนฟังใน 2 นาทีว่าทำไม `/healthz` กับ `/readyz` ต้องแยกกัน โดยไม่เปิดเอกสาร
- [ ] ลองนึกว่าถ้า `/readyz` เช็ค DB แต่ `/healthz` ก็เช็ค DB ด้วย จะเกิดอะไรขึ้นตอน DB ล่ม

➡️ ต่อไป: [02 — รันในเครื่อง](02-local-development.md)
