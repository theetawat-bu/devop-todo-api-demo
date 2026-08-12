# เฉลย — ⚙️ CI/CD ระดับ 2

⬅️ [กลับไปที่โจทย์](../../cicd/02-intermediate.md)

---

## C2.1 เทสจริงด้วย Jest + Supertest

```bash
npm i -D jest ts-jest @types/jest supertest @types/supertest
npx ts-jest config:init
```

```js
// jest.config.js
module.exports = {
  preset: "ts-jest",
  testEnvironment: "node",
  testMatch: ["**/__tests__/**/*.test.ts"],
};
```

```ts
// src/__tests__/todos.test.ts
import request from "supertest";
import { createApp } from "../app";
import { prisma } from "../prisma";

const app = createApp();

beforeEach(async () => {
  await prisma.todo.deleteMany();
});
afterAll(async () => {
  await prisma.$disconnect();
});

describe("Todos API", () => {
  it("สร้าง todo ได้", async () => {
    const res = await request(app).post("/api/todos").send({ title: "ทดสอบ" });
    expect(res.status).toBe(201);
    expect(res.body).toMatchObject({ title: "ทดสอบ", done: false });
  });

  it("ปฏิเสธ body ที่ไม่ถูกต้อง", async () => {
    const res = await request(app).post("/api/todos").send({});
    expect(res.status).toBe(400);
  });

  it("คืน 404 เมื่อไม่เจอ", async () => {
    expect((await request(app).get("/api/todos/999999")).status).toBe(404);
  });

  it("คืน 400 เมื่อ id ไม่ใช่ตัวเลข", async () => {
    expect((await request(app).get("/api/todos/abc")).status).toBe(400);
  });

  it("แก้ไขได้", async () => {
    const c = await request(app).post("/api/todos").send({ title: "a" });
    const res = await request(app).patch(`/api/todos/${c.body.id}`).send({ done: true });
    expect(res.body.done).toBe(true);
  });

  it("ลบแล้วคืน 204", async () => {
    const c = await request(app).post("/api/todos").send({ title: "b" });
    expect((await request(app).delete(`/api/todos/${c.body.id}`)).status).toBe(204);
  });
});
```

**ทำไม `createApp()` ถึงถูกแยกออกจาก `listen()` ตั้งแต่แรก:**

```ts
export function createApp() { ... }        // src/app.ts — คืน app object
const app = createApp();
app.listen(port)                            // src/index.ts — เปิด port
```

supertest รับ app object ได้โดยตรง → **ไม่ต้องเปิด port จริง** ทำให้เทสรันขนานกันได้และเร็วกว่ามาก
ถ้าเขียน `app.listen()` ไว้ในไฟล์เดียวกับ app จะเทสยากมาก — นี่คือตัวอย่างของการออกแบบเพื่อการทดสอบ

ใน CI แทน smoke test เดิม:

```yaml
- run: npx jest --coverage
```

---

## C2.2 lint แบบขนาน

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: "22", cache: "npm" }
      - run: npm ci
      - run: npx eslint src --max-warnings=0
      - run: npx prettier --check "src/**/*.ts"

  build-and-test:
    runs-on: ubuntu-latest
    # ไม่ใส่ needs: → รันขนานกับ lint
```

**ทำไมไม่ใส่ `needs`:** job ที่ไม่มี `needs` จะเริ่มพร้อมกันทั้งหมด
เวลารวมของ workflow = เวลาของ job ที่ช้าที่สุด ไม่ใช่ผลรวม

**`--max-warnings=0`** สำคัญ — ไม่งั้น warning จะสะสมไปเรื่อย ๆ จนไม่มีใครอ่าน

---

## C2.3 matrix

```yaml
strategy:
  fail-fast: false
  matrix:
    node: [20, 22]
steps:
  - uses: actions/setup-node@v4
    with: { node-version: ${{ matrix.node }} }
```

**`fail-fast: false`** = ถ้า Node 20 พัง ให้ Node 22 รันต่อจนจบ
ค่าเริ่มต้นคือ `true` ซึ่งจะยกเลิกทั้งหมดทันที — ทำให้ไม่รู้ว่าพังเฉพาะเวอร์ชันเดียวหรือพังทั้งคู่ (ข้อมูลนี้สำคัญมากในการวินิจฉัย)

**จำนวน job จะคูณกัน:**

```yaml
matrix:
  node: [20, 22] # 2
  os: [ubuntu-latest, macos-latest] # × 2
# = 4 job
```

ระวังเรื่องโควตา — macOS runner คิดค่าใช้จ่าย **10 เท่า** ของ Linux

---

## C2.4 branch protection

Settings → Branches → Add rule สำหรับ `main`:

- ✅ Require a pull request before merging
- ✅ Require status checks to pass → เลือก `build-and-test` และ `lint`
- ✅ Require branches to be up to date before merging
- ✅ Do not allow bypassing the above settings ← **สำคัญ** ไม่งั้น admin ข้ามได้

**ทำไม "up to date" ถึงสำคัญ:** PR ที่ CI ผ่านตอนแยก branch ไป อาจพังเมื่อรวมกับ commit ใหม่ใน main
(เรียกว่า semantic conflict — git merge ผ่านแต่โค้ดพัง) การบังคับ rebase ก่อน merge กันปัญหานี้

**ข้อเสีย:** ถ้าทีมใหญ่ PR จะต้อง rebase บ่อยมาก → พิจารณา merge queue แทน

---

## C2.5 ลดเวลา CI

ลำดับที่มักได้ผลมากสุด:

| วิธี | ผลที่มักได้ |
| --- | --- |
| `cache: 'npm'` ใน setup-node | 30-60 วินาที |
| docker layer cache (`type=gha`) | 1-3 นาที |
| ลบ step ที่ซ้ำระหว่าง job | ตามที่ซ้ำ |
| `paths` filter | ประหยัดทั้ง run |
| `concurrency` ยกเลิก run เก่า | ประหยัดโควตา |

**สำคัญกว่าทุกวิธี: ต้องรู้ก่อนว่าเวลาหมดไปกับ step ไหน**
GitHub แสดงเวลาของแต่ละ step อยู่แล้ว — กดดูก่อนแล้วค่อยจูนตรงที่ช้าที่สุด

**สิ่งที่มักถูกมองข้าม:** `npm ci` ใน 3 job = ดาวน์โหลด 3 ครั้ง
ถ้าแคชแล้วยังช้า พิจารณา build ครั้งเดียวแล้วส่งผ่าน artifact ให้ job อื่น

---

## C2.6 concurrency

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

push 3 ครั้งติด → run ที่ 1 และ 2 ถูกยกเลิก เหลือแค่ run ที่ 3

**ทำไม job ที่ deploy ห้ามตั้ง `cancel-in-progress: true`:**

```
14:00  deploy v1 เริ่ม → กำลัง rolling update อยู่ครึ่งทาง
14:01  push ใหม่ → deploy v1 ถูกยกเลิกกลางคัน
       → คลัสเตอร์ค้างในสถานะครึ่ง ๆ กลาง ๆ: pod บางตัวเป็น v1 บางตัวเป็นของเก่า
```

**การยกเลิกงานที่กำลังเปลี่ยนสถานะของระบบ = ทิ้งระบบไว้ในสภาพที่ไม่รู้ว่าอยู่ตรงไหน**

กฎ: `cancel-in-progress: true` สำหรับงานที่ **อ่านอย่างเดียว** (test, lint, build)
`false` สำหรับงานที่ **เปลี่ยนแปลงระบบ** (deploy, migration)

---

## C2.7 coverage report

```yaml
- run: npx jest --coverage --coverageReporters=text-summary --coverageReporters=html

- name: เขียน coverage ลงสรุป
  if: always()
  run: |
    echo "### Coverage" >> $GITHUB_STEP_SUMMARY
    echo '```' >> $GITHUB_STEP_SUMMARY
    npx jest --coverage --coverageReporters=text-summary 2>&1 | tail -10 >> $GITHUB_STEP_SUMMARY
    echo '```' >> $GITHUB_STEP_SUMMARY

- uses: actions/upload-artifact@v4
  if: always()
  with:
    name: coverage-html
    path: coverage/
```

**`if: always()`** จำเป็น — ถ้าไม่ใส่ พอเทสพัง step นี้จะถูกข้าม
ซึ่งเป็นตอนที่**ต้องการดู report มากที่สุด**

**ข้อควรระวังเรื่อง coverage:** อย่าตั้งเป้าเป็นตัวเลขแล้วบังคับ (เช่น "ต้อง 80%")
เพราะจะได้เทสที่เขียนเพื่อเพิ่มเปอร์เซ็นต์ ไม่ใช่เทสที่จับบั๊กได้
ให้ดู coverage เป็น**ข้อมูลว่าตรงไหนยังไม่ได้ทดสอบ** ไม่ใช่เป้าหมายในตัวเอง

---

## 🎯 ต่อยอด

- ลอง `jest --changedSince=main` ให้รันเฉพาะเทสที่เกี่ยวกับไฟล์ที่แก้
- เพิ่ม `--runInBand` ถ้าเทสแย่ง DB กัน หรือดีกว่านั้นคือให้แต่ละ worker ใช้ schema แยกกัน
- ลองใช้ testcontainers เพื่อให้เทสปั้น Postgres เองโดยไม่ต้องพึ่ง service container
