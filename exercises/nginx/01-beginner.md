# 🌐 Nginx — ระดับ 1: เริ่มต้น

> **เป้าหมาย:** อ่าน config ออก รู้ว่า request วิ่งผ่านอะไรบ้าง และแก้ config เล็ก ๆ ได้
> **ต้องมีก่อน:** `docker compose up -d` ขึ้นครบทุก service
> **เวลาโดยประมาณ:** 1–2 ชั่วโมง
> **อ่านประกอบ:** [docs/05 — Nginx](../../docs/05-nginx.md)

**นิสัยที่ต้องติดตัวตั้งแต่วันแรก:**

```bash
docker compose exec nginx nginx -t          # ตรวจ syntax ก่อนเสมอ
docker compose exec nginx nginx -s reload   # reload โดยไม่ตัด connection
```

---

### N1.1 ไล่เส้นทาง request

**โจทย์:** ยิง `curl -v http://localhost:8080/api/todos` แล้วเขียนเส้นทางที่ request วิ่งผ่านทั้งหมด
**ผ่านเมื่อ:** วาดได้ครบ: client → nginx:80 → บล็อก `location` ไหน → `upstream` ชื่ออะไร → container ไหน port อะไร

---

### N1.2 หา config ให้เจอ

**โจทย์:** ตอบว่า `limit_req_zone` ประกาศไว้ไฟล์ไหน ถูกใช้งานที่ไฟล์ไหน และทำไมต้องแยกกัน
**ผ่านเมื่อ:** อธิบายได้ว่า zone ต้องอยู่ในบล็อก `http` ส่วนการบังคับใช้อยู่ในบล็อก `location`

---

### N1.3 เพิ่ม endpoint ของ nginx เอง

**โจทย์:** เพิ่ม `location = /ping` ที่ตอบ `pong` โดยไม่ส่งต่อไป backend
**คำใบ้:** `return 200 "pong\n";`
**ผ่านเมื่อ:** `curl localhost:8080/ping` ได้ `pong` และ `nginx -t` ผ่าน

---

### N1.4 อ่าน access log ให้ออก

**โจทย์:** ยิง 5 request แล้วอธิบายค่า `upstream=`, `rt=`, `urt=` ใน log
**ผ่านเมื่อ:** บอกได้ว่า `rt - urt` คือเวลาที่หายไปกับอะไร

---

### N1.5 ทำให้เกิด 502 แล้วแก้

**โจทย์:** `docker compose stop api` แล้วยิง request
**ผ่านเมื่อ:** เห็น 502, อ่าน error log ของ nginx เจอสาเหตุ แล้วทำให้กลับมาปกติได้

---

### N1.6 `=` vs prefix vs regex

**โจทย์:** ในไฟล์ config มี `location = /nginx-health`, `location /api/`, และ `location ~ ^/(healthz|readyz)$` — เรียงลำดับความสำคัญของทั้งสามแบบให้ถูก
**ผ่านเมื่อ:** ตอบได้ว่า nginx เลือก location ยังไงเมื่อ path ตรงกับหลายบล็อก และทดลองพิสูจน์ด้วยการเพิ่ม location ทับซ้อนกัน

---

### N1.7 หา config ที่ nginx ใช้จริง

**โจทย์:** ดู config ทั้งหมดที่ nginx โหลดอยู่จริงในตอนนี้ (รวมไฟล์ที่ include เข้ามาแล้ว)
**คำใบ้:** `docker compose exec nginx nginx -T`
**ผ่านเมื่อ:** เห็น `nginx.conf` และ `default.conf` ต่อกันเป็นไฟล์เดียว และหาบรรทัด `limit_req_zone` เจอในผลลัพธ์

---

## ✅ เช็กลิสต์จบระดับ 1

- [ ] รู้ว่าไฟล์ config อยู่ที่ไหนและอันไหนทำอะไร
- [ ] แก้ config แล้ว test + reload เป็นนิสัย
- [ ] อ่าน access log และ error log แล้วบอกสาเหตุปัญหาได้
- [ ] เข้าใจว่า `location` ถูกเลือกยังไง

➡️ [เฉลยระดับ 1](../solutions/nginx/01-beginner.md) · [ไประดับ 2](02-intermediate.md)
