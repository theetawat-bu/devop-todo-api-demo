# ☸️ Kubernetes — ระดับ 2: พื้นฐานแน่น

> **เป้าหมาย:** เข้าใจ probe, rolling update และ kustomize จากการทดลองจริง
> **ต้องมีก่อน:** ผ่านระดับ 1
> **เวลาโดยประมาณ:** 4–6 ชั่วโมง

---

### K2.1 ทำให้ readiness ไม่ผ่าน แล้วดูผล

**โจทย์:** ทำให้ `/readyz` คืน 503 (เช่นดับ postgres) แล้วสังเกต 3 อย่าง — สถานะ READY ของ pod, endpoints ของ Service, และจำนวน RESTARTS
**ผ่านเมื่อ:** ยืนยันได้ว่า pod **ไม่ถูก restart** แต่หายไปจาก endpoints — แล้วอธิบายว่าทำไมพฤติกรรมนี้ถึงถูกต้อง

---

### K2.2 ทำให้ liveness ไม่ผ่าน แล้วดูผล

**โจทย์:** แก้ `livenessProbe` ให้ชี้ไป path ที่ไม่มีจริง แล้ว apply
**ผ่านเมื่อ:** เห็น RESTARTS เพิ่มขึ้นเรื่อย ๆ จนเข้า `CrashLoopBackOff` และเข้าใจความต่างจาก K2.1 อย่างชัดเจน — จากนั้นแก้กลับ

---

### K2.3 ทดลอง anti-pattern ของ probe

**โจทย์:** เปลี่ยน `livenessProbe` ให้ชี้ไป `/readyz` (ซึ่งเช็ค DB) แล้วดับ postgres
**ผ่านเมื่อ:** เห็น pod ทั้งหมดเข้า CrashLoopBackOff พร้อมกัน — นี่คือ **restart storm** ที่ [docs/08](../../docs/08-kubernetes.md) เตือนไว้ แล้วแก้กลับ
**คำถามที่ต้องตอบ:** ตอน DB กลับมา ระบบจะฟื้นเร็วขึ้นหรือช้าลงจากการที่ pod restart วนอยู่ เพราะอะไร

---

### K2.4 rolling update แบบไม่มี downtime

**โจทย์:** ยิง request ต่อเนื่องขณะเปลี่ยน image แล้วนับว่าพลาดกี่ตัว

```bash
# terminal 1
while true; do curl -s -o /dev/null -w "%{http_code}\n" http://todo.local/healthz; sleep 0.2; done
# terminal 2
kubectl -n todo-app set image deployment/todo-api api=<image ใหม่>
```

**ผ่านเมื่อ:** ไม่เห็น 502/503 เลย — ถ้าเห็น ให้หาว่าเป็นเพราะ probe หรือเพราะ graceful shutdown แล้วแก้

---

### K2.5 พิสูจน์ maxUnavailable

**โจทย์:** เปลี่ยนเป็น `maxUnavailable: 1, maxSurge: 0` แล้วทำ K2.4 ซ้ำ
**ผ่านเมื่อ:** เทียบจำนวน request ที่พลาดกับค่าเดิมได้ และอธิบายว่าสองค่านี้แลกอะไรกับอะไร (ความเร็วในการ deploy vs ความจุที่เหลือระหว่าง deploy)

---

### K2.6 rollback

**โจทย์:** deploy image ที่ไม่มีจริง แล้ว rollback กลับ
**คำใบ้:** `kubectl -n todo-app rollout history/undo deployment/todo-api`
**ผ่านเมื่อ:** กลับมาใช้งานได้ และอธิบายได้ว่า `revisionHistoryLimit` มีผลยังไงต่อจำนวนเวอร์ชันที่ย้อนได้

---

### K2.7 kustomize overlay

**โจทย์:** สร้าง overlay ใหม่ชื่อ `staging` — 2 replica, host `todo-staging.local`, namespace `todo-staging`
**คำใบ้:** ก็อปโครงจาก `overlays/dev` แล้วแก้เฉพาะส่วนต่าง
**ผ่านเมื่อ:** `kubectl kustomize k8s/overlays/staging` ออกมาถูกต้อง และ apply แล้วทั้งสอง environment อยู่ร่วมกันได้โดยไม่ชนกัน

---

### K2.8 ดู diff ก่อน apply

**โจทย์:** แก้ replicas ในไฟล์แล้วดูว่าจะเปลี่ยนอะไรบ้าง ก่อนกด apply จริง
**คำใบ้:** `kubectl diff -k k8s/overlays/dev`
**ผ่านเมื่อ:** อ่าน diff ออกและอธิบายได้ว่าทำไมนิสัยนี้สำคัญมากใน production

---

### K2.9 ข้อมูลไม่หายจริงไหม

**โจทย์:** สร้าง todo หลายรายการ แล้วลบ pod ของ postgres ทิ้ง
**ผ่านเมื่อ:** pod ใหม่ขึ้นมาแล้วข้อมูลยังอยู่ครบ แล้วอธิบายได้ว่า PVC/volumeClaimTemplates ทำงานยังไง
**คำถามต่อ:** ถ้าลบ StatefulSet ทิ้งเลย PVC จะหายไปด้วยไหม (ลองดูจริง)

---

## ✅ เช็กลิสต์จบระดับ 2

- [ ] อธิบายความต่างของ liveness/readiness จากผลทดลองของตัวเอง
- [ ] ทำ rolling update โดยไม่มี request พลาดสักตัว
- [ ] rollback ได้ภายในไม่กี่วินาที
- [ ] สร้าง overlay ใหม่เองได้โดยไม่ก็อปทั้งชุด

➡️ [เฉลยระดับ 2](../solutions/kubernetes/02-intermediate.md) · [ไประดับ 3](03-production.md)
