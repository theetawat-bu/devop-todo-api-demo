# 05 — Nginx (Reverse Proxy)

## reverse proxy คืออะไร ทำไมต้องมี

client ไม่คุยกับแอปตรง ๆ แต่คุยกับ nginx แล้ว nginx ส่งต่อให้แอป ได้อะไร:

- **จุดเข้าเดียว** — มีหลาย service ก็ route ตาม path ได้ (`/api` → api, `/` → frontend)
- **Load balancing** — กระจายไปหลาย instance
- **TLS termination** — จบ HTTPS ที่ nginx แอปข้างในคุย HTTP ธรรมดา
- **Rate limiting** — กันยิงถล่ม ก่อนถึงแอป
- **ซ่อน backend** — ไม่ต้องเปิด port แอปออกสู่โลกภายนอก
- **เสิร์ฟ static / gzip / cache** — งานที่ nginx ทำได้ดีกว่า Node

## โครงสร้างไฟล์ config

| ไฟล์                        | หน้าที่                                                     |
| --------------------------- | ----------------------------------------------------------- |
| `nginx/nginx.conf`          | config หลัก — worker, log format, **นิยาม rate limit zone** |
| `nginx/conf.d/default.conf` | virtual host — upstream, location, **บังคับใช้ rate limit** |

nginx.conf มี `include /etc/nginx/conf.d/*.conf;` อยู่ท้าย `http {}` — เลยแยกไฟล์ต่อ site ได้

## Upstream & load balancing

```nginx
upstream api_backend {
    server api:3000;
    keepalive 32;
}
```

`api` คือชื่อ service ใน compose — Docker DNS คืน IP ของทุก replica ให้ nginx กระจายเอง (default = round-robin)

`keepalive 32` = เก็บ connection ไป upstream ไว้ใช้ซ้ำ ลดค่า handshake
ต้องใช้คู่กับสองบรรทัดนี้ในบล็อก location ไม่งั้นไม่มีผล:

```nginx
proxy_http_version 1.1;
proxy_set_header Connection "";
```

### เลือกอัลกอริทึม load balancing ตัวไหนดี

```nginx
upstream api_backend {
    least_conn;                                   # ← ใส่บรรทัดนี้เพื่อเปลี่ยนอัลกอริทึม
    server api:3000 max_fails=3 fail_timeout=10s;
    keepalive 32;
}
```

| อัลกอริทึม                         | วิธีเลือก backend                    | เหมาะกับ                                                               | ข้อควรระวัง                                                                                                              |
| ---------------------------------- | ------------------------------------ | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| **round-robin** (ค่าเริ่มต้น)      | วนไปทีละตัว                          | REST API ที่ทุก request ใช้เวลาใกล้ ๆ กัน — **เคสส่วนใหญ่ใช้ตัวนี้พอ** | ถ้าบาง request หนักกว่ามาก โหลดจะไม่สมดุล                                                                                |
| **`least_conn`**                   | ตัวที่มี connection ค้างน้อยสุด      | request ใช้เวลาต่างกันมาก (อัปโหลดไฟล์, รายงาน, long polling)          | ไม่ได้ดูว่า CPU ใครหนัก ดูแค่จำนวน connection                                                                            |
| **`ip_hash`**                      | hash จาก IP ผู้ใช้                   | ระบบเก่าที่เก็บ session ไว้ในหน่วยความจำของ instance                   | ผู้ใช้หลังเน็ตองค์กรจะออก IP เดียวกันหมด → กระจุก และ scale ไม่สวยเพราะเพิ่ม backend ทีเดียว mapping เปลี่ยนเกือบทั้งหมด |
| **`hash $arg_user_id consistent`** | hash จากค่าที่เราเลือกเอง            | cache ที่อยากให้ key เดิมไปเครื่องเดิม                                 | ต้องเลือก key ให้กระจายพอ                                                                                                |
| **`random two least_conn`**        | สุ่มมา 2 ตัว แล้วเลือกตัวที่ว่างกว่า | backend เยอะมาก / มี nginx หลายตัว                                     | ไม่จำเป็นถ้ามี backend แค่ 2-3 ตัว                                                                                       |

**สรุปสั้น ๆ:** เริ่มที่ round-robin เสมอ → ถ้าเห็นว่าโหลดไม่สมดุลจาก log (`urt=` ต่างกันมาก) ค่อยเปลี่ยนเป็น `least_conn`
ส่วน `ip_hash` ให้มองเป็น "ทางออกสุดท้ายของแอปที่ยังเก็บ session ในเครื่อง" — ทางที่ถูกกว่าคือย้าย session ไป Redis แล้วแอปจะ stateless ทันที

### Health check ของ Nginx — Passive vs Active

| แบบ         | ทำงานยังไง                                                   | มีใน                           | config                                          |
| ----------- | ------------------------------------------------------------ | ------------------------------ | ----------------------------------------------- |
| **Passive** | ดูจาก request จริงที่ล้มเหลว พังครบ N ครั้ง → พักไว้ชั่วคราว | **NGINX OSS (ที่เราใช้)**      | `server api:3000 max_fails=3 fail_timeout=10s;` |
| **Active**  | ยิงเช็ค backend เองเป็นระยะแม้ไม่มี traffic                  | NGINX Plus (เสียเงิน) เท่านั้น | `health_check interval=5s uri=/healthz;`        |

nginx ฟรีทำ active health check ไม่ได้ ทางแก้ในทางปฏิบัติคือใช้ passive + `proxy_next_upstream` คู่กัน:

```nginx
server api:3000 max_fails=3 fail_timeout=10s;      # ใน upstream
proxy_next_upstream error timeout http_502 http_503; # ใน location
```

ผลคือ request แรก ๆ ที่ชนเครื่องพังยัง "เสียฟรี" ไปบ้าง (แต่ผู้ใช้ไม่เห็น เพราะ nginx วิ่งไปตัวถัดไปให้)
ถ้าอยากได้ active health check จริง ๆ โดยไม่จ่ายเงิน → นี่คือหนึ่งในเหตุผลที่คนย้ายไป Kubernetes เพราะ **readinessProbe คือ active health check ที่ได้มาฟรี** (ดู [08](08-kubernetes.md))

## Rate limiting

นิยาม zone ใน `nginx.conf`:

```nginx
limit_req_zone  $binary_remote_addr zone=api_limit:10m rate=10r/s;
limit_conn_zone $binary_remote_addr zone=conn_limit:10m;
limit_req_status 429;
```

- `$binary_remote_addr` — นับแยกตาม IP (ใช้ binary เพราะกินหน่วยความจำน้อยกว่า)
- `10m` — พื้นที่เก็บสถานะ ~160,000 IP
- `rate=10r/s` — เฉลี่ย 10 request/วินาที/IP

เอาไปใช้ใน `conf.d/default.conf`:

```nginx
location /api/ {
    limit_req  zone=api_limit burst=20 nodelay;
    limit_conn conn_limit 20;
}
```

nginx ใช้อัลกอริทึม **leaky bucket**:

- ไม่ใส่ `burst` → เกิน 10r/s เมื่อไร ตัด 429 ทันที (โหดเกินไป เพราะ traffic จริงมาเป็นกระจุก)
- `burst=20` → มีถังพักได้อีก 20 request เกินมาก็ต่อคิวไว้ ปล่อยทีละตัวตาม rate
- `burst=20 nodelay` → 20 ตัวในถังปล่อยทันที ไม่หน่วง แต่ที่นั่งในถังจะค่อย ๆ คืนตาม rate → **ตัวนี้เหมาะกับ API มากที่สุด**

### ทำไม `/healthz` กับ `/readyz` ต้องหลุด rate limit

```nginx
location ~ ^/(healthz|readyz)$ {
    access_log off;
    proxy_pass http://api_backend;
}
```

ถ้า health check โดน 429 ระบบ monitoring จะคิดว่าแอปตายทั้งที่ยังดีอยู่ แล้วสั่ง restart วนไม่จบ
เป็นเคสคลาสสิกที่ทำให้ระบบล่มจากการป้องกันตัวเอง

### ⚠️ กับดักใหญ่: rate limit ของ nginx ไม่ได้แชร์กันข้ามเครื่อง

`limit_req_zone` เก็บสถานะไว้ใน **หน่วยความจำของ nginx process นั้นตัวเดียว**

```
nginx #1 (10r/s) ─┐
nginx #2 (10r/s) ─┼─▶ ผู้ใช้คนเดียวยิงได้จริง 30r/s ไม่ใช่ 10r/s
nginx #3 (10r/s) ─┘
```

ถ้ามี nginx / ingress controller หลาย replica ต้องเอา rate ที่ต้องการ **หารจำนวน replica** เอง
และถ้าจำนวน replica เปลี่ยนตาม HPA → limit จริงก็เปลี่ยนตามโดยที่เราไม่รู้ตัว

ถ้าต้องการ limit ที่แม่นจริง ๆ ต้องมีที่เก็บสถานะกลาง → ดูตารางเปรียบเทียบข้างล่าง

### ตั้ง rate limit ที่ชั้นไหนดี — เปรียบเทียบ

| ชั้น                | ตัวอย่าง                                                          | นับรวมข้ามเครื่องได้ | แยกตามผู้ใช้/แผนได้               | โหลดถึงแอปก่อนถูกตัดไหม        | ความยากในการดูแล      |
| ------------------- | ----------------------------------------------------------------- | -------------------- | --------------------------------- | ------------------------------ | --------------------- |
| **CDN / WAF**       | Cloudflare, AWS WAF                                               | ✅                   | ⚠️ ทำได้บ้าง                      | ❌ ตัดตั้งแต่ขอบสุด (ดีที่สุด) | ต่ำ (แต่มีค่าใช้จ่าย) |
| **Nginx / Ingress** | `limit_req`, `limit-rps`                                          | ❌ ต่อ replica       | ❌ รู้แค่ IP / header ดิบ         | ❌ ตัดก่อนถึงแอป               | ต่ำ                   |
| **API Gateway**     | Kong, APISIX, Tyk                                                 | ✅ (มี Redis)        | ✅ per API key / plan             | ❌ ตัดก่อนถึงแอป               | กลาง                  |
| **Service Mesh**    | Istio + Envoy RLS                                                 | ✅                   | ✅                                | ❌ ตัดก่อนถึง pod              | สูง                   |
| **ในแอป**           | rate-limit middleware ระดับแอป (เช่น token bucket ธรรมดา) + Redis | ✅ (ถ้าใช้ Redis)    | ✅ รู้ทุกอย่าง เช่น user id, tier | ✅ **โหลดถึงแอปแล้ว**          | ต่ำ แต่แอปรับภาระ     |

**วิธีที่ทีมส่วนใหญ่ใช้จริงคือทำสองชั้น ไม่ใช่เลือกอย่างเดียว:**

1. **ชั้นขอบ (nginx/ingress/CDN)** — ตั้งหลวม ๆ กัน volumetric attack และ bot เช่น 100r/s ต่อ IP
   จุดประสงค์คือ "อย่าให้ traffic ขยะถึงแอป" ไม่ใช่ความแม่นยำ
2. **ชั้นแอป (Redis)** — ตั้งตาม business rule เช่น free plan 1,000 req/วัน, pro plan 50,000 req/วัน
   จุดประสงค์คือความถูกต้องของ quota ซึ่งต้องรู้ว่าใครเป็นใคร — สิ่งที่ nginx ไม่มีทางรู้

หลักคิด: **สิ่งที่ต้องรู้ business context → ตั้งใกล้แอป / สิ่งที่แค่ต้องกันปริมาณ → ตั้งใกล้ผู้ใช้**
(ดูตารางเต็มข้ามทุกชั้นที่ [09 — ตั้งค่าที่ชั้นไหนดี](09-where-to-configure.md))

## Header ที่ต้องส่งต่อ

```nginx
proxy_set_header Host              $host;
proxy_set_header X-Real-IP         $remote_addr;
proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
```

ถ้าไม่ใส่ แอปจะเห็น IP ของ nginx เป็น client ทุก request → log ผิด, rate limit ฝั่งแอปผิด, redirect ผิด protocol

ฝั่ง Gin ต้องรับด้วย เราตั้งไว้ใน `internal/app/app.go`:

```go
r.SetTrustedProxies(nil) // ให้ c.ClientIP() อ่านจาก X-Forwarded-For เมื่ออยู่หลัง Nginx / Ingress
```

## Timeout & failover

```nginx
proxy_connect_timeout 5s;
proxy_read_timeout    30s;
proxy_next_upstream error timeout http_502 http_503 http_504;
```

`proxy_next_upstream` = ถ้า backend ตัวหนึ่งพัง ให้ลองตัวถัดไปอัตโนมัติ ผู้ใช้ไม่เห็น error

## ทดสอบจริง

```bash
# 1) config ถูกไหม (ทำก่อน reload เสมอ)
docker compose exec nginx nginx -t

# 2) reload โดยไม่ตัด connection ที่ค้างอยู่
docker compose exec nginx nginx -s reload

# 3) ยิงรัว ๆ ให้ติด rate limit — ควรเริ่มเห็น 429
for i in $(seq 1 60); do
  curl -s -o /dev/null -w "%{http_code} " http://localhost:8080/api/todos
done; echo

# 4) health check ต้องได้ 200 ตลอด ไม่โดน limit
curl -i http://localhost:8080/healthz

# 5) ดู log ว่ากระจายไป upstream ไหน ใช้เวลาเท่าไร
docker compose logs nginx | tail -20
```

## จาก Nginx ไป Ingress

พอขึ้น Kubernetes เราไม่เขียน `nginx.conf` เองแล้ว แต่ใช้ **ingress-nginx controller** ซึ่งก็คือ nginx ที่ generate config ให้อัตโนมัติจาก Ingress object
ของเดิมกลายเป็น annotation (ดู `k8s/base/ingress.yaml`):

| nginx.conf                | Ingress annotation                            |
| ------------------------- | --------------------------------------------- |
| `rate=10r/s`              | `nginx.ingress.kubernetes.io/limit-rps: "10"` |
| `burst=20`                | `limit-burst-multiplier: "3"`                 |
| `limit_conn 20`           | `limit-connections: "20"`                     |
| `client_max_body_size 1m` | `proxy-body-size: "1m"`                       |
| `proxy_read_timeout 30s`  | `proxy-read-timeout: "30"`                    |

เข้าใจ nginx ดิบ ๆ ก่อน แล้วจะอ่าน Ingress ออกทันที

## Nginx เอง vs Kubernetes — งานไหนควรอยู่ที่ใคร

พอขึ้น k8s หลายคนสับสนว่าจะยังเขียน nginx.conf เองไหม ตารางนี้ตอบให้:

| งาน                       | ทำที่ Nginx (compose/VM)      | ทำที่ Kubernetes                                 | ควรเลือกอันไหน                                                                                                   |
| ------------------------- | ----------------------------- | ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| **Load balance**          | `upstream` + อัลกอริทึม       | Service (L4) หรือ Ingress (L7)                   | อยู่บน k8s ให้ **Service** ทำ L4 พื้นฐาน แล้วใช้ **Ingress** เมื่อต้องการ L7 (path routing, gRPC, sticky cookie) |
| **Health check**          | passive เท่านั้น (OSS)        | liveness / readiness / startup probe             | **k8s ชนะขาด** — เป็น active, แยกความหมายชัด, และ "ฆ่า" กับ "ถอดออกจาก pool" แยกกันได้                           |
| **Rate limit**            | `limit_req` แม่นระดับ process | annotation ที่ Ingress (ก็คือ nginx ตัวเดียวกัน) | เหมือนกันในทางเทคนิค — ทั้งคู่มีปัญหา "ไม่แชร์ข้าม replica" เท่ากัน                                              |
| **TLS**                   | ต่อ cert เอง / certbot        | cert-manager ออก + ต่ออายุอัตโนมัติ              | **k8s + cert-manager** สบายกว่ามาก                                                                               |
| **Auto scale ตามโหลด**    | ทำเองไม่ได้                   | HPA                                              | **k8s เท่านั้น**                                                                                                 |
| **Retry / failover**      | `proxy_next_upstream`         | Ingress annotation หรือ Service Mesh             | ถ้าต้องการ circuit breaker, outlier detection จริงจัง → Service Mesh                                             |
| **Routing ตาม path/host** | `location` / `server_name`    | Ingress rules / Gateway API                      | เท่ากัน แต่บน k8s ควรใช้ Ingress เพื่อให้ config เป็น declarative อยู่ใน git                                     |

**สรุป:** บน Kubernetes **ไม่ควรยัด nginx container ของตัวเองไว้หน้า Service** (เป็น anti-pattern ที่เจอบ่อย)
เพราะจะได้ nginx สองชั้นซ้อนกัน — ingress-nginx อยู่แล้วชั้นหนึ่ง ของเราอีกชั้นหนึ่ง = debug ยากขึ้นเท่าตัว, timeout ซ้อนกัน, และ health check ตีกันเอง
ให้ย้าย config ที่เคยเขียนเองไปเป็น annotation ของ Ingress แทน

ยกเว้นกรณีเดียวที่ยอมรับได้: nginx เป็น **sidecar** ในงานเฉพาะทางจริง ๆ เช่นเสิร์ฟ static file ที่อยู่ใน volume เดียวกับแอป

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] ยิง `for i in $(seq 1 60); do curl -s -o /dev/null -w "%{http_code} " http://localhost:8080/api/todos; done` แล้วนับว่า 429 เริ่มโผล่ที่ request ที่เท่าไร ตรงกับ `burst=20` ไหม
- [ ] ลบ header `X-Forwarded-For` ออกจาก `proxy_set_header` แล้วดู log ของแอป — `c.ClientIP()` เปลี่ยนเป็น IP ของ nginx เองไหม
- [ ] เปลี่ยน `least_conn` เป็น `ip_hash` แล้วยิงจากเครื่องเดิมหลายรอบ สังเกตว่า request ไปตกที่ backend ตัวเดิมตลอดจริงไหม
- [ ] ลด `proxy_read_timeout` ให้สั้นลงมาก ๆ (เช่น `1s`) แล้วยิง endpoint ที่ตอบช้า ดูว่า nginx ตัดเมื่อไรและคืน status อะไร
- [ ] `docker compose exec nginx nginx -t` แล้วลองพิมพ์ syntax error ใน config ดูว่าข้อความ error บอกอะไรบ้าง

➡️ ต่อไป: [06 — GitHub Actions CI](06-github-actions-ci.md)
📊 อ่านคู่กัน: [09 — จะตั้ง LB / rate limit / health check ที่ชั้นไหนดี](09-where-to-configure.md)
🏋️ ฝึกมือ: [แบบฝึกหัด Nginx](../exercises/nginx/01-beginner.md)
