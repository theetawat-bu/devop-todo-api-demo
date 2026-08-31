# 🌐 Nginx — ระดับ 3: ใช้งานจริง

> **เป้าหมาย:** ตั้ง proxy ที่ปลอดภัย ทน backend ล่ม และเปิด HTTPS ได้
> **ต้องมีก่อน:** ผ่านระดับ 2
> **เวลาโดยประมาณ:** 4–8 ชั่วโมง

---

### N3.1 เปิด HTTPS

**โจทย์:** ทำ self-signed cert แล้วเปิด 443 พร้อม redirect 80 → 443

```bash
mkdir -p nginx/certs
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout nginx/certs/key.pem -out nginx/certs/cert.pem -subj "/CN=localhost"
```

**ผ่านเมื่อ:** `curl -k https://localhost/api/todos` ใช้ได้ และ `curl -I http://localhost` ได้ 301
**อย่าลืม:** ต้อง mount โฟลเดอร์ certs เข้า container และเปิด port 443 ใน compose

---

### N3.2 จูน TLS ให้ได้เกรดดี

**โจทย์:** ตั้ง `ssl_protocols TLSv1.2 TLSv1.3;`, ปิด cipher เก่า, เปิด `ssl_session_cache`
**ผ่านเมื่อ:** `docker run --rm -it drwetter/testssl.sh https://host.docker.internal` ไม่มีคำเตือนระดับ HIGH และอธิบายได้ว่าทำไมไม่ควรเปิด TLS 1.0/1.1

---

### N3.3 passive health check

**โจทย์:** ตั้ง `max_fails=2 fail_timeout=10s` แล้ว scale เป็น 3 จากนั้น kill ไป 1 ตัวขณะยิงต่อเนื่อง
**ผ่านเมื่อ:** ยิง 200 request แล้ว**ไม่เห็น 502 เลย** และอธิบายได้ว่าทำไมยังมี request ช่วงแรกที่ช้ากว่าปกติ

---

### N3.4 security headers

**โจทย์:** เพิ่ม `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Strict-Transport-Security`, `Content-Security-Policy`
**ผ่านเมื่อ:** `curl -I` เห็นครบ และอธิบายได้ว่าแต่ละตัวกันการโจมตีแบบไหน — พร้อมบอกได้ว่าทำไม HSTS ถึงอันตรายถ้าตั้งผิด (เบราว์เซอร์จำ ยกเลิกยาก)

---

### N3.5 gzip ที่วัดผลได้

**โจทย์:** สร้าง todo 500 รายการ แล้ววัดขนาด response ก่อน-หลังเปิด gzip

```bash
for i in $(seq 1 500); do curl -s -X POST localhost:8080/api/todos \
  -H 'Content-Type: application/json' -d "{\"title\":\"task $i\"}" > /dev/null; done

curl -s -o /dev/null -w 'ไม่บีบ: %{size_download}\n' localhost:8080/api/todos
curl -s -H 'Accept-Encoding: gzip' -o /dev/null -w 'บีบแล้ว: %{size_download}\n' localhost:8080/api/todos
```

**ผ่านเมื่อ:** มีตัวเลขเทียบ และอธิบายได้ว่าทำไมต้องมี `gzip_min_length`

---

### N3.6 routing หลาย service

**โจทย์:** เพิ่ม adminer ที่ `/admin` โดย `/api` ยังทำงานเหมือนเดิม
**คำใบ้:** ระวังเรื่อง trailing slash ของ `proxy_pass` — `proxy_pass http://adminer:8080;` กับ `proxy_pass http://adminer:8080/;` ให้ผลต่างกันคนละเรื่อง
**ผ่านเมื่อ:** เปิด `/admin` แล้ว CSS/JS โหลดครบ ไม่ 404

---

### N3.7 ปิดข้อมูลที่ไม่ควรเปิดเผย

**โจทย์:** ตรวจว่า response บอกเวอร์ชัน nginx หรือเปล่า แล้วปิด
**คำใบ้:** `server_tokens off;` (ในโปรเจกต์นี้ตั้งไว้แล้ว — ให้ลองเปิดดูว่าต่างกันยังไง)
**ผ่านเมื่อ:** `curl -I` เห็นแค่ `Server: nginx` ไม่มีเลขเวอร์ชัน และอธิบายได้ว่าทำไมเรื่องนี้ถือเป็น defense in depth ไม่ใช่การป้องกันจริง

---

### N3.8 log ให้พร้อมใช้สอบสวน

**โจทย์:** เพิ่ม `$request_id` เข้า log format และส่งต่อเป็น header ไปให้แอป
**คำใบ้:** `proxy_set_header X-Request-Id $request_id;` แล้วให้ middleware log ของ Gin (`requestLogger()` ใน `internal/app/app.go`) log ค่านี้ด้วย
**ผ่านเมื่อ:** หา request หนึ่งใน log ของ nginx แล้วตามไปเจอ log เดียวกันฝั่งแอปได้ (นี่คือพื้นฐานของ distributed tracing)

---

## ✅ เช็กลิสต์จบระดับ 3

- [ ] HTTPS ใช้งานได้ พร้อม redirect และ TLS config ที่ไม่มีคำเตือน
- [ ] backend ล่มแล้วผู้ใช้ไม่เห็น 502
- [ ] security header ครบและอธิบายได้ทุกตัว
- [ ] ตามรอย request เดียวข้ามชั้นได้ด้วย request id

➡️ [เฉลยระดับ 3](../solutions/nginx/03-production.md) · [ไประดับ 4](04-advanced.md)
