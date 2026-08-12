# ⚙️ CI/CD — ระดับ 4: ขั้นสูง

> **เป้าหมาย:** ทำ pipeline แบบ GitOps และ supply chain security ที่ตรวจสอบย้อนกลับได้
> **ต้องมีก่อน:** ผ่านระดับ 3
> **เวลาโดยประมาณ:** 3–5 วัน
> **อ่านประกอบ:** [docs/11 — Enterprise GitOps](../../docs/11-enterprise-gitops.md)

---

### C4.1 แยก config repo

**โจทย์:** ย้าย `k8s/` ออกไปเป็นอีก repo แล้วให้ CI commit image digest กลับเข้าไป
**ผ่านเมื่อ:** history ของ config repo อ่านแล้วเห็นชัดว่าแต่ละ environment เปลี่ยนเวอร์ชันเมื่อไร — และยืนยันได้ว่าไม่เกิด CI วนไม่จบ

---

### C4.2 GitOps ด้วย Argo CD

**โจทย์:** ให้ Argo CD sync จาก config repo แทนที่ CI จะ `kubectl apply`
**ผ่านเมื่อ:** ไม่มีคำสั่ง `kubectl` เหลืออยู่ใน workflow เลย และ deploy ยังทำงานได้ครบ

---

### C4.3 deploy ด้วย digest ไม่ใช่ tag

**โจทย์:** เปลี่ยนทุกจุดให้ใช้ `image@sha256:...`
**ผ่านเมื่อ:** อธิบายได้ว่าทำไม tag ลอยถึงทำให้ "rollback แล้วยังพังเหมือนเดิม" ได้ และพิสูจน์ด้วยการ deploy digest เดิมซ้ำแล้วได้ผลเหมือนเดิมเป๊ะ

---

### C4.4 SBOM + provenance + ลายเซ็น

**โจทย์:** ทำ 3 อย่างใน pipeline — สร้าง SBOM, แนบ provenance, เซ็นด้วย cosign
**คำใบ้:** `--sbom=true --provenance=true` ของ buildx และ keyless signing ผ่าน OIDC ของ GitHub Actions
**ผ่านเมื่อ:** `cosign verify` ผ่าน และดึง SBOM กลับมาอ่านได้

---

### C4.5 reusable workflow

**โจทย์:** แยกส่วนที่ซ้ำ (build+push) ออกเป็น workflow ที่เรียกใช้ซ้ำได้
**คำใบ้:** `on: workflow_call:` แล้วเรียกด้วย `uses: ./.github/workflows/build.yml`
**ผ่านเมื่อ:** ทั้ง `deploy-paas.yml` และ `deploy-k8s.yml` เรียก `build-push.yml` ตัวเดียวกัน และแก้ที่เดียวมีผลทุกที่

---

### C4.6 self-hosted runner

**โจทย์:** ตั้ง self-hosted runner แล้วให้ workflow บาง job รันบนนั้น
**ผ่านเมื่อ:** job รันบน runner ของตัวเองสำเร็จ และตอบได้ว่าทำไม**ห้าม**ใช้ self-hosted runner กับ public repo

---

### C4.7 preview environment ต่อ PR

**โจทย์:** เปิด PR แล้วให้ระบบสร้าง environment ชั่วคราวให้ พร้อมคอมเมนต์ URL กลับใน PR — ปิด PR แล้วลบทิ้ง
**ผ่านเมื่อ:** ทำงานครบวงจร รวมถึง **การเก็บกวาด** (ข้อที่คนลืมบ่อยที่สุดจนทรัพยากรรั่วไปเรื่อย ๆ)

---

### C4.8 progressive delivery

**โจทย์:** ทำ canary ที่ปล่อย traffic 10% → 50% → 100% โดยดู metric ประกอบ และย้อนกลับอัตโนมัติถ้า error rate สูง
**คำใบ้:** Argo Rollouts + analysis template ที่ query Prometheus
**ผ่านเมื่อ:** จงใจ deploy เวอร์ชันที่มี error สูง แล้วระบบหยุดและย้อนกลับเองโดยไม่มีคนสั่ง

---

## ✅ เช็กลิสต์จบระดับ 4

- [ ] git คือแหล่งความจริงของสิ่งที่รันอยู่จริง
- [ ] ตรวจสอบย้อนกลับได้ว่า image ที่รันอยู่มาจาก commit ไหนและใครเซ็น
- [ ] workflow ไม่มีโค้ดซ้ำ
- [ ] ปล่อยเวอร์ชันใหม่แบบค่อยเป็นค่อยไปพร้อมย้อนกลับอัตโนมัติได้

➡️ [เฉลยระดับ 4](../solutions/cicd/04-advanced.md) · [ไประดับ 5](05-expert.md)
