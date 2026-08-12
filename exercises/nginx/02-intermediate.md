# 🌐 Nginx — ระดับ 2: พื้นฐานแน่น

> **เป้าหมาย:** พิสูจน์พฤติกรรมของ load balancing และ rate limit ด้วยการทดลอง ไม่ใช่เชื่อตามเอกสาร
> **ต้องมีก่อน:** ผ่านระดับ 1
> **เวลาโดยประมาณ:** 2–4 ชั่วโมง

---

### N2.1 พิสูจน์ว่า load balance ทำงานจริง

**โจทย์:** scale api เป็น 4 แล้วนับว่า request กระจายไปตัวไหนบ้าง

```bash
docker compose up -d --scale api=4
for i in $(seq 1 20); do curl -s localhost:8080/api/todos > /dev/null; done
docker compose logs nginx | grep -o 'upstream=[0-9.:]*' | sort | uniq -c
```

**ผ่านเมื่อ:** เห็นปลายทาง 4 ตัว จำนวนใกล้เคียงกัน

---

### N2.2 round-robin vs least_conn

**โจทย์:** ทำให้บาง request ช้า (เพิ่ม endpoint ที่หน่วง 3 วิ) แล้วยิงผสมกับ request เร็ว เทียบสองอัลกอริทึม
**คำใบ้:** เพิ่มใน `src/app.ts`: `app.get('/api/slow', async (_,res)=>{ await new Promise(r=>setTimeout(r,3000)); res.json({ok:true}) })`
**ผ่านเมื่อ:** อธิบายได้ว่าเคสไหน `least_conn` ช่วย และเคสไหนไม่ต่างจาก round-robin เลย

---

### N2.3 rate limit 3 แบบ

**โจทย์:** ทดลอง 3 ค่าแล้วทำตาราง — ไม่ใส่ burst / `burst=20` / `burst=20 nodelay`

```bash
time (for i in $(seq 1 30); do curl -s -o /dev/null -w "%{http_code} " localhost:8080/api/todos; done)
```

**ผ่านเมื่อ:** ตารางมี 3 แถว บันทึกทั้งจำนวน 200/429 และเวลารวม แล้วอธิบายได้ว่าทำไม `nodelay` ถึงเวลารวมสั้นกว่าทั้งที่จำนวน 429 ใกล้เคียงกัน

---

### N2.4 แยก limit ตาม method

**โจทย์:** ให้ GET 20r/s แต่ POST แค่ 2r/s
**คำใบ้:** `map $request_method $limit_key { POST $binary_remote_addr; default ""; }` — key ที่เป็นค่าว่างจะไม่ถูกนับ
**ผ่านเมื่อ:** ยิง GET รัว ๆ ไม่โดนตัด แต่ POST โดนตัดที่ 2r/s

---

### N2.5 X-Forwarded-For มีผลจริงไหม

**โจทย์:** ลบบรรทัด `proxy_set_header X-Forwarded-For` ออกแล้วดู log ของแอป
**ผ่านเมื่อ:** เห็น `ip` ในแอปเปลี่ยนเป็น IP ภายในของ docker แล้วใส่กลับ พร้อมอธิบายว่าถ้าทำ rate limit ในแอปโดยไม่มี header นี้จะเกิดอะไร

---

### N2.6 client_max_body_size

**โจทย์:** ส่ง body 2MB แล้วดูว่าใครปฏิเสธ

```bash
python3 -c "print('{\"title\":\"' + 'x'*2000000 + '\"}')" > /tmp/big.json
curl -i -X POST localhost:8080/api/todos -H 'Content-Type: application/json' --data-binary @/tmp/big.json
```

**ผ่านเมื่อ:** ได้ 413 จาก nginx (ดูจาก header `Server`) และอธิบายได้ว่าทำไมตัดที่ nginx ดีกว่าปล่อยให้ Express อ่านจนครบ

---

### N2.7 timeout ทำงานยังไง

**โจทย์:** ตั้ง `proxy_read_timeout 5s` แล้วยิงไปที่ endpoint ที่หน่วง 10 วิ
**ผ่านเมื่อ:** ได้ 504 ที่วินาทีที่ 5 พอดี และตรวจใน log ของแอปว่า **แอปยังทำงานต่อจนครบ 10 วิ** ทั้งที่ไม่มีใครรออยู่แล้ว (นี่คือปัญหาที่ [docs/09](../../docs/09-where-to-configure.md) พูดถึง)

---

### N2.8 ลอง failover

**โจทย์:** scale เป็น 3 แล้ว kill ทีละตัวขณะยิง request ต่อเนื่อง
**คำใบ้:** `docker compose exec -T api kill 1` หรือ `docker kill <container>`
**ผ่านเมื่อ:** นับได้ว่ามี request พลาดกี่ตัว แล้วอธิบายว่า `proxy_next_upstream` ช่วยลดจำนวนนั้นได้ยังไง

---

## ✅ เช็กลิสต์จบระดับ 2

- [ ] พิสูจน์การกระจาย traffic ด้วยข้อมูลจาก log ได้
- [ ] อธิบาย burst/nodelay ได้จากผลการทดลองของตัวเอง
- [ ] เข้าใจว่าทำไม header ที่ส่งต่อถึงสำคัญ
- [ ] รู้ว่า timeout ที่ nginx ไม่ได้หยุดงานที่แอป

➡️ [เฉลยระดับ 2](../solutions/nginx/02-intermediate.md) · [ไประดับ 3](03-production.md)
