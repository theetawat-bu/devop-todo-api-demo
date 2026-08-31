# เฉลย — 🌐 Nginx ระดับ 5

⬅️ [กลับไปที่โจทย์](../../nginx/05-expert.md)

> ระดับนี้เน้นการตัดสินใจและเอกสาร — ด้านล่างคือแนวคำตอบและเกณฑ์ประเมิน

---

## N5.1 zero-downtime reload

```bash
docker run --rm --network host williamyeh/wrk -t4 -c50 -d30s http://localhost:8080/healthz &
sleep 10
docker compose exec nginx nginx -s reload
wait
# Non-2xx or 3xx responses: 0
```

**กลไกที่เกิดขึ้นจริง:**

```
1. master process รับสัญญาณ SIGHUP
2. master อ่าน config ใหม่ — ถ้า syntax ผิด จะยกเลิกและใช้ config เดิมต่อ (ไม่พัง)
3. master สร้าง worker ชุดใหม่ที่ใช้ config ใหม่
4. master สั่ง worker เก่าให้ "graceful shutdown"
   → เลิกรับ connection ใหม่ แต่ทำงานที่ค้างอยู่จนจบ
5. worker เก่าจบงานแล้วดับตัวเอง
```

ช่วงหนึ่งจะมี worker สองชุดอยู่พร้อมกัน — เห็นได้จาก `docker compose exec nginx ps aux`

**ทำไม `nginx -s stop` ต่างกัน:** มันคือ fast shutdown — ตัด connection ที่ค้างทันที ผู้ใช้เห็น connection reset
(`nginx -s quit` คือ graceful stop ซึ่งรอ request จบก่อน)

**บทเรียนสำคัญ:** `nginx -t` ก่อน reload ไม่ใช่แค่นิสัยดี — ถ้า config ผิดตอน **restart** (ไม่ใช่ reload) nginx จะสตาร์ทไม่ขึ้นเลย = ระบบล่มทั้งหมด

---

## N5.2 แปลง config เป็น Ingress

ตารางที่ควรได้:

| nginx directive | Ingress annotation | ทำไม่ได้ → ต้องใช้อะไรแทน |
| --- | --- | --- |
| `limit_req rate=10r/s` | `limit-rps: "10"` | — |
| `limit_req burst=20` | `limit-burst-multiplier: "2"` | ตั้ง burst ตรง ๆ ไม่ได้ ได้แค่ตัวคูณของ rps |
| `limit_conn 20` | `limit-connections: "20"` | — |
| `client_max_body_size 1m` | `proxy-body-size: "1m"` | — |
| `proxy_read_timeout 30s` | `proxy-read-timeout: "30"` | — |
| `least_conn` | `load-balance: "ewma"` | ไม่มี least_conn ตรง ๆ — ewma ใกล้เคียงที่สุด |
| `ip_hash` | `upstream-hash-by: "$remote_addr"` | — |
| `proxy_cache` | ❌ | **ไม่มี** — ต้องใช้ CDN, Varnish, หรือ cache ในแอป |
| `proxy_cache_lock` | ❌ | **ไม่มี** — ต้องแก้ที่แอป |
| `split_clients` (canary) | `canary: "true"` + `canary-weight` | ทำได้แต่ต้องสร้าง Ingress แยกอีกตัว |
| `map` + logic ซับซ้อน | ❌ | ต้องใช้ `configuration-snippet` ซึ่ง**หลายองค์กรปิดไว้เพราะเสี่ยง** |
| `sub_filter` | ❌ | ต้องแก้ที่แอป |
| log format ที่กำหนดเอง | ตั้งได้ที่ ConfigMap ของ controller | **เป็นค่าระดับคลัสเตอร์ ไม่ใช่ต่อ Ingress** |

**คอลัมน์ที่ 3 คือของจริง** — มันบอกว่าอะไรจะ "หายไป" ถ้าย้ายขึ้น k8s แบบไม่คิด

**สิ่งที่ควรสรุปได้:** Ingress ครอบคลุมงาน 80% ที่ใช้บ่อย แต่ตัวที่ทำไม่ได้มักเป็นตัวที่ระบบเราพึ่งพาที่สุด (โดยเฉพาะ cache)
→ ต้องวางแผนย้ายงานนั้นไปชั้นอื่นตั้งแต่ก่อนย้าย ไม่ใช่ค่อยมารู้ตอน migrate เสร็จแล้ว

---

## N5.3 ADR เรื่อง rate limit

โครงที่ควรได้:

```markdown
# ADR-012: ตำแหน่งของ rate limiting

## บริบท
- ingress-nginx ปัจจุบันมี 3 replica และมี HPA ปรับได้ถึง 6
- ตั้ง limit-rps: 10 → limit จริงคือ 30-60 rps ต่อ IP ไม่ใช่ 10
- ทีมขายตกลง SLA กับลูกค้าไว้ที่ 20 rps ต่อ API key
- ปัจจุบันไม่มี Redis ในระบบ

## ปัญหา
1. ตัวเลขที่บังคับใช้จริงไม่ตรงกับที่ตั้งใจ และเปลี่ยนตามจำนวน replica
2. Ingress ไม่รู้จัก API key จึงบังคับ SLA ต่อลูกค้าไม่ได้

## ทางเลือก
| ทางเลือก | บังคับ SLA ได้ | ค่าใช้จ่าย | เวลา implement |
|---|---|---|---|
| A. ตั้ง limit-rps เป็น 20/replica แล้วล็อก replica | บางส่วน | ไม่มี | 1 วัน |
| B. เพิ่ม Redis + middleware rate-limit ของ Gin (เช่น `ulule/limiter`) | ✅ | Redis 1 instance | 1 สัปดาห์ |
| C. ใส่ API Gateway (Kong) | ✅ | สูง + ต้องเรียนรู้ | 1 เดือน |

## การตัดสินใจ
เลือก B — ทำสองชั้น: ingress ตั้งหลวม 100rps กัน bot + Redis ในแอปบังคับ SLA 20rps ต่อ key

## เหตุผล
- SLA ผูกกับสัญญาลูกค้า จึงต้องแม่นระดับ key ซึ่งชั้น ingress ทำไม่ได้โดยธรรมชาติ
- Redis จะถูกใช้ต่อในงาน session และ cache อยู่แล้วในไตรมาสหน้า จึงไม่ใช่ต้นทุนที่เสียเปล่า
- ทางเลือก C เกินความจำเป็นสำหรับ 12 บริการ

## ผลที่ตามมา
- เพิ่ม dependency ใหม่ (Redis ล่ม = ต้องตัดสินใจว่า fail-open หรือ fail-closed)
- **ตัดสินใจไว้ล่วงหน้า: fail-open** — ยอมให้ traffic ผ่านดีกว่าปฏิเสธผู้ใช้ทั้งหมด

## จะทบทวนเมื่อ
- จำนวนบริการเกิน 30 → พิจารณา API Gateway อีกครั้ง
- มีลูกค้าที่ต้องการ quota แบบซับซ้อนกว่า rps
```

**เกณฑ์ผ่าน:**

- ✅ มีตัวเลข replica จริงและคำนวณ limit จริงให้เห็น
- ✅ ตอบคำถาม "Redis ล่มแล้วยังไง" ไว้ล่วงหน้า — **ข้อนี้แยกคนที่คิดจบกับคนที่คิดไม่จบ**
- ✅ มีเงื่อนไขทบทวน

---

## N5.4 mTLS

```bash
# สร้าง CA
openssl req -x509 -newkey rsa:2048 -nodes -days 365 -keyout ca.key -out ca.crt -subj "/CN=internal-ca"
# cert ของ nginx (ทำหน้าที่เป็น client เมื่อคุยกับ backend)
openssl req -newkey rsa:2048 -nodes -keyout nginx.key -out nginx.csr -subj "/CN=nginx"
openssl x509 -req -in nginx.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out nginx.crt -days 365
```

```nginx
location /api/ {
    proxy_pass https://api_backend;
    proxy_ssl_certificate     /etc/nginx/certs/nginx.crt;
    proxy_ssl_certificate_key /etc/nginx/certs/nginx.key;
    proxy_ssl_trusted_certificate /etc/nginx/certs/ca.crt;
    proxy_ssl_verify on;
}
```

ฝั่ง backend (Go) ต้องเปิด TLS พร้อมบังคับ client cert แทน `http.ListenAndServe` ธรรมดา:

```go
caPool := x509.NewCertPool()
caCert, _ := os.ReadFile("/etc/certs/ca.crt")
caPool.AppendCertsFromPEM(caCert)

srv := &http.Server{
	Addr:    ":3000",
	Handler: r, // *gin.Engine
	TLSConfig: &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
	},
}
srv.ListenAndServeTLS("/etc/certs/server.crt", "/etc/certs/server.key")
```

**กันอะไร:** คนที่เข้ามาอยู่ในเครือข่ายภายในได้แล้ว (lateral movement) จะยิงตรงไปที่ backend ไม่ได้ เพราะไม่มี cert — `tls.RequireAndVerifyClientCert` บังคับให้ handshake ล้มเหลวทันทีถ้า client (ในที่นี้คือ nginx) ไม่ยื่น cert ที่เซ็นโดย CA ใน `ClientCAs`
เป็นการยืนยันว่า **"traffic นี้มาจาก proxy ของเราจริง"** ไม่ใช่แค่ "มาจากใครสักคนในวงเดียวกัน"

**ในทางปฏิบัติ:** ถ้าอยู่บน Kubernetes การทำ mTLS ด้วยมือแบบนี้เจ็บปวดมาก (ต้องหมุน cert เอง)
→ ใช้ **service mesh** ที่ทำ mTLS อัตโนมัติพร้อมหมุน cert ให้ (Linkerd ทำได้แทบไม่ต้องตั้งค่า)

---

## N5.5 traffic ผิดปกติ

```bash
# bot จาก IP เดียว
docker run --rm --network host williamyeh/wrk -t2 -c200 -d60s http://localhost:8080/api/todos &
# ผู้ใช้ปกติ
for i in $(seq 1 100); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/api/todos; sleep 1; done
```

การตั้งค่าที่ทำให้ผ่านเกณฑ์:

```nginx
limit_req_zone  $binary_remote_addr zone=per_ip:10m rate=10r/s;
limit_conn_zone $binary_remote_addr zone=conn:10m;

location /api/ {
    limit_req  zone=per_ip burst=20 nodelay;
    limit_conn conn 10;                        # ← ตัวที่สำคัญที่สุดในสถานการณ์นี้
}
```

**ทำไม `limit_conn` สำคัญกว่า `limit_req` ในเคสนี้:** bot ที่เปิด 200 connection พร้อมกันจะกิน worker slot ของ nginx
แม้ request จะถูกตอบ 429 แต่ connection ยังถูกยึดไว้ → ผู้ใช้ปกติต่อไม่ติดตั้งแต่ระดับ TCP
`limit_conn` ตัดปัญหานี้ที่ต้นทาง

**ผลข้างเคียงที่ต้องบันทึก:** ผู้ใช้หลัง NAT เดียวกัน (ออฟฟิศ, มหาวิทยาลัย, มือถือ) จะแชร์ IP กัน → อาจโดนลูกหลง
→ ในระบบจริงต้องมี allowlist สำหรับ IP ขององค์กรลูกค้า หรือย้ายไป limit ตาม identity แทน IP

---

## N5.6 observability ของ edge

**metric ที่ต้องเก็บ:**

| metric | ทำไม |
| --- | --- |
| `request_time` p50/p95/p99 | ผู้ใช้เจออะไรจริง (ค่าเฉลี่ยโกหกเสมอ) |
| `upstream_response_time` p95 | แยกว่าแอปช้าหรือ network ช้า |
| **`request_time - upstream_response_time`** | เวลาที่หายไปนอกแอป — **ตัวชี้ขาด** |
| อัตรา 5xx / 4xx / 429 แยกกัน | 429 พุ่งอาจแปลว่า limit เข้มเกิน ไม่ใช่ถูกโจมตี |
| active connections vs worker_connections | ใกล้ชนเพดานหรือยัง |
| `accepts` vs `handled` | ไม่เท่ากัน = มีคนถูกปฏิเสธตั้งแต่ TCP |
| cache hit ratio | cache ได้ผลจริงไหม |

**alert ที่ควรตั้ง (พร้อมเหตุผลของ threshold):**

| alert | threshold | ทำไมเลขนี้ |
| --- | --- | --- |
| 5xx rate | > 1% นาน 5 นาที | ต่ำกว่านี้อาจเป็น bot ยิง path มั่ว |
| p95 latency | > 2× ค่าปกติ นาน 10 นาที | ใช้ค่าสัมพัทธ์ ไม่ใช่ค่าตายตัว เพราะแต่ละ endpoint ต่างกัน |
| 429 rate | > 5% | ผู้ใช้จริงกำลังโดนลูกหลง |
| accepts ≠ handled | ใด ๆ | ไม่ควรเกิดเลย |

**เกณฑ์ผ่านที่แท้จริง:** ให้เพื่อนร่วมทีมใช้ dashboard นี้ debug ปัญหาจริงได้โดยไม่ต้องถามคุณ
dashboard ที่มีกราฟ 40 อันแต่ไม่มีใครรู้ว่าดูอันไหนก่อน = ไม่มีประโยชน์

---

## N5.7 ยังต้องมี nginx ไหมบน k8s

**คำตอบที่ถูกในกรณีส่วนใหญ่: ไม่ต้อง** — ย้าย config ไปเป็น Ingress annotation

เหตุผล:

- ingress-nginx **คือ nginx** อยู่แล้ว การวาง nginx ของเราอีกชั้นคือ nginx ซ้อน nginx
- ได้ nginx สองชั้น = timeout สองชุด, log สองที่, health check สองระบบ → debug ยากขึ้นเท่าตัวโดยไม่ได้อะไรเพิ่ม
- config ที่เป็น annotation อยู่ใน git และ review ได้ ต่างจากไฟล์ที่ mount เข้า container

**ข้อยกเว้นที่ยอมรับได้:**

| กรณี | เหตุผล |
| --- | --- |
| ต้องใช้ `proxy_cache` | Ingress ทำไม่ได้ และ CDN ไม่เหมาะ (เช่นข้อมูลภายใน) |
| เสิร์ฟ static file จาก volume เดียวกับแอป | sidecar สมเหตุสมผล |
| องค์กรห้ามใช้ `configuration-snippet` แต่เราต้องการ logic ซับซ้อน | ไม่มีทางเลือกอื่น |

**config ที่จะ "สูญหาย" ตอนย้าย และแผนรับมือ:**

| สูญหาย | แผน |
| --- | --- |
| proxy_cache | ย้ายไป Redis cache ในแอป หรือใส่ CDN หน้า ingress |
| cache_lock | ทำ single-flight ในโค้ดแอป |
| rate limit ที่แม่นยำ | ย้ายไปแอป + Redis (ดู N5.3) |
| log format กำหนดเอง | ต้องคุยกับทีม platform เพราะเป็นค่าระดับคลัสเตอร์ |

**เกณฑ์ผ่าน:** ข้อเสนอต้องมีตารางนี้ ไม่ใช่แค่บอกว่า "ย้ายไป Ingress"

---

## 🎯 ประเมินตัวเอง

- [ ] อธิบายกลไก reload ได้โดยไม่เปิดเอกสาร
- [ ] รู้ว่า Ingress ทำอะไรไม่ได้บ้าง และมีแผนรองรับ
- [ ] การตัดสินใจทุกข้อมีตัวเลขรองรับ
- [ ] เคยตอบคำถาม "ถ้าสิ่งนี้ล่มแล้วยังไง" สำหรับทุก dependency ที่เพิ่มเข้ามา
