# 04 — Docker Compose

Compose = อธิบายว่าระบบเรามี container อะไรบ้าง ต่อกันยังไง ในไฟล์เดียว แล้วสั่งขึ้นทั้งชุดด้วยคำสั่งเดียว

## เริ่มใช้งาน

```bash
cp .env.example .env
docker compose up -d --build     # -d = รันเบื้องหลัง, --build = build ใหม่
docker compose ps
docker compose logs -f api
docker compose down              # ดับ (ข้อมูลใน volume ยังอยู่)
docker compose down -v           # ดับ + ลบ volume (ข้อมูล DB หายหมด)
```

เข้าใช้งานที่ **http://localhost:8080** (ผ่าน nginx) — ไม่ใช่ 3000 เพราะ api ไม่ได้ publish port ออกมา

## ชำแหละ `docker-compose.yml`

### `depends_on` + `healthcheck`

```yaml
db:
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U app -d tododb"]
    interval: 5s
    retries: 10

api:
  depends_on:
    db:
      condition: service_healthy
```

**จุดที่คนพลาดบ่อยที่สุด:** `depends_on` เฉย ๆ รอแค่ "container ถูกสร้าง" ไม่ได้รอ "แอปข้างในพร้อม"
Postgres ใช้เวลา init หลายวินาที ถ้าไม่ใส่ `condition: service_healthy` แอปจะสตาร์ทแล้วต่อ DB ไม่ติดทันที

ถึงจะใส่แล้วก็ยังควรทำให้แอป retry เองได้ (production จริง DB restart กลางทางได้เสมอ)

### Network

```yaml
networks:
  appnet:
    driver: bridge
```

ทุก service อยู่ network เดียวกัน → คุยกันด้วย **ชื่อ service** ได้เลย (`db`, `api`, `nginx`)
Docker มี DNS ในตัว นี่คือเหตุผลที่ `DATABASE_URL` ใช้ `@db:5432`

### Volume

```yaml
volumes:
  pgdata:
```

container เป็นของ **ชั่วคราว** — ลบทิ้งเมื่อไรข้อมูลข้างในหายหมด
named volume ทำให้ข้อมูล Postgres อยู่นอก container → `docker compose down` แล้ว `up` ใหม่ ข้อมูลยังอยู่

**บทเรียนสำคัญ: อะไรที่มี state ต้องอยู่บน volume เสมอ**

### `ports` vs `expose`

```yaml
api:
  expose: ["3000"] # เห็นเฉพาะใน network ภายใน
nginx:
  ports: ["8080:80"] # เปิดออกสู่เครื่องเรา (host:container)
```

จงใจไม่ publish port ของ api เพื่อบังคับให้ traffic ผ่าน nginx เท่านั้น — เป็นแพตเทิร์นเดียวกับ production
(ถ้าอยากยิงตรงเพื่อ debug ให้ใช้ `docker compose exec api sh` หรือใช้ไฟล์ dev override)

### Environment & ค่า default

```yaml
POSTGRES_USER: ${POSTGRES_USER:-app}
```

`${VAR:-default}` = อ่านจาก `.env` ถ้าไม่มีใช้ค่า default → ไม่มี `.env` ก็ยังรันได้

## โหมด dev (hot reload)

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up
```

Compose จะ **merge** ไฟล์หลังทับไฟล์แรก ไฟล์ dev เปลี่ยน 3 อย่าง:

1. `target: builder` — ใช้ stage ที่มี Go toolchain เต็ม (`go run` ใช้ตัวคอมไพเลอร์ตรง ๆ)
2. mount `./cmd`, `./internal`, `./migrations` เข้า container + ใช้ `go run ./cmd/api` — แก้โค้ดแล้ว restart container ให้คอมไพล์ใหม่ (Go ไม่มี hot-reload แบบ `tsx watch` ในตัว เพราะเป็นภาษา compiled — `go run` คอมไพล์ทุกครั้งที่รัน ซึ่งเร็วพอสำหรับโปรเจกต์เล็กขนาดนี้)
3. เปิด port 3000 ตรง ๆ ไว้ debug

ถ้าอยากได้ hot-reload แบบไม่ต้อง restart container เอง มี tool อย่าง [`air`](https://github.com/air-verse/air) ที่ watch ไฟล์แล้วสั่ง build+run ให้อัตโนมัติ — โปรเจกต์นี้ตั้งใจไม่ใช้เพื่อลด dependency แต่รู้ไว้ว่ามีตัวเลือกนี้อยู่

pattern "ไฟล์ base + ไฟล์ override" นี้ใช้กันมาก เช่น `docker-compose.prod.yml` สำหรับ production

## ลอง scale

```bash
docker compose up -d --scale api=4
docker compose ps
# ยิงซ้ำ ๆ แล้วดูว่า nginx กระจายไป container ไหนบ้าง
for i in $(seq 1 10); do curl -s localhost:8080/healthz > /dev/null; done
docker compose logs nginx | tail -20    # ดูคอลัมน์ upstream=
```

## คำสั่งดีบักที่ใช้บ่อย

```bash
docker compose logs -f --tail=100 api
docker compose exec db psql -U app -d tododb -c '\dt'   # ดูตารางใน DB
docker compose exec api sh                              # เข้าไปใน container api
docker compose config                                   # ดู yaml ที่ merge/แทนค่าแล้ว
docker compose restart api
docker compose up -d --build api                        # build+restart เฉพาะ service เดียว
```

`docker compose config` มีประโยชน์มากเวลาไม่แน่ใจว่าตัวแปรถูกแทนค่าอะไรไป

## 🪛 Playground

ลองเล่นก่อนไปบทถัดไป:

- [ ] `docker compose stop db` แล้วดู `docker compose logs api` — แอปพัง หรือ retry
- [ ] ลบ `condition: service_healthy` ออกชั่วคราวแล้ว `up` ใหม่ ดูว่าเกิดอะไรขึ้น
- [ ] scale เป็น 4 แล้วยิง `for i in $(seq 1 20); do curl -s localhost:8080/api/todos > /dev/null; done` ดู `docker compose logs nginx`
- [ ] `docker compose down -v` แล้ว `up` ใหม่ — ข้อมูลหายจริงไหม เทียบกับตอนใช้ `down` เฉย ๆ
- [ ] แก้ `PORT` ใน `.env` เป็นค่าอื่น แล้วดูว่าตรงไหนพังบ้าง (เพราะ hardcode ไว้หลายที่)

➡️ ต่อไป: [05 — Nginx](05-nginx.md)
