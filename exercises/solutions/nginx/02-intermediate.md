# เฉลย — 🌐 Nginx ระดับ 2

⬅️ [กลับไปที่โจทย์](../../nginx/02-intermediate.md)

---

## N2.1 พิสูจน์ load balance

```bash
docker compose up -d --scale api=4
for i in $(seq 1 20); do curl -s localhost:8080/api/todos > /dev/null; done
docker compose logs nginx | grep -o 'upstream=[0-9.]*:[0-9]*' | sort | uniq -c
#   5 upstream=172.20.0.4:3000
#   5 upstream=172.20.0.5:3000
#   5 upstream=172.20.0.6:3000
#   5 upstream=172.20.0.7:3000
```

**ทำไม nginx รู้จัก 4 ตัวทั้งที่ config เขียน `server api:3000;` บรรทัดเดียว:**
Docker DNS คืน **A record หลายรายการ** สำหรับชื่อ service ที่มีหลาย replica แล้ว nginx กระจายไปตามนั้น

⚠️ **กับดักสำคัญ:** nginx resolve DNS **แค่ตอนสตาร์ท/reload** เท่านั้น
ถ้า scale เพิ่มระหว่างที่ nginx รันอยู่ มันจะยังไม่เห็นตัวใหม่จนกว่าจะ reload

ทางแก้ในโปรดักชัน:

```nginx
resolver 127.0.0.11 valid=10s;              # DNS ของ Docker
set $upstream_api http://api:3000;
proxy_pass $upstream_api;                    # ใช้ตัวแปร = resolve ใหม่ตาม TTL
```

การใส่ตัวแปรใน `proxy_pass` บังคับให้ nginx resolve ใหม่เรื่อย ๆ — เป็นเทคนิคที่จำเป็นมากในสภาพแวดล้อมที่ IP เปลี่ยนบ่อย

---

## N2.2 round-robin vs least_conn

เพิ่ม endpoint ช้า:

```go
r.GET("/api/slow", func(c *gin.Context) {
	time.Sleep(3 * time.Second)
	c.JSON(http.StatusOK, gin.H{"ok": true})
})
```

ยิงผสม:

```bash
for i in $(seq 1 10); do curl -s localhost:8080/api/slow > /dev/null & done
for i in $(seq 1 20); do curl -s -w "%{time_total}\n" -o /dev/null localhost:8080/api/todos; done
```

| | round-robin | least_conn |
| --- | --- | --- |
| ทุก request เร็วเท่ากัน | ✅ เท่ากัน | ✅ เท่ากัน (ไม่ได้เปรียบ) |
| มี request ช้าปนอยู่ | request เร็วอาจไปต่อคิวหลัง request ช้า | ✅ หลบไป backend ที่ว่างกว่า |

**ทำไม:** round-robin นับแค่ "ตาใคร" ไม่สนใจว่าตัวนั้นยังทำงานค้างอยู่กี่งาน
`least_conn` ดูจำนวน connection ที่ยังไม่จบ — request ที่ค้างนานทำให้ตัวนั้นถูกข้ามไปเอง

**สรุปที่ใช้ตัดสินใจ:** ถ้า response time ของทุก endpoint ใกล้เคียงกัน round-robin เพียงพอและมี overhead น้อยกว่า
เปลี่ยนไป `least_conn` ต่อเมื่อเห็นจาก log ว่า `urt` กระจายตัวกว้างมาก

---

## N2.3 rate limit 3 แบบ

ผลที่ควรได้ (ตั้ง `rate=10r/s` ยิง 30 request):

| config | 200 | 429 | เวลารวม | อธิบาย |
| --- | --- | --- | --- | --- |
| `limit_req zone=api_limit;` | ~1-2 | ~28 | เร็วมาก | เกิน 10r/s ตัดทันที โหดเกินไปสำหรับ traffic จริง |
| `burst=20` | ~21 | ~9 | **~2 วินาที** | 20 ตัวเข้าคิวรอ ปล่อยทีละตัวตาม rate |
| `burst=20 nodelay` | ~21 | ~9 | **เร็วมาก** | 20 ตัวปล่อยทันที ที่นั่งในถังค่อย ๆ คืน |

**ทำไม `nodelay` เวลารวมสั้นกว่าทั้งที่จำนวน 429 เท่ากัน:**

- ไม่มี `nodelay` → request ในถังถูก **หน่วง** ให้ออกทีละตัวตาม rate (10r/s = ตัวละ 100ms) → 20 ตัวใช้เวลา 2 วินาที
- มี `nodelay` → ปล่อยทั้ง 20 ตัวทันที แต่ที่นั่งในถังจะคืนกลับมาในอัตรา 10 ที่/วินาที

**ผลลัพธ์ระยะยาวเท่ากัน แต่ประสบการณ์ผู้ใช้ต่างกันมาก** — ผู้ใช้ปกติที่บังเอิญคลิกรัวจะไม่รู้สึกว่าเว็บช้า
นี่คือเหตุผลที่ `burst=N nodelay` เป็นค่าที่แนะนำสำหรับ API

---

## N2.4 แยก limit ตาม method

ใน `nginx.conf` (บล็อก `http`):

```nginx
map $request_method $post_limit_key {
    POST    $binary_remote_addr;
    default "";                       # ค่าว่าง = ไม่ถูกนับ
}
limit_req_zone $post_limit_key zone=post_limit:10m rate=2r/s;
limit_req_zone $binary_remote_addr zone=get_limit:10m rate=20r/s;
```

ใน `conf.d/default.conf`:

```nginx
location /api/ {
    limit_req zone=get_limit  burst=40 nodelay;
    limit_req zone=post_limit burst=5  nodelay;
    proxy_pass http://api_backend;
}
```

**กุญแจสำคัญ:** ถ้า key ของ `limit_req_zone` เป็น**ค่าว่าง** nginx จะข้ามการนับไปเลย
เราจึงใช้ `map` แปลง method ที่ไม่ใช่ POST ให้เป็นค่าว่าง — ทำให้ zone นั้นมีผลเฉพาะ POST

**ใส่ `limit_req` หลายบรรทัดได้ไหม:** ได้ nginx จะตรวจทุกอันและ**ตัวที่เข้มที่สุดชนะ**

**ทำไมควรทำ:** POST มักแพงกว่า GET หลายเท่า (เขียน DB, ส่งอีเมล, เรียก API ภายนอก) การใช้ limit เดียวกันทั้งคู่จึงไม่สมเหตุสมผล

---

## N2.5 X-Forwarded-For

```bash
# ลบ proxy_set_header X-Forwarded-For ออก แล้ว reload
docker compose logs api | tail -1
# {"level":"info","method":"GET","path":"/api/todos","ip":"172.20.0.3"}   ← IP ของ nginx
```

**ถ้าทำ rate limit ในแอปโดยไม่มี header นี้:** ทุก request จะดูเหมือนมาจาก IP เดียวกัน (IP ของ nginx)
→ ผู้ใช้คนแรกที่ยิงถึงเพดานจะทำให้**ผู้ใช้ทุกคนถูกบล็อกพร้อมกัน** = สร้าง DoS ให้ตัวเองด้วยระบบป้องกัน DoS

ต้องมีทั้งสองฝั่ง:

```nginx
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

```go
r.SetTrustedProxies(nil) // ตั้งไว้ใน internal/app/app.go — ให้ Gin อ่านค่าจาก header นี้เมื่อมาจาก proxy ที่เชื่อถือ
```

⚠️ **ความปลอดภัย:** ถ้าตั้ง `SetTrustedProxies(nil)` Gin จะไม่เชื่อ `X-Forwarded-For` เลยและใช้ IP ของ connection จริงแทน (ปลอดภัยแต่ได้ IP ของ nginx เสมอ)
ถ้าต้องการให้ Gin อ่านค่า `X-Forwarded-For` จริง ๆ ใน production ที่เข้มงวดควรระบุ IP ของ proxy ที่เชื่อถือแทน เช่น `r.SetTrustedProxies([]string{"172.20.0.0/16"})` — ไม่ควรเชื่อ header นี้แบบไม่จำกัดเพราะผู้ใช้ปลอมได้ถ้ายิงตรงมาที่แอป

---

## N2.6 client_max_body_size

```bash
python3 -c "print('{\"title\":\"' + 'x'*2000000 + '\"}')" > /tmp/big.json
curl -i -X POST localhost:8080/api/todos \
  -H 'Content-Type: application/json' --data-binary @/tmp/big.json
# HTTP/1.1 413 Request Entity Too Large
# Server: nginx
```

ดูจาก header `Server: nginx` และ HTML error page ของ nginx (ไม่ใช่ JSON ของแอปเรา) → nginx เป็นคนตัด

**ทำไมตัดที่ nginx ดีกว่า:**

| | ตัดที่ nginx | ปล่อยให้ Gin รับ |
| --- | --- | --- |
| อ่านข้อมูลเข้ามาเท่าไร | หยุดทันทีที่รู้ว่าเกิน (จาก Content-Length) | ต้องอ่าน body ครบก่อนถึงจะรู้ |
| หน่วยความจำที่ใช้ | แทบไม่ใช้ | ใช้เต็มขนาด body |
| ถ้ามีคนยิง 1000 request × 100MB | nginx ตัดหมด แอปไม่รู้เรื่อง | แอปหน่วยความจำเต็ม ตาย |

**หลักการ: ตัดของที่ไม่ต้องการให้เร็วที่สุด และใกล้ผู้ใช้ที่สุด**

---

## N2.7 timeout

```nginx
proxy_read_timeout 5s;
```

```bash
time curl -i localhost:8080/api/slow      # 10 วินาที endpoint
# HTTP/1.1 504 Gateway Timeout
# real 0m5.0s
```

แต่ดู log ของแอป:

```bash
docker compose logs api | tail -2
# request มาถึงตอน t=0 และแอปทำงานจนครบ 10 วิ แล้วส่ง response ที่ไม่มีใครรับ
```

**นี่คือปัญหาจริงที่คนมองข้าม:**

```
t=0s   client ยิง → nginx ส่งต่อ → แอปเริ่มทำงาน
t=5s   nginx ตัด → client ได้ 504 → client อาจ retry ทันที
t=10s  แอปทำงานเสร็จ ส่ง response ให้ connection ที่ปิดไปแล้ว
```

ผลคือตอนโหลดสูง: client retry → แอปมีงานเพิ่มขึ้นเรื่อย ๆ ทั้งที่งานเก่าไม่มีใครรอ → **ตายด้วย retry storm**

**ทางแก้ที่ถูก:** ตั้ง timeout ในแอปให้**สั้นกว่า** nginx เสมอ (เช่น nginx 30s → แอป 25s → DB query 20s)
เพื่อให้แอปเป็นคนยอมแพ้ก่อนและปล่อยทรัพยากรคืน อ่านเพิ่ม: [docs/09](../../../docs/09-where-to-configure.md)

---

## N2.8 failover

```bash
docker compose up -d --scale api=3
# terminal 1
while true; do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/api/todos; sleep 0.1; done
# terminal 2
docker kill $(docker ps -q -f name=api | head -1)
```

ผลที่ควรได้: เห็น 502 ประมาณ 1-3 ตัวแล้วกลับมาเป็น 200 ทั้งหมด

**`proxy_next_upstream` ช่วยยังไง:** เมื่อ nginx เจอ error จาก backend ตัวหนึ่ง มันจะ**ลองตัวถัดไปทันทีในคำขอเดียวกัน** ผู้ใช้จึงไม่เห็น error

ทำไมยังมีบางตัวที่พลาด: connection ที่กำลังส่งข้อมูลอยู่แล้ว (nginx ได้รับ response header ไปแล้วบางส่วน) จะ retry ไม่ได้เพราะอาจทำให้ข้อมูลซ้ำ

เพิ่ม `max_fails=3 fail_timeout=10s` แล้ว nginx จะจำว่าตัวนั้นพัง แล้ว**ไม่ส่งไปหาอีกเลย 10 วินาที** → จำนวนที่พลาดลดลงเหลือแค่ 3 ตัวแรก

---

## 🎯 ต่อยอด

- ลองตั้ง `resolver` แล้วพิสูจน์ว่า scale เพิ่มระหว่างรันแล้ว nginx เห็นเองโดยไม่ต้อง reload
- ลอง `limit_req_status 503` แทน 429 แล้วคิดว่าอันไหนเหมาะกว่า (429 บอก client ว่า "ช้าลงหน่อย" ซึ่งตรงความหมายกว่า)
- วัดว่า `keepalive 32` ช่วยลด latency ได้จริงกี่ ms (ลองลบออกแล้วเทียบ)
