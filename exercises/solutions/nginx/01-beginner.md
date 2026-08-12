# เฉลย — 🌐 Nginx ระดับ 1

⬅️ [กลับไปที่โจทย์](../../nginx/01-beginner.md)

---

## N1.1 เส้นทาง request

```
curl → localhost:8080
     → docker port mapping 8080:80
     → nginx container port 80
     → server { listen 80; }
     → location /api/           ← เลือกบล็อกนี้เพราะ path ขึ้นต้นด้วย /api/
     → proxy_pass http://api_backend
     → upstream api_backend { server api:3000; }
     → Docker DNS แปลง "api" เป็น IP ของ container
     → Express app port 3000
     → todosRouter → prisma → postgres
```

ยืนยันด้วยตัวเอง:

```bash
docker compose logs nginx | tail -1     # เห็น upstream=172.x.x.x:3000
docker compose logs api  | tail -1      # เห็น log ของ Express request เดียวกัน
```

---

## N1.2 rate limit zone อยู่ไหน

| ไฟล์ | มีอะไร |
| --- | --- |
| `nginx/nginx.conf` | `limit_req_zone` — **ประกาศ** zone (ต้องอยู่ในบล็อก `http`) |
| `nginx/conf.d/default.conf` | `limit_req zone=api_limit …` — **บังคับใช้** (อยู่ในบล็อก `location`) |

**ทำไมต้องแยก:** `limit_req_zone` จองหน่วยความจำร่วมสำหรับเก็บสถานะของทุก IP ซึ่งเป็นทรัพยากรระดับ process
จึงต้องประกาศครั้งเดียวในระดับ `http` แล้ว `server`/`location` ไหนก็เรียกใช้ zone เดียวกันได้

ผลพลอยได้ที่มีประโยชน์: หลาย location แชร์ zone เดียวกันได้ → นับรวมกัน หรือแยก zone → นับแยก

---

## N1.3 endpoint ของ nginx เอง

```nginx
location = /ping {
    access_log off;
    default_type text/plain;
    return 200 "pong\n";
}
```

```bash
docker compose exec nginx nginx -t
docker compose exec nginx nginx -s reload
curl localhost:8080/ping
```

**หมายเหตุ:** ถ้า mount config เป็น `:ro` การแก้ไฟล์บนเครื่องมีผลทันที (เพราะ mount) แต่ nginx ยังใช้ config เก่าในหน่วยความจำจนกว่าจะ reload

**เกร็ด:** วาง `add_header Content-Type` คู่กับ `return` มักไม่ทำงานอย่างที่คิด — ใช้ `default_type` แทนจะตรงกว่า

---

## N1.4 อ่าน log

จาก log format ที่เราตั้งไว้:

```
172.20.0.1 - 200 "GET /api/todos HTTP/1.1" upstream=172.20.0.4:3000 rt=0.015 urt=0.012
```

| ค่า | ความหมาย |
| --- | --- |
| `upstream=` | IP:port ของ backend ที่รับงานจริง — ใช้ตรวจว่ากระจายดีไหม |
| `rt=` (`$request_time`) | เวลาทั้งหมดที่ nginx ใช้ นับตั้งแต่รับ byte แรกจนส่ง byte สุดท้าย |
| `urt=` (`$upstream_response_time`) | เวลาที่ backend ใช้ล้วน ๆ |

**`rt - urt` คือเวลาที่หายไปกับ:** เวลาอ่าน request จาก client, เวลาเปิด connection ไป upstream, เวลาส่ง response กลับให้ client (ถ้าเน็ต client ช้า ค่านี้จะสูง)

**ใช้ debug ยังไง:**

- `urt` สูง → แอปช้า ไปดูที่แอป/DB
- `urt` ต่ำ แต่ `rt` สูง → ปัญหาอยู่ที่ network ระหว่าง client กับ nginx หรือ response ใหญ่เกินไป

นี่คือการวินิจฉัยที่ทำได้จาก log อย่างเดียวโดยไม่ต้องมี APM

---

## N1.5 502

```bash
docker compose stop api
curl -i localhost:8080/api/todos          # HTTP/1.1 502 Bad Gateway
docker compose exec nginx cat /var/log/nginx/error.log | tail -3
# connect() failed (111: Connection refused) while connecting to upstream
docker compose start api
```

**ทำไม 502 ไม่ใช่ 503:**

| code | ความหมาย |
| --- | --- |
| **502 Bad Gateway** | nginx ติดต่อ upstream ไม่ได้ หรือ upstream ตอบมาผิดรูปแบบ |
| **503 Service Unavailable** | nginx เองปฏิเสธ เช่น ไม่มี upstream ที่ใช้ได้เลย หรือโดน limit |
| **504 Gateway Timeout** | ติดต่อได้แต่ตอบไม่ทันเวลา |

จำสามตัวนี้ให้แม่นแล้วจะ debug เร็วขึ้นมาก เพราะมันบอกว่าปัญหาอยู่ชั้นไหน

---

## N1.6 ลำดับความสำคัญของ location

nginx เลือกตามลำดับนี้ (ไม่ใช่ตามลำดับที่เขียนในไฟล์):

1. `location = /path` — **ตรงเป๊ะ ชนะทันที หยุดค้นหาเลย**
2. `location ^~ /path` — prefix ที่ยาวที่สุด ถ้าเจอแล้วข้าม regex
3. `location ~ /regex` หรือ `~*` — **ตามลำดับที่เขียนในไฟล์** ตัวแรกที่ match ชนะ
4. `location /path` — prefix ธรรมดา ที่ยาวที่สุดชนะ (ใช้เมื่อไม่มี regex ไหน match)

ในไฟล์ของเรา:

| บล็อก | ชนิด | ลำดับ |
| --- | --- | --- |
| `= /nginx-health` | ตรงเป๊ะ | 1 |
| `~ ^/(healthz\|readyz)$` | regex | 3 |
| `/api/` | prefix | 4 |
| `/` | prefix | 4 (สั้นสุด = แพ้เสมอถ้ามีตัวอื่น match) |

**ทดลองพิสูจน์:** เพิ่ม `location /api/todos { return 200 "specific\n"; }` แล้วยิง `/api/todos` → ได้ `specific` เพราะ prefix ยาวกว่า `/api/`

---

## N1.7 config ที่โหลดจริง

```bash
docker compose exec nginx nginx -T | less
docker compose exec nginx nginx -T | grep -n limit_req_zone
```

**`-T` vs `-t`:**

- `-t` = test อย่างเดียว บอกว่า syntax ถูกไหม
- `-T` = test + **พิมพ์ config ทั้งหมดที่ include เข้ามาแล้ว**

`-T` มีค่ามากเวลามี config หลายไฟล์แล้วไม่แน่ใจว่าค่าไหนถูกทับ — เห็นภาพรวมทั้งหมดในที่เดียว

---

## 🎯 ต่อยอด

- ลอง `curl -H "Host: something.else" localhost:8080/api/todos` แล้วดูว่า `server_name _;` ทำงานยังไง
- เปรียบเทียบ `proxy_pass http://api_backend;` กับ `proxy_pass http://api_backend/;` (ต่างกันเรื่อง path ที่ส่งต่อ)
- ลองปิด `access_log` แล้วดูว่า debug ยากขึ้นแค่ไหน
