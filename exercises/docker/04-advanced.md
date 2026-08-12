# 🐳 Docker — ระดับ 4: ขั้นสูง

> **เป้าหมาย:** ใช้ฟีเจอร์ลึกของ BuildKit ทำ image ที่เล็ก เร็ว และปลอดภัยระดับที่ทีมใหญ่ใช้กัน
> **ต้องมีก่อน:** ผ่านระดับ 3, เปิด BuildKit (ค่าเริ่มต้นของ Docker รุ่นใหม่เปิดอยู่แล้ว)
> **เวลาโดยประมาณ:** 1–2 วัน

---

### D4.1 build หลาย architecture

**โจทย์:** ทำ image ที่รันได้ทั้ง `linux/amd64` และ `linux/arm64`
**คำใบ้:** `docker buildx create --use` แล้ว `docker buildx build --platform linux/amd64,linux/arm64 --push -t ghcr.io/<you>/todo:multi .`
**ผ่านเมื่อ:** `docker manifest inspect` เห็นครบทั้งสอง platform และอธิบายได้ว่าทำไมต้อง `--push` (buildx เก็บ multi-arch ไว้ใน local image store ไม่ได้)

---

### D4.2 cache mount

**โจทย์:** ทำให้ `npm ci` ใช้ cache ข้าม build ได้แม้ `package-lock.json` เปลี่ยน
**คำใบ้:**

```dockerfile
RUN --mount=type=cache,target=/root/.npm npm ci
```

**ผ่านเมื่อ:** แก้ `package.json` (เพิ่ม dependency 1 ตัว) แล้ว build ยังเร็วกว่าเดิมชัดเจน — พร้อมอธิบายความต่างระหว่าง cache mount กับ layer cache

---

### D4.3 secret mount

**โจทย์:** สมมติต้องใช้ token ตอน build (เช่นดึงจาก private registry) ทำยังไงไม่ให้ token ติดใน layer
**คำใบ้:**

```dockerfile
RUN --mount=type=secret,id=npmtoken \
    NPM_TOKEN=$(cat /run/secrets/npmtoken) npm ci
```

```bash
docker build --secret id=npmtoken,src=./token.txt .
```

**ผ่านเมื่อ:** ค้นหา token ใน `docker history --no-trunc` และใน tar ของ `docker save` แล้วไม่เจอ

---

### D4.4 distroless

**โจทย์:** เปลี่ยน runtime stage เป็น `gcr.io/distroless/nodejs22-debian12`
**คำใบ้:** distroless ไม่มี shell → `CMD` ต้องเป็น exec form (`["dist/index.js"]`), HEALTHCHECK ที่ใช้ curl จะใช้ไม่ได้, และ `sh -c "prisma migrate deploy && ..."` ก็รันไม่ได้ → ต้องย้าย migration ไป initContainer
**ผ่านเมื่อ:** image เล็กลงชัดเจน แอปยังทำงาน และเขียนสรุปได้ว่า **แลกอะไรไปบ้าง** (debug ยากขึ้นเพราะ exec เข้าไปไม่ได้)

---

### D4.5 แยก base image ของทีมเอง

**โจทย์:** แยกส่วนที่เปลี่ยนน้อย (base + system deps) ออกเป็น image ของตัวเอง แล้วให้ Dockerfile หลัก `FROM` ตัวนั้น
**ผ่านเมื่อ:** วัดได้ว่า build ของโปรเจกต์เร็วขึ้น และอธิบายข้อเสียได้ (ต้องดูแล base image เอง + ถ้าไม่อัปเดตจะกลายเป็นแหล่งสะสม CVE)

---

### D4.6 ทำ image ให้ reproducible

**โจทย์:** build สองครั้งแล้วได้ digest เดียวกัน
**คำใบ้:** pin ทุก `FROM` ด้วย digest, ตั้ง `SOURCE_DATE_EPOCH`, ระวัง timestamp ของไฟล์ที่ copy เข้าไป
**ผ่านเมื่อ:** digest ตรงกัน หรือถ้าไม่ตรง ระบุได้ว่าอะไรคือตัวการ

---

### D4.7 หาว่าอะไรทำให้ image ใหญ่

**โจทย์:** ใช้ `dive` วิเคราะห์ image แล้วหา "wasted space"
**คำใบ้:** `docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock wagoodman/dive devops-todo-api:local`
**ผ่านเมื่อ:** ระบุไฟล์ที่ถูกเพิ่มแล้วลบใน layer หลัง (ซึ่งยังกินที่อยู่) ได้อย่างน้อย 1 รายการ

---

### D4.8 ทดสอบ image อัตโนมัติ

**โจทย์:** เขียนสคริปต์ที่ตรวจ image ก่อนอนุญาตให้ push — ต้องผ่านทุกข้อ: non-root, ไม่มี tag `latest` ใน FROM, ขนาดไม่เกิน X MB, ไม่มี HIGH CVE
**ผ่านเมื่อ:** สคริปต์คืน exit code ไม่เป็น 0 เมื่อผิดกฎ และเสียบเข้า CI ได้

---

## ✅ เช็กลิสต์จบระดับ 4

- [ ] build multi-arch เป็น
- [ ] ใช้ cache mount และ secret mount ได้ถูกที่
- [ ] เข้าใจ trade-off ของ distroless แล้วตัดสินใจเองได้ว่าคุ้มไหม
- [ ] วิเคราะห์ได้ว่าขนาด image หายไปกับอะไร
- [ ] เขียน gate อัตโนมัติสำหรับ image ได้

➡️ [เฉลยระดับ 4](../solutions/docker/04-advanced.md) · [ไประดับ 5](05-expert.md)
