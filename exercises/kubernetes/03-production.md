# ☸️ Kubernetes — ระดับ 3: ใช้งานจริง

> **เป้าหมาย:** ตั้งค่าให้ปลอดภัย ทนต่อความล้มเหลว และ scale ได้จริง
> **ต้องมีก่อน:** ผ่านระดับ 2
> **เวลาโดยประมาณ:** 1–2 วัน

---

### K3.1 ทดสอบ HPA จริง

**โจทย์:** ยิงโหลดจนเห็น pod เพิ่ม แล้วหยุดโหลดแล้วจับเวลาว่ากี่นาที pod ถึงลด

```bash
kubectl -n todo-app get hpa -w
kubectl -n todo-app run load --rm -it --image=busybox --restart=Never -- \
  sh -c "while true; do wget -q -O- http://todo-api/api/todos; done"
```

**ผ่านเมื่อ:** เห็น replica เพิ่มจริง และเวลาที่ลดตรงกับ `stabilizationWindowSeconds` ที่ตั้งไว้ (~2 นาที)

---

### K3.2 ทดสอบ resource limit

**โจทย์:** ลด memory limit เหลือ `64Mi` แล้วยิงโหลด
**ผ่านเมื่อ:** เห็น `OOMKilled` ใน `describe pod` ส่วน Last State และอธิบายได้ว่าต่างจากการเกิน CPU limit ยังไง

---

### K3.3 PodDisruptionBudget

**โจทย์:** ทดสอบว่า PDB กัน pod ถูกไล่ออกพร้อมกันได้จริง
**คำใบ้:** `kubectl drain <node> --ignore-daemonsets` (ถ้ามี node เดียวให้ลด replica แล้วดู `kubectl -n todo-app get pdb`)
**ผ่านเมื่อ:** อธิบายได้ว่า `minAvailable: 1` มีผลตอนไหนบ้าง และตอนไหนที่ PDB **ไม่ช่วย** (เช่น node ดับกะทันหัน — PDB คุมได้แค่การไล่ออกโดยสมัครใจ)

---

### K3.4 NetworkPolicy

**โจทย์:** ทำให้เฉพาะ pod ที่มี label `app: todo-api` เท่านั้นที่ต่อ postgres ได้
**คำใบ้:** ต้องมี CNI ที่รองรับ (`minikube start --cni=calico`)
**ผ่านเมื่อ:** สร้าง pod ทดสอบที่ไม่มี label นั้นแล้วต่อ postgres ไม่ได้ ขณะที่ api ยังใช้งานได้ปกติ

---

### K3.5 ย้าย migration ออกจาก initContainer ไปเป็น Job

**โจทย์:** เปลี่ยนให้ migration รันเป็น `Job` แยก แทนที่จะอยู่ใน initContainer ของทุก pod
**ผ่านเมื่อ:** deploy ด้วย 3 replica แล้ว migration รันครั้งเดียว (ตรวจจาก log) และอธิบายได้ว่าทำไมวิธีนี้ดีกว่าเมื่อมีหลาย replica

---

### K3.6 จัดการ Secret ให้ถูกวิธี

**โจทย์:** เอา `secret.yaml` ออกจาก kustomize แล้วเปลี่ยนไปใช้ Sealed Secrets
**คำใบ้:** ติดตั้ง sealed-secrets controller แล้วใช้ `kubeseal` แปลง Secret ธรรมดาเป็น SealedSecret
**ผ่านเมื่อ:** commit ไฟล์ SealedSecret ลง git ได้อย่างปลอดภัย และแอปยังอ่านค่าได้ — พร้อมพิสูจน์ว่าคนที่ได้ไฟล์นั้นไปถอดรหัสเองไม่ได้

---

### K3.7 ตรวจ security context

**โจทย์:** ยืนยันว่า pod รัน non-root จริง แล้วเปิด `readOnlyRootFilesystem: true`
**คำใบ้:** `kubectl -n todo-app exec <pod> -- id` และ `-- touch /test`
**ผ่านเมื่อ:** uid ไม่ใช่ 0, เขียน root fs ไม่ได้ และแอปยังทำงานปกติ

---

### K3.8 กระจาย pod ไม่ให้กระจุก

**โจทย์:** ตั้ง `topologySpreadConstraints` หรือ pod anti-affinity ให้ pod ไม่ไปอยู่ node เดียวกันหมด
**ผ่านเมื่อ:** (ถ้ามีหลาย node) เห็น pod กระจายจริง / (ถ้ามี node เดียว) อธิบายได้ว่าทำไม pod ถึงค้างที่ Pending เมื่อตั้ง `whenUnsatisfiable: DoNotSchedule`

---

### K3.9 ทดสอบ graceful shutdown บนคลัสเตอร์

**โจทย์:** ยิง request ที่ใช้เวลา 5 วิ แล้วลบ pod นั้นระหว่างที่ request ยังไม่เสร็จ
**ผ่านเมื่อ:** request เดิมได้คำตอบครบ ไม่ถูกตัดกลางคัน แล้วอธิบายลำดับเหตุการณ์ได้ — k8s ถอดออกจาก endpoints → ส่ง SIGTERM → แอปปิด server → หมด grace period ค่อย SIGKILL

---

## ✅ เช็กลิสต์จบระดับ 3

- [ ] HPA ทำงานจริงและรู้ว่ามันคำนวณจากอะไร
- [ ] รู้จักอาการของ OOMKilled และ CPU throttling
- [ ] Secret ไม่อยู่ใน git แบบดิบ ๆ อีกต่อไป
- [ ] migration รันครั้งเดียวแม้มีหลาย replica
- [ ] deploy แล้วไม่มี request ไหนถูกตัดกลางคัน

➡️ [เฉลยระดับ 3](../solutions/kubernetes/03-production.md) · [ไประดับ 4](04-advanced.md)
