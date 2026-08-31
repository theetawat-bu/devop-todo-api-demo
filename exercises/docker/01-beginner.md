# 🐳 Docker — ระดับ 1: เริ่มต้น

> **เป้าหมาย:** รัน container เป็น อ่าน Dockerfile ออก และหาสาเหตุเวลามันไม่ขึ้นได้
> **ต้องมีก่อน:** Docker Desktop ติดตั้งแล้ว (`docker version` ใช้ได้)
> **เวลาโดยประมาณ:** 1–2 ชั่วโมง
> **อ่านประกอบ:** [docs/03 — Docker](../../docs/03-docker.md)

---

### D1.1 แยกให้ออกว่า image กับ container ต่างกันยังไง

**โจทย์:** build image จากโปรเจกต์นี้ แล้วรันเป็น container 2 ตัวพร้อมกันจาก image เดียวกัน
**คำใบ้:** `docker build -t devops-todo-api:local .` แล้ว `docker run -d --name a ...` และ `--name b ...`
(ตอนนี้ container จะยังตายเพราะไม่มี DB — ไม่เป็นไร ข้อนี้แค่ให้เห็นว่า image เดียวสร้าง container ได้หลายตัว)
**ผ่านเมื่อ:** `docker images` เห็น image 1 รายการ แต่ `docker ps -a` เห็น container 2 รายการ และอธิบายความต่างได้ด้วยคำพูดตัวเอง

---

### D1.2 รันให้ติดจริง ๆ

**โจทย์:** ทำให้ container ที่ build เอง ต่อกับ Postgres ได้และตอบ `/healthz` ได้ โดย**ยังไม่ใช้ compose**
**คำใบ้:**

```bash
docker compose up -d db          # ยืม postgres จาก compose มาใช้ก่อน
docker network ls                # หาชื่อ network ที่ compose สร้าง
docker run -d --name api-manual --network <ชื่อ network> -p 3000:3000 \
  -e DATABASE_URL="postgresql://app:app_password@db:5432/tododb?sslmode=disable" \
  devops-todo-api:local
```

**ผ่านเมื่อ:** `curl localhost:3000/healthz` ได้ 200 และตอบได้ว่าทำไม host ใน `DATABASE_URL` ต้องเป็น `db` ไม่ใช่ `localhost`

---

### D1.3 อ่าน image ให้เป็น

**โจทย์:** ตอบ 3 คำถาม — image ใหญ่เท่าไร, มีกี่ layer, layer ไหนใหญ่สุดและมาจากคำสั่งไหนใน Dockerfile
**คำใบ้:** `docker images`, `docker history devops-todo-api:local`
**ผ่านเมื่อ:** ชี้บรรทัดใน Dockerfile ที่ทำให้เกิด layer ใหญ่สุดได้

---

### D1.4 สำรวจข้างใน container

**โจทย์:** เข้าไปใน container แล้วตอบ — รันด้วย user อะไร, `/app` มีอะไรบ้าง, มี source code `.go` หรือ Go toolchain ติดไปด้วยไหม
**คำใบ้:** `docker exec -it api-manual sh` แล้วลอง `whoami`, `id`, `ls -la /app`, `which go`
**ผ่านเมื่อ:** ตอบได้ว่าทำไมไม่ใช่ root และทำไม `/app` มีแค่ `api` (binary), `migrate` (binary), กับโฟลเดอร์ `migrations/` — ไม่มีไฟล์ `.go` ไม่มี `go.mod` ไม่มี module cache ทั้งที่โปรเจกต์เขียนด้วย Go

---

### D1.5 อ่าน log และ exit code

**โจทย์:** รัน container โดย**ไม่ใส่** `DATABASE_URL` แล้วหาสาเหตุว่าทำไมมันตาย
**คำใบ้:** `docker logs <container>` และ `docker ps -a` ดูคอลัมน์ STATUS
**ผ่านเมื่อ:** อ้างอิงได้ว่า error มาจากส่วนไหนของ startup (`internal/config` อ่าน env แล้วโปรแกรมออก non-zero exit ทันทีถ้าค่าที่จำเป็นหาย) และอธิบายได้ว่าทำไมการตายทันทีแบบนี้ดีกว่าปล่อยให้รันไปเรื่อย ๆ แล้วค่อยพังตอนมี request

---

### D1.6 หยุด ลบ และทำความสะอาด

**โจทย์:** ลบ container ทั้งหมดที่สร้างมา แล้วดูว่า docker กินพื้นที่เครื่องไปเท่าไร
**คำใบ้:** `docker stop`, `docker rm`, `docker system df`
**ผ่านเมื่อ:** อธิบายความต่างของ 3 อย่างที่ `docker system df` แสดง — Images / Containers / Build Cache

---

### D1.7 อ่าน Dockerfile ให้จบทั้งไฟล์

**โจทย์:** ไล่อ่าน `Dockerfile` แล้วตอบว่าทำไมมี `FROM` แค่ 2 อัน (ไม่ใช่ 3 แบบที่โปรเจกต์ Node ทั่วไปมักมี — build / deps / runner) แล้ว image สุดท้ายมีของจาก stage ไหนบ้าง
**คำใบ้:** สังเกตว่า `builder` stage ติดตั้งทั้ง Go toolchain และ compile ทั้ง `api` และ `migrate` เอง ไม่มี stage แยกสำหรับ "production dependencies" เหมือนฝั่ง Node เพราะ Go binary ที่ compile แล้วไม่ต้องพก dependency ใด ๆ ไปด้วยเลย
**ผ่านเมื่อ:** วาดแผนภาพได้ว่าอะไรถูก copy จาก `builder` ไปที่ `runner` (มีแค่ 2 ไฟล์ + 1 โฟลเดอร์: `api`, `migrate`, `migrations/`) และอธิบายได้ว่าทำไม Go ถึงทำให้ multi-stage build เรียบง่ายกว่าภาษาที่ต้อง ship runtime + module cache ไปด้วย

---

## ✅ เช็กลิสต์จบระดับ 1

- [ ] build image เองได้ โดยไม่ต้องเปิดเอกสาร
- [ ] รัน container พร้อมส่ง env และต่อ network ได้
- [ ] เข้าไปดูข้างใน container ได้
- [ ] อ่าน log แล้วบอกสาเหตุที่ container ตายได้
- [ ] อธิบายได้ว่า image / container / layer ต่างกันยังไง

➡️ [เฉลยระดับ 1](../solutions/docker/01-beginner.md) · [ไประดับ 2](02-intermediate.md)
