# เฉลย — 🌐 Nginx ระดับ 4

⬅️ [กลับไปที่โจทย์](../../nginx/04-advanced.md)

---

## N4.1 proxy cache

ใน `nginx.conf` (บล็อก `http`):

```nginx
proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=api_cache:10m
                 max_size=100m inactive=60s use_temp_path=off;
```

ใน `conf.d/default.conf`:

```nginx
location /api/todos {
    proxy_cache api_cache;
    proxy_cache_valid 200 10s;
    proxy_cache_key "$scheme$request_method$host$request_uri";
    proxy_cache_methods GET HEAD;
    add_header X-Cache-Status $upstream_cache_status always;
    proxy_pass http://api_backend;
}
```

```bash
curl -sI localhost:8080/api/todos | grep X-Cache    # MISS
curl -sI localhost:8080/api/todos | grep X-Cache    # HIT
```

**ทำไม POST/PATCH/DELETE ห้าม cache:**

- ไม่ใช่ idempotent — การส่งคำขอเดิมซ้ำอาจสร้างข้อมูลซ้ำ
- cache คือการ "ตอบแทน backend" ซึ่งแปลว่าคำสั่งเขียนไม่ไปถึง DB
- `proxy_cache_methods` ค่าเริ่มต้นคือ `GET HEAD` อยู่แล้ว — แต่การเขียนไว้ชัด ๆ ช่วยให้คนอ่านเข้าใจเจตนา

⚠️ **กับดักใหญ่:** ถ้า response ขึ้นกับผู้ใช้ (มี auth) การ cache ด้วย key ที่ไม่มี user id จะทำให้
**ผู้ใช้ A เห็นข้อมูลของผู้ใช้ B** — เป็นบั๊กความปลอดภัยร้ายแรงที่เกิดจากการเปิด cache แบบไม่คิด

ถ้ามี auth ต้องใส่ใน key: `proxy_cache_key "$scheme$request_method$host$request_uri$http_authorization";`
หรือดีกว่านั้นคือ **อย่า cache ที่ proxy เลย** ให้ cache ในแอปที่รู้ว่าข้อมูลเป็นของใคร

---

## N4.2 stale-while-revalidate

```nginx
proxy_cache_use_stale error timeout updating http_500 http_502 http_503 http_504;
proxy_cache_background_update on;
```

```bash
docker compose stop api
curl -sI localhost:8080/api/todos | grep X-Cache    # STALE — ยังได้ 200
```

**ทำไมนี่คือฟีเจอร์ที่คุ้มค่าที่สุดข้อหนึ่ง:** ตอน backend ล่ม ผู้ใช้เห็นข้อมูลเก่า 10 วินาที
แทนที่จะเห็นหน้า error — สำหรับข้อมูลที่ไม่ต้องสด 100% นี่คือการแลกที่คุ้มมาก

`proxy_cache_background_update on` เสริมอีกชั้น: พอ cache หมดอายุ nginx ส่งของเก่าให้ผู้ใช้ทันที
แล้วไปดึงของใหม่เบื้องหลัง → **ไม่มีผู้ใช้คนไหนต้องรอ cache miss เลย**

**ต้องคิดก่อนใช้:** ข้อมูลไหนที่เก่าได้ ข้อมูลไหนที่เก่าไม่ได้ (ยอดเงินคงเหลือ, สต็อกสินค้า)

---

## N4.3 cache stampede

```nginx
proxy_cache_lock on;
proxy_cache_lock_timeout 5s;
proxy_cache_lock_age 5s;
```

ทดสอบ:

```bash
docker compose exec nginx sh -c 'rm -rf /var/cache/nginx/*' && docker compose restart nginx
seq 1 100 | xargs -P100 -I{} curl -s -o /dev/null localhost:8080/api/todos
docker compose logs api | grep "/api/todos" | wc -l    # ควรได้ 1
```

**ปัญหาที่แก้:** cache หมดอายุพอดีตอนมี traffic สูง → ทุก request พร้อมกันเห็น MISS → **ทุกตัววิ่งไป backend พร้อมกัน**
backend ที่ปกติรับ 10 rps จู่ ๆ ได้ 1,000 rps ในเสี้ยววินาที → ล่ม → cache ยิ่งไม่ถูกเติม → ล่มซ้ำ

`proxy_cache_lock` ให้ request แรกเท่านั้นที่ไปหา backend ที่เหลือรอผลของตัวแรก

**นี่คือปัญหาที่เจอตอนระบบโตแล้วเท่านั้น** — ตอนทดสอบคนเดียวไม่มีทางเจอ ซึ่งเป็นเหตุผลที่ต้องรู้ไว้ล่วงหน้า

---

## N4.4 limit ตาม API key

```nginx
limit_req_zone $http_x_api_key zone=key_limit:10m rate=5r/s;

location /api/ {
    limit_req zone=key_limit burst=10 nodelay;
}
```

**ปัญหา:** คนที่ไม่ส่ง `X-API-Key` จะได้ key เป็นค่าว่าง → nginx **ไม่นับเลย** → ไม่มี limit สำหรับคนที่ไม่ส่ง key

ทางแก้ที่ถูกต้อง — บังคับให้ต้องมี key และ fallback ไปนับตาม IP:

```nginx
map $http_x_api_key $limit_key {
    ""      $binary_remote_addr;    # ไม่มี key → นับตาม IP แทน
    default $http_x_api_key;
}
limit_req_zone $limit_key zone=smart_limit:10m rate=5r/s;
```

หรือปฏิเสธไปเลยถ้าไม่มี key:

```nginx
if ($http_x_api_key = "") { return 401; }
```

**ข้อจำกัดที่ต้องรู้:** nginx ตรวจได้แค่ว่า header **มีอยู่** ไม่รู้ว่า key นั้น**ถูกต้องหรือไม่**
ผู้โจมตีส่ง key มั่ว ๆ ที่ไม่ซ้ำกันทุก request → หลบ limit ได้ทั้งหมด
→ **การ limit ตาม API key ที่ปลอดภัยจริงต้องทำที่ชั้นที่ตรวจสอบ key ได้** (API gateway หรือแอป) ตามที่ [docs/09](../../../docs/09-where-to-configure.md) อธิบายไว้

---

## N4.5 metrics

```nginx
location = /nginx_status {
    stub_status;
    access_log off;
    allow 127.0.0.1;
    allow 172.16.0.0/12;
    deny all;
}
```

```bash
docker compose exec nginx curl -s localhost/nginx_status
# Active connections: 3
# server accepts handled requests
#  128 128 1024
# Reading: 0 Writing: 1 Waiting: 2
```

| ค่า | ความหมาย | ดูเพื่ออะไร |
| --- | --- | --- |
| Active connections | connection ที่เปิดอยู่ | ใกล้ `worker_connections × worker_processes` หรือยัง |
| accepts vs handled | ถ้าไม่เท่ากัน = มี connection ถูกปฏิเสธ | **สัญญาณว่าชน worker_connections** |
| Waiting | keepalive ที่ว่างอยู่ | ปกติสูงได้ ไม่ใช่ปัญหา |

**accepts ≠ handled คือ alert ที่ต้องตั้ง** — แปลว่ามีผู้ใช้ถูกปฏิเสธตั้งแต่ระดับ TCP

ต่อยอด: `nginx-prometheus-exporter` แปลงค่าพวกนี้เป็น metric ของ Prometheus

---

## N4.6 หาคอขวด

```bash
docker run --rm --network host williamyeh/wrk -t4 -c100 -d30s http://localhost:8080/api/todos
```

**วิธีหาคอขวดอย่างเป็นระบบ** — วัดทีละชั้นจากในออกนอก:

```bash
# 1) แอปเปล่า ๆ (ข้าม nginx)
wrk -t4 -c100 -d30s http://localhost:3000/healthz      # ได้เท่าไร?
# 2) ผ่าน nginx
wrk -t4 -c100 -d30s http://localhost:8080/healthz      # ลดลงไหม?
# 3) endpoint ที่แตะ DB
wrk -t4 -c100 -d30s http://localhost:8080/api/todos    # ลดลงมากไหม?
```

| ผลที่เจอ | คอขวดอยู่ที่ |
| --- | --- |
| 1 กับ 2 ใกล้กัน, 3 ต่ำกว่ามาก | **database** |
| 2 ต่ำกว่า 1 มาก | **nginx** (จูน worker/keepalive) |
| ทั้งสามใกล้กันและต่ำ | **แอปหรือ CPU ของเครื่อง** |

การจูนที่มักได้ผล:

```nginx
worker_processes auto;
worker_rlimit_nofile 65535;
events { worker_connections 4096; use epoll; multi_accept on; }
upstream api_backend { keepalive 64; }   # เพิ่มจาก 32
```

**สิ่งสำคัญที่สุดของข้อนี้:** พอจูนเสร็จแล้ว **คอขวดจะย้ายไปที่อื่นเสมอ** ไม่ใช่หายไป
งานของเราคือรู้ว่าตอนนี้มันอยู่ที่ไหน ไม่ใช่ไล่จูนไปเรื่อย ๆ โดยไม่วัด

---

## N4.7 canary

```nginx
split_clients "${remote_addr}${request_id}" $api_pool {
    10%     api_new;
    *       api_old;
}

upstream api_old { server api-v1:3000; }
upstream api_new { server api-v2:3000; }

location /api/ {
    proxy_pass http://$api_pool;
    add_header X-Backend-Pool $api_pool always;
}
```

```bash
for i in $(seq 1 100); do curl -sI localhost:8080/api/todos | grep X-Backend-Pool; done | sort | uniq -c
#  90 X-Backend-Pool: api_old
#  10 X-Backend-Pool: api_new
```

**ข้อจำกัดเทียบกับ service mesh:**

| | nginx `split_clients` | Argo Rollouts / Service Mesh |
| --- | --- | --- |
| แบ่ง % ได้ | ✅ | ✅ |
| ปรับ % อัตโนมัติตามผล | ❌ ต้องแก้ config + reload เอง | ✅ |
| ย้อนกลับอัตโนมัติเมื่อ error สูง | ❌ | ✅ วัดจาก metric แล้วตัดสินใจเอง |
| ผู้ใช้คนเดิมได้ pool เดิมตลอด | ✅ ถ้าใช้แค่ `$remote_addr` เป็น key | ✅ |

ใช้ `${remote_addr}${request_id}` = สุ่มทุก request (ดีสำหรับ API)
ใช้ `${remote_addr}` อย่างเดียว = ผู้ใช้คนเดิมเห็นเวอร์ชันเดิมตลอด (ดีสำหรับหน้าเว็บ ไม่งั้น UI จะสลับไปมา)

---

## N4.8 limit หลายชั้น

```nginx
limit_req_zone $binary_remote_addr zone=per_ip:10m  rate=10r/s;
limit_req_zone $http_x_api_key     zone=per_key:10m rate=100r/s;
limit_req_zone $server_name        zone=global:10m  rate=1000r/s;

location /api/ {
    limit_req zone=per_ip  burst=20  nodelay;
    limit_req zone=per_key burst=200 nodelay;
    limit_req zone=global  burst=2000;
}
```

**ลำดับการตรวจ:** nginx ตรวจ**ทุก** `limit_req` ที่ประกาศไว้ และ **ตัวที่เข้มที่สุดชนะ**
ไม่ได้ตรวจตามลำดับที่เขียนแล้วหยุดที่ตัวแรก

**เหตุผลของการซ้อนสามชั้น:**

| ชั้น | กันอะไร |
| --- | --- |
| per IP | ผู้ใช้คนเดียวยิงรัว |
| per API key | ลูกค้ารายเดียวใช้เกินโควตา (ต่อให้ยิงจากหลาย IP) |
| global | โหลดรวมเกินที่ระบบรับไหว — ป้องกัน backend ล่มเป็นด่านสุดท้าย |

ชั้น global ตั้งใจไม่ใส่ `nodelay` เพื่อให้ **หน่วง** แทนการปฏิเสธ — ชะลอทั้งระบบดีกว่าให้ล่มทั้งระบบ

---

## 🎯 ต่อยอด

- ใช้ `proxy_cache_bypass $http_cache_control` เพื่อให้บังคับ refresh ได้
- ลอง `limit_req_dry_run on` เพื่อดูว่าจะโดนตัดแค่ไหนโดยยังไม่ตัดจริง — ปลอดภัยมากสำหรับการทดสอบใน production
- วัด hit rate ของ cache แล้วคำนวณว่าลดโหลด backend ได้กี่ %
