# ⚙️ CI/CD — ระดับ 3: ใช้งานจริง

> **เป้าหมาย:** ทำ CD ที่ deploy ได้จริง ตรวจได้ว่าสำเร็จ และย้อนกลับได้
> **ต้องมีก่อน:** ผ่านระดับ 2
> **เวลาโดยประมาณ:** 1–2 วัน
> **อ่านประกอบ:** [docs/07 — CD + GHCR](../../docs/07-cd-ghcr.md) · [docs/10 — deploy ฟรีบน cloud](../../docs/10-deploy-free-cloud.md)

---

### C3.1 push image ขึ้น GHCR ให้สำเร็จ

**โจทย์:** ทำให้ workflow `build-push.yml` push image ขึ้น GHCR ได้
**คำใบ้:** ต้องตั้ง Workflow permissions เป็น read/write และชื่อ image ต้องเป็นตัวพิมพ์เล็กทั้งหมด
**ผ่านเมื่อ:** เห็น package ในแท็บ Packages และ `docker pull` ลงเครื่องได้

---

### C3.2 เข้าใจ tag strategy

**โจทย์:** push tag `v1.0.0` แล้วดูว่า `metadata-action` สร้าง tag อะไรให้บ้าง
**ผ่านเมื่อ:** เห็น `1.0.0`, `1.0`, `latest`, `sha-xxxxx` ครบ และอธิบายได้ว่าควรใช้ tag ไหนตอน deploy จริงและเพราะอะไร

---

### C3.3 deploy ขึ้น cloud ฟรีด้วย branch `dev` ⭐

**โจทย์:** ทำตาม [docs/10](../../docs/10-deploy-free-cloud.md) ให้ครบ — Neon + Render + workflow `deploy-paas.yml` (และลอง `deploy-k8s.yml` ต่อถ้าอยากได้ Kubernetes จริง)
**ผ่านเมื่อ:** `git push origin dev` แล้วเว็บอัปเดตเองภายในไม่กี่นาที และเปิดจากมือถือ (ปิด wifi) ใช้งานได้

---

### C3.4 พิสูจน์ว่า gate ทำงาน

**โจทย์:** ทำให้ job `test` พังโดยตั้งใจ แล้ว push เข้า `dev`
**ผ่านเมื่อ:** **ไม่มีการ deploy เกิดขึ้นเลย** — ยืนยันจากที่เว็บยังเป็นเวอร์ชันเก่า
นี่คือข้อที่พิสูจน์ว่า pipeline ของคุณเป็น CD จริง ไม่ใช่แค่ "สคริปต์ที่บังเอิญทำงาน"

---

### C3.5 พิสูจน์ว่า verify ทำงาน

**โจทย์:** ทำให้แอปพังตอน runtime (เช่น ใส่ `throw` ตอน boot) แล้ว push
**ผ่านเมื่อ:** job `verify` จับได้และ workflow แดง — ถ้าเขียว แปลว่า verify ของคุณยังตรวจไม่จริง ให้กลับไปแก้

---

### C3.6 environment + approval gate

**โจทย์:** ตั้ง environment `production` ที่ต้องมีคนกด approve ก่อน deploy
**คำใบ้:** Settings → Environments → Required reviewers
**ผ่านเมื่อ:** workflow หยุดรอจริง และมีอีเมลแจ้งผู้อนุมัติ

---

### C3.7 rollback อัตโนมัติ

**โจทย์:** ทำให้ deploy ที่ล้มเหลวย้อนกลับเวอร์ชันก่อนหน้าเอง
**คำใบ้:** `if: failure()` + `kubectl rollout undo` (หรือเรียก deploy hook ด้วย image ตัวก่อนหน้า)
**ผ่านเมื่อ:** จงใจ deploy image ที่พัง แล้วระบบกลับมาใช้งานได้เองโดยไม่ต้องแตะ

---

### C3.8 สแกนความปลอดภัยใน pipeline

**โจทย์:** เพิ่ม 3 อย่าง — `govulncheck ./...`, Trivy สแกน image, gitleaks หา secret ที่หลุด
**ผ่านเมื่อ:** ลองใส่ secret ปลอมลงไฟล์แล้ว gitleaks จับได้และ CI แดง

---

### C3.9 แจ้งเตือน

**โจทย์:** ส่งข้อความเข้า Discord/Slack เมื่อ deploy สำเร็จหรือล้มเหลว
**คำใบ้:** `if: success()` / `if: failure()` + webhook URL เก็บใน secret
**ผ่านเมื่อ:** ได้รับข้อความจริงทั้งสองกรณี และข้อความมีลิงก์กลับไปหน้า run

---

## ✅ เช็กลิสต์จบระดับ 3

- [ ] push เข้า `dev` แล้วขึ้นเว็บจริงโดยไม่ต้องทำอะไรเพิ่ม
- [ ] โค้ดที่เทสไม่ผ่าน deploy ไม่ได้ (พิสูจน์แล้ว)
- [ ] deploy ที่พัง ถูกจับได้และย้อนกลับเอง (พิสูจน์แล้ว)
- [ ] มีการสแกนความปลอดภัยใน pipeline

➡️ [เฉลยระดับ 3](../solutions/cicd/03-production.md) · [ไประดับ 4](04-advanced.md)
