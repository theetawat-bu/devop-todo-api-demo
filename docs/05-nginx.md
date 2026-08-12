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

อัลกอริทึมอื่นที่มีให้ใช้: `least_conn;` (ส่งไปตัวที่งานน้อยสุด), `ip_hash;` (IP เดิมไปเครื่องเดิม — sticky session)

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

## Header ที่ต้องส่งต่อ

```nginx
proxy_set_header Host              $host;
proxy_set_header X-Real-IP         $remote_addr;
proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
```

ถ้าไม่ใส่ แอปจะเห็น IP ของ nginx เป็น client ทุก request → log ผิด, rate limit ฝั่งแอปผิด, redirect ผิด protocol

ฝั่ง Express ต้องรับด้วย เราตั้งไว้ใน `src/app.ts`:

```ts
app.set("trust proxy", true); // ให้ req.ip อ่านจาก X-Forwarded-For
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

➡️ ต่อไป: [06 — GitHub Actions CI](06-github-actions-ci.md)
