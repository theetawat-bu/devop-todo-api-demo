# 🌐 Nginx — ระดับ 4: ขั้นสูง

> **เป้าหมาย:** ใช้ nginx ลดภาระ backend จริงจัง — cache, กัน stampede, จูนจนเจอคอขวด
> **ต้องมีก่อน:** ผ่านระดับ 3
> **เวลาโดยประมาณ:** 1–2 วัน

---

### N4.1 proxy cache

**โจทย์:** cache GET `/api/todos` ไว้ 10 วินาที
**คำใบ้:**

```nginx
# ในบล็อก http
proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=api_cache:10m max_size=100m inactive=60s;

# ในบล็อก location
proxy_cache api_cache;
proxy_cache_valid 200 10s;
add_header X-Cache-Status $upstream_cache_status;
```

**ผ่านเมื่อ:** ยิงสองครั้งติดเห็น `MISS` แล้ว `HIT` และอธิบายได้ว่าทำไม POST/PATCH/DELETE ห้าม cache

---

### N4.2 stale-while-revalidate

**โจทย์:** ทำให้เวลา backend ล่ม nginx ยังเสิร์ฟของเก่าจาก cache ได้
**คำใบ้:** `proxy_cache_use_stale error timeout updating http_500 http_502 http_503;`
**ผ่านเมื่อ:** `docker compose stop api` แล้วยังได้ 200 พร้อม `X-Cache-Status: STALE`

---

### N4.3 กัน cache stampede

**โจทย์:** 100 request พร้อมกันขอของที่ยังไม่มีใน cache — ทำยังไงให้ backend โดนแค่ครั้งเดียว
**คำใบ้:** `proxy_cache_lock on;` + `proxy_cache_lock_timeout 5s;`
**ผ่านเมื่อ:** ดู log ของ api แล้วเห็น request เข้าไปแค่ครั้งเดียว (ยิงพร้อมกันด้วย `xargs -P100` หรือ `wrk`)

---

### N4.4 limit ตาม API key

**โจทย์:** เปลี่ยน rate limit ให้นับตาม header `X-API-Key` แทน IP
**คำใบ้:** `limit_req_zone $http_x_api_key zone=key_limit:10m rate=5r/s;`
**ผ่านเมื่อ:** key ต่างกันยิงได้อิสระต่อกัน และตอบได้ว่าคนที่ไม่ส่ง key จะเกิดอะไร (ทุกคนแชร์ bucket ของค่าว่างเดียวกัน) พร้อมเสนอวิธีแก้

---

### N4.5 metrics

**โจทย์:** เปิด `stub_status` แล้วดึงค่าจากภายในเท่านั้น
**คำใบ้:**

```nginx
location = /nginx_status {
    stub_status;
    allow 127.0.0.1;
    allow 172.16.0.0/12;
    deny all;
}
```

**ผ่านเมื่อ:** ดึงจากใน container ได้ แต่จากภายนอกได้ 403 — แล้วต่อยอดด้วย nginx-prometheus-exporter

---

### N4.6 หาคอขวดด้วยการยิงโหลด

**โจทย์:** ใช้ `wrk` หรือ `k6` ยิงจนเจอเพดาน แล้วจูนให้สูงขึ้น

```bash
docker run --rm --network host williamyeh/wrk -t4 -c100 -d30s http://localhost:8080/api/todos
```

**คำใบ้ที่ควรลองไล่:** `worker_connections`, `keepalive` ใน upstream, `worker_processes auto`, `worker_rlimit_nofile`
**ผ่านเมื่อ:** มีตัวเลข rps ก่อน-หลัง และ**ระบุได้ว่าคอขวดย้ายไปอยู่ที่ไหน** (nginx → node → postgres)

---

### N4.7 ทำ canary ด้วย nginx

**โจทย์:** ส่ง traffic 10% ไป api เวอร์ชันใหม่ อีก 90% ไปเวอร์ชันเก่า
**คำใบ้:** `split_clients "${remote_addr}${request_id}" $backend { 10% api_new; * api_old; }`
**ผ่านเมื่อ:** ยิง 100 request แล้วนับได้ประมาณ 10/90 และอธิบายข้อจำกัดของวิธีนี้เทียบกับ service mesh

---

### N4.8 rate limit หลายชั้นซ้อนกัน

**โจทย์:** ตั้ง 3 ชั้นพร้อมกัน — ต่อ IP, ต่อ API key, และเพดานรวมทั้งระบบ
**ผ่านเมื่อ:** ทดสอบได้ว่าแต่ละชั้นทำงานแยกกันจริง และอธิบายลำดับการตรวจของ nginx ได้

---

## ✅ เช็กลิสต์จบระดับ 4

- [ ] cache ทำงานและตรวจสอบสถานะได้ผ่าน header
- [ ] backend ล่มแล้วผู้ใช้ยังได้ข้อมูล (stale)
- [ ] กัน stampede ได้จริง พิสูจน์จาก log ของ backend
- [ ] มีตัวเลข rps ก่อน-หลังการจูน และรู้ว่าคอขวดอยู่ที่ไหน

➡️ [เฉลยระดับ 4](../solutions/nginx/04-advanced.md) · [ไประดับ 5](05-expert.md)
