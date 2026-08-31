# เฉลย — 🌐 Nginx ระดับ 3

⬅️ [กลับไปที่โจทย์](../../nginx/03-production.md)

---

## N3.1 HTTPS

```bash
mkdir -p nginx/certs
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout nginx/certs/key.pem -out nginx/certs/cert.pem -subj "/CN=localhost"
```

`docker-compose.yml`:

```yaml
nginx:
  ports: ["8080:80", "8443:443"]
  volumes:
    - ./nginx/certs:/etc/nginx/certs:ro
```

`conf.d/default.conf`:

```nginx
server {
    listen 80;
    server_name _;
    location = /nginx-health { return 200 "ok\n"; }   # ให้ health check ไม่ต้องผ่าน redirect
    location / { return 301 https://$host$request_uri; }
}

server {
    listen 443 ssl;
    http2 on;
    server_name _;

    ssl_certificate     /etc/nginx/certs/cert.pem;
    ssl_certificate_key /etc/nginx/certs/key.pem;

    # … location เดิมทั้งหมดย้ายมาไว้ที่นี่
}
```

```bash
curl -k https://localhost:8443/api/todos
curl -I http://localhost:8080          # 301
```

**ทำไม health check ไม่ควรโดน redirect:** monitoring หลายตัวไม่ตาม redirect และจะรายงานว่า service ล่มทั้งที่ปกติ

---

## N3.2 จูน TLS

```nginx
ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers off;              # TLS 1.3 ให้ client เลือกดีกว่า
ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384;
ssl_session_cache shared:SSL:10m;
ssl_session_timeout 1d;
ssl_session_tickets off;
```

**ทำไมไม่ควรเปิด TLS 1.0/1.1:**

- มีช่องโหว่ที่รู้กันแล้ว (BEAST, POODLE) และ cipher เก่าที่แตกได้
- PCI DSS ห้ามใช้ตั้งแต่ปี 2018
- เบราว์เซอร์หลักเลิกรองรับหมดแล้วตั้งแต่ปี 2020 — เปิดไว้ก็ไม่มีใครใช้ นอกจากผู้โจมตีที่พยายาม downgrade

**`ssl_session_cache` ช่วยอะไร:** TLS handshake แพงมาก (2 round-trip + คำนวณ crypto)
session cache ทำให้ client ที่เคยต่อแล้วกลับมาใหม่ทำ handshake แบบย่อได้ → ลด latency ได้หลายสิบ ms

**`ssl_session_tickets off`:** ticket key ที่ไม่หมุนเวียนทำให้เสีย forward secrecy ถ้าไม่มีระบบหมุน key ให้ปิดไว้

---

## N3.3 passive health check

```nginx
upstream api_backend {
    server api:3000 max_fails=2 fail_timeout=10s;
    keepalive 32;
}
```

```bash
docker compose up -d --scale api=3
while true; do curl -s -o /dev/null -w "%{http_code} " localhost:8080/api/todos; sleep 0.05; done &
docker kill $(docker ps -q -f name=api | head -1)
```

ผล: ไม่เห็น 502 เลย (`proxy_next_upstream` ทำงาน)

**ทำไมยังมี request ที่ช้ากว่าปกติในช่วงแรก:**

```
request #1 → เจอตัวที่ตาย → รอ connect timeout → ลองตัวถัดไป → สำเร็จ
             └── เวลาที่เสียไปตรงนี้คือ proxy_connect_timeout (5s ในค่าเริ่มต้นของเรา)
```

ผู้ใช้ได้ 200 แต่ช้ากว่าปกติมาก จนกว่า nginx จะนับครบ `max_fails=2` แล้วเลิกส่งไปหาตัวนั้น

**วิธีลดผลกระทบ:** ลด `proxy_connect_timeout` เหลือ 1-2 วินาที (การเชื่อมต่อภายใน network เดียวกันควรใช้เวลาไม่ถึง 100ms อยู่แล้ว)
ตั้งไว้ 5 วินาทีสำหรับ connection ภายในคือการรอนานเกินความจำเป็น

---

## N3.4 security headers

```nginx
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Content-Security-Policy "default-src 'self'" always;
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

| header | กันอะไร |
| --- | --- |
| `X-Frame-Options` | clickjacking — เว็บอื่นเอาเราไปใส่ iframe แล้วหลอกให้คลิก |
| `X-Content-Type-Options: nosniff` | เบราว์เซอร์เดา MIME type เอง แล้วรันไฟล์ที่ควรเป็นข้อความเป็น JavaScript |
| `Referrer-Policy` | ข้อมูลใน URL รั่วไปเว็บอื่นผ่าน Referer header |
| `Content-Security-Policy` | XSS — จำกัดว่าโหลด script จากไหนได้บ้าง (**ตัวที่ได้ผลที่สุด**) |
| `Strict-Transport-Security` | SSL stripping — บังคับให้เบราว์เซอร์ใช้ HTTPS เท่านั้น |

**`always` สำคัญ:** ถ้าไม่ใส่ header จะไม่ถูกส่งใน response ที่เป็น error (4xx/5xx) ซึ่งเป็นช่องที่ถูกใช้โจมตีได้

**ทำไม HSTS อันตรายถ้าตั้งผิด:**
เบราว์เซอร์จะ**จำไว้ตาม `max-age`** (1 ปี) ว่าโดเมนนี้ต้องใช้ HTTPS เท่านั้น
ถ้าวันหนึ่ง cert หมดอายุหรือต้องกลับไป HTTP ชั่วคราว → **ผู้ใช้เข้าเว็บไม่ได้เลยและข้ามคำเตือนไม่ได้ด้วย**

วิธีที่ปลอดภัย: เริ่มที่ `max-age=300` (5 นาที) → ทดสอบสักสัปดาห์ → ค่อยเพิ่มเป็น 1 ปี
และอย่าใส่ `preload` จนกว่าจะมั่นใจ 100% (ถอนออกจาก preload list ใช้เวลาหลายเดือน)

---

## N3.5 gzip

```bash
for i in $(seq 1 500); do curl -s -X POST localhost:8080/api/todos \
  -H 'Content-Type: application/json' -d "{\"title\":\"งานที่ $i\"}" > /dev/null; done

curl -s -o /dev/null -w 'ไม่บีบ: %{size_download}\n' localhost:8080/api/todos
curl -s -H 'Accept-Encoding: gzip' -o /dev/null -w 'บีบแล้ว: %{size_download}\n' localhost:8080/api/todos
```

ผลที่มักได้: ~60KB → ~4KB (**ลดลง ~93%** เพราะ JSON มีโครงสร้างซ้ำ ๆ ที่บีบได้ดีมาก)

```nginx
gzip on;
gzip_types application/json text/plain application/javascript text/css;
gzip_min_length 1024;
gzip_comp_level 5;
```

**ทำไมต้องมี `gzip_min_length`:**

- gzip มี header ประมาณ 20 bytes → response ขนาด 50 bytes บีบแล้วอาจ**ใหญ่ขึ้น**
- การบีบกิน CPU — ไม่คุ้มกับไฟล์เล็ก
- ค่า 1024 bytes เป็นจุดที่ยอมรับกันทั่วไป

**`gzip_comp_level`:** ระดับ 1-9 แต่ระดับ 9 กิน CPU มากกว่าระดับ 5 เกือบเท่าตัวโดยได้ขนาดเล็กลงแค่ ~2%
**ระดับ 4-6 คือจุดคุ้มค่าที่สุด**

---

## N3.6 routing หลาย service

```yaml
# docker-compose.yml
adminer:
  image: adminer
  networks: [appnet]
```

```nginx
upstream adminer_backend { server adminer:8080; }

location /admin/ {
    proxy_pass http://adminer_backend/;      # ← สังเกต / ท้ายสุด
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Prefix /admin;
}
```

**เรื่อง trailing slash ที่ทำให้คนสับสนที่สุดใน nginx:**

| config | ยิง `/admin/foo` | backend ได้รับ |
| --- | --- | --- |
| `proxy_pass http://adminer_backend;` | | `/admin/foo` (ส่งทั้ง path) |
| `proxy_pass http://adminer_backend/;` | | `/foo` (ตัด `/admin` ออก) |

จำง่าย ๆ: **มี `/` ท้าย = ตัด prefix ออก, ไม่มี `/` = ส่งทั้ง path**

**ทำไม CSS/JS มักจะ 404:** แอปหลายตัวสร้าง URL ของ asset เป็น absolute path (`/style.css`) ซึ่งไม่ผ่าน `/admin/` ของเรา
ทางแก้: `sub_filter` เขียน HTML ใหม่ หรือให้แอปรองรับ base path (adminer ทำได้ผ่าน `X-Forwarded-Prefix`)

**บทเรียนจริง:** การ proxy แอปที่ไม่ได้ออกแบบมาให้อยู่ใต้ subpath เป็นงานที่ยากกว่าที่คิดเสมอ — ถ้าเลือกได้ให้ใช้ subdomain แทน (`admin.example.com`)

---

## N3.7 server_tokens

```bash
# server_tokens on;
curl -I localhost:8080 | grep -i server     # Server: nginx/1.27.0

# server_tokens off;
curl -I localhost:8080 | grep -i server     # Server: nginx
```

**ทำไมนี่คือ defense in depth ไม่ใช่การป้องกันจริง:**
คนที่ตั้งใจโจมตีระบุเวอร์ชันได้จากพฤติกรรมอื่นอยู่ดี (ลำดับ header, การตอบ request ที่ผิดรูปแบบ, TLS fingerprint)

สิ่งที่มันช่วยจริง ๆ คือ**ลดโอกาสถูกสแกนอัตโนมัติ** — บอทที่ไล่หาเวอร์ชันที่มีช่องโหว่จะข้ามเราไป
มีประโยชน์ แต่ **อย่าคิดว่าปลอดภัยเพราะซ่อนเวอร์ชัน** ที่สำคัญกว่าคือการอัปเดตให้ทัน

---

## N3.8 request id

```nginx
log_format main '... request_id=$request_id ...';

location /api/ {
    proxy_set_header X-Request-Id $request_id;
}
```

ฝั่ง Gin (แก้ `requestLogger()` ใน `internal/app/app.go`):

```go
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		entry, _ := json.Marshal(gin.H{
			"level":     "info",
			"requestId": c.GetHeader("X-Request-Id"),
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
		})
		log.Println(string(entry))
	}
}
```

ตามรอย:

```bash
docker compose logs nginx | grep request_id=abc123
docker compose logs api   | grep abc123
```

**ทำไมสำคัญ:** เมื่อมีผู้ใช้แจ้งว่า "ตอนบ่ายสองใช้ไม่ได้" การหา log ด้วยเวลาอย่างเดียวคือการงมเข็มในมหาสมุทร
แต่ถ้ามี request id ที่ส่งกลับไปให้ผู้ใช้ในหน้า error ด้วย → ผู้ใช้บอก id มา แล้วเราตามได้ทันทีทุกชั้น

นี่คือรากฐานของ distributed tracing — พอมีหลาย service ก็แค่ส่ง id นี้ต่อไปเรื่อย ๆ (หรือใช้มาตรฐาน W3C Trace Context)

**ควรทำต่อ:** ส่ง request id กลับใน response header ด้วย `add_header X-Request-Id $request_id always;`

---

## 🎯 ต่อยอด

- ลอง Let's Encrypt ด้วย certbot แทน self-signed (ต้องมีโดเมนจริง)
- ตั้ง OCSP stapling แล้ววัดว่า handshake เร็วขึ้นไหม
- ลอง `ssl_early_data on` (0-RTT) แล้วอ่านว่ามีความเสี่ยง replay attack ยังไง
