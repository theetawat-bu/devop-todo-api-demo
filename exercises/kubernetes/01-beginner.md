# ☸️ Kubernetes — ระดับ 1: เริ่มต้น

> **เป้าหมาย:** deploy แอปขึ้นคลัสเตอร์ในเครื่องได้ และใช้คำสั่งสำรวจเป็น
> **ต้องมีก่อน:** มีคลัสเตอร์ในเครื่อง (minikube / kind / Docker Desktop)
> **เวลาโดยประมาณ:** 2–3 ชั่วโมง
> **อ่านประกอบ:** [docs/08 — Kubernetes](../../docs/08-kubernetes.md)

```bash
minikube start --cpus=2 --memory=4096
minikube addons enable ingress
minikube addons enable metrics-server
```

---

### K1.1 deploy ครั้งแรก

**โจทย์:** deploy overlay `dev` แล้วทำให้ pod ทุกตัวขึ้นเป็น Running
**คำใบ้:** `kubectl apply -k k8s/overlays/dev` แล้ว `kubectl -n todo-app get pods -w`
**ผ่านเมื่อ:** `kubectl -n todo-app get all` เห็น Deployment, StatefulSet, Service ครบและ pod พร้อม

---

### K1.2 เข้าใช้งานแอป

**โจทย์:** เรียก `/healthz` ให้ได้ 2 วิธี — port-forward และผ่าน ingress
**คำใบ้:** `kubectl -n todo-app port-forward svc/todo-api 8080:80` / เพิ่ม `$(minikube ip) todo.local` ใน `/etc/hosts`
**ผ่านเมื่อ:** ได้ 200 ทั้งสองทาง และอธิบายได้ว่าสองวิธีนี้ traffic วิ่งต่างกันยังไง

---

### K1.3 อ่าน describe ให้เป็น

**โจทย์:** `describe` pod หนึ่งตัว แล้วหาข้อมูล 4 อย่าง — image ที่ใช้, node ที่อยู่, ค่า probe, ส่วน Events
**ผ่านเมื่อ:** ชี้ได้ว่าส่วน Events อยู่ตรงไหนและมันบอกอะไร (นี่คือส่วนที่ต้องดูเป็นอย่างแรกเสมอเวลามีปัญหา)

---

### K1.4 ดู log รวมถึง initContainer

**โจทย์:** ดู log ของ container หลัก และของ initContainer ที่รัน migration
**คำใบ้:** `kubectl -n todo-app logs <pod>` และ `-c migrate`
**ผ่านเมื่อ:** เห็นข้อความ prisma migrate ใน log ของ initContainer

---

### K1.5 ทำให้ pod ตายแล้วดูว่าเกิดอะไร

**โจทย์:** ลบ pod ทิ้ง 1 ตัว
**คำใบ้:** `kubectl -n todo-app delete pod <pod>`
**ผ่านเมื่อ:** เห็น pod ใหม่ขึ้นมาแทนทันที และอธิบายได้ว่าใครเป็นคนสร้างให้ (ReplicaSet ที่ Deployment สร้างไว้)

---

### K1.6 scale ด้วยมือ

**โจทย์:** เพิ่มเป็น 5 replica แล้วลดกลับเหลือ 2
**คำใบ้:** `kubectl -n todo-app scale deployment/todo-api --replicas=5`
**ผ่านเมื่อ:** เห็นจำนวนเปลี่ยนจริง แล้วตอบได้ว่าถ้ารัน `kubectl apply -k` ซ้ำจะเกิดอะไรกับค่าที่เพิ่ง scale ไป

---

### K1.7 ConfigMap กับ Secret ต่างกันยังไง

**โจทย์:** ดูค่าใน ConfigMap และ Secret แล้วถอด Secret ออกมาอ่าน

```bash
kubectl -n todo-app get configmap todo-config -o yaml
kubectl -n todo-app get secret todo-secret -o jsonpath='{.data.DATABASE_URL}' | base64 -d
```

**ผ่านเมื่อ:** อ่านค่ารหัสผ่านออกมาได้ แล้วอธิบายได้ว่าทำไมนี่คือเหตุผลที่ห้าม commit Secret ลง git

---

### K1.8 หา Service กับ Endpoint

**โจทย์:** ดูว่า Service `todo-api` ชี้ไปที่ pod ตัวไหนบ้าง
**คำใบ้:** `kubectl -n todo-app get endpoints todo-api`
**ผ่านเมื่อ:** จำนวน IP ใน endpoints ตรงกับจำนวน pod ที่ ready และลองทำให้ pod ตัวหนึ่งไม่ ready แล้วเห็นว่ามันหายไปจาก endpoints

---

## ✅ เช็กลิสต์จบระดับ 1

- [ ] deploy และลบทั้งชุดได้ด้วยคำสั่งเดียว
- [ ] ใช้ `get` / `describe` / `logs` / `exec` เป็น
- [ ] เข้าใจความสัมพันธ์ Deployment → ReplicaSet → Pod
- [ ] เข้าใจว่า Service หา pod เจอได้ยังไง

➡️ [เฉลยระดับ 1](../solutions/kubernetes/01-beginner.md) · [ไประดับ 2](02-intermediate.md)
