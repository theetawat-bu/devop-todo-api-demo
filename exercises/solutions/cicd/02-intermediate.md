# เฉลย — ⚙️ CI/CD ระดับ 2

⬅️ [กลับไปที่โจทย์](../../cicd/02-intermediate.md)

---

## C2.1 เทสจริงด้วย net/http/httptest

```bash
go test ./... -v
```

```go
// internal/todos/handler_test.go
package todos_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/<you>/devops-todo-api/internal/app"
)

func setupRouter(t *testing.T) http.Handler {
	t.Helper()
	// ต่อ DB ทดสอบจริง (schema เดียวกับที่ migrate up ไว้ใน CI)
	// แล้ว TRUNCATE todos ก่อนแต่ละเทสเพื่อความสะอาด
	return app.New(testDB(t))
}

func TestCreateTodo(t *testing.T) {
	r := setupRouter(t)
	body, _ := json.Marshal(map[string]string{"title": "ทดสอบ"})
	req := httptest.NewRequest(http.MethodPost, "/api/todos", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestCreateTodoRejectsEmptyBody(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/todos", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetTodoNotFound(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/todos/999999", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetTodoInvalidID(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/todos/abc", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// + เทส update (PATCH คืน done:true) และ delete (DELETE คืน 204) ในแพตเทิร์นเดียวกัน
```

**ทำไม `app.New(pool)` ถึงถูกแยกออกจาก `.Run()` ตั้งแต่แรก:**

```go
func New(pool *pgxpool.Pool) *gin.Engine { ... }   // internal/app/app.go — คืน *gin.Engine
r := app.New(pool)
r.Run(":3000")                                       // cmd/api/main.go — เปิด port
```

`httptest.NewRecorder()` + `r.ServeHTTP(w, req)` เรียก handler ได้โดยตรง → **ไม่ต้องเปิด port จริง** ทำให้เทสรันขนานกันได้และเร็วกว่ามาก
ถ้าเขียนการเปิด port ไว้ในฟังก์ชันเดียวกับการสร้าง router จะเทสยากมาก — นี่คือตัวอย่างของการออกแบบเพื่อการทดสอบ เหมือนกับที่ Express แยก `createApp()` จาก `listen()`

ใน CI แทน smoke test เดิม:

```yaml
- run: go test ./... -coverprofile=coverage.out -v
```

---

## C2.2 lint แบบขนาน

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.25" }
      - name: gofmt check
        run: |
          fmt_out=$(gofmt -l .)
          if [ -n "$fmt_out" ]; then
            echo "::error::ไฟล์ต่อไปนี้ยังไม่ผ่าน gofmt: $fmt_out"
            exit 1
          fi
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          args: --timeout=5m

  quality:
    runs-on: ubuntu-latest
    # ไม่ใส่ needs: → รันขนานกับ lint
```

**ทำไมไม่ใส่ `needs`:** job ที่ไม่มี `needs` จะเริ่มพร้อมกันทั้งหมด
เวลารวมของ workflow = เวลาของ job ที่ช้าที่สุด ไม่ใช่ผลรวม

**`gofmt -l .`** คือฝั่ง Go ของ `prettier --check` — list ไฟล์ที่ format ไม่ตรงมาตรฐานโดยไม่แก้ให้ (ต้องรัน `gofmt -w` เองถ้าจะแก้)
**`golangci-lint`** คือฝั่ง Go ของ ESLint — รวม linter หลายตัว (`govet`, `staticcheck`, `errcheck`, `unused` ฯลฯ) ไว้ในคำสั่งเดียว ตั้ง exit code ไม่เป็น 0 เมื่อเจอปัญหาอยู่แล้วโดยไม่ต้องมี flag แยกแบบ `--max-warnings=0`

---

## C2.3 matrix

```yaml
strategy:
  fail-fast: false
  matrix:
    go: ["1.24", "1.25"]
steps:
  - uses: actions/setup-go@v5
    with: { go-version: ${{ matrix.go }} }
```

**`fail-fast: false`** = ถ้า Go 1.24 พัง ให้ Go 1.25 รันต่อจนจบ
ค่าเริ่มต้นคือ `true` ซึ่งจะยกเลิกทั้งหมดทันที — ทำให้ไม่รู้ว่าพังเฉพาะเวอร์ชันเดียวหรือพังทั้งคู่ (ข้อมูลนี้สำคัญมากในการวินิจฉัย)

**จำนวน job จะคูณกัน:**

```yaml
matrix:
  go: ["1.24", "1.25"] # 2
  os: [ubuntu-latest, macos-latest] # × 2
# = 4 job
```

ระวังเรื่องโควตา — macOS runner คิดค่าใช้จ่าย **10 เท่า** ของ Linux

---

## C2.4 branch protection

Settings → Branches → Add rule สำหรับ `main`:

- ✅ Require a pull request before merging
- ✅ Require status checks to pass → เลือก `quality` และ `lint`
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
| module cache ของ `actions/setup-go@v5` (เปิดอยู่แล้วโดยดีฟอลต์) | 20-40 วินาที |
| docker layer cache (`type=gha`) | 1-3 นาที |
| ลบ step ที่ซ้ำระหว่าง job | ตามที่ซ้ำ |
| `paths` filter | ประหยัดทั้ง run |
| `concurrency` ยกเลิก run เก่า | ประหยัดโควตา |

**สำคัญกว่าทุกวิธี: ต้องรู้ก่อนว่าเวลาหมดไปกับ step ไหน**
GitHub แสดงเวลาของแต่ละ step อยู่แล้ว — กดดูก่อนแล้วค่อยจูนตรงที่ช้าที่สุด

**สิ่งที่มักถูกมองข้าม:** `go build`/`go mod download` ใน 3 job = ดาวน์โหลด module 3 ครั้ง
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
- run: go test ./... -coverprofile=coverage.out
- run: go tool cover -html=coverage.out -o coverage.html

- name: เขียน coverage ลงสรุป
  if: always()
  run: |
    echo "### Coverage" >> $GITHUB_STEP_SUMMARY
    echo '```' >> $GITHUB_STEP_SUMMARY
    go tool cover -func=coverage.out | tail -10 >> $GITHUB_STEP_SUMMARY
    echo '```' >> $GITHUB_STEP_SUMMARY

- uses: actions/upload-artifact@v4
  if: always()
  with:
    name: coverage-html
    path: coverage.html
```

**`if: always()`** จำเป็น — ถ้าไม่ใส่ พอเทสพัง step นี้จะถูกข้าม
ซึ่งเป็นตอนที่**ต้องการดู report มากที่สุด**

**ข้อควรระวังเรื่อง coverage:** อย่าตั้งเป้าเป็นตัวเลขแล้วบังคับ (เช่น "ต้อง 80%")
เพราะจะได้เทสที่เขียนเพื่อเพิ่มเปอร์เซ็นต์ ไม่ใช่เทสที่จับบั๊กได้
ให้ดู coverage เป็น**ข้อมูลว่าตรงไหนยังไม่ได้ทดสอบ** ไม่ใช่เป้าหมายในตัวเอง

---

## 🎯 ต่อยอด

- ลอง `go test ./... -run TestCreateTodo` ให้รันเฉพาะเทสที่ระบุชื่อ
- เพิ่ม `-p 1` ถ้าเทสแย่ง DB กัน หรือดีกว่านั้นคือให้แต่ละ package ใช้ schema แยกกัน
- ลองใช้ [testcontainers-go](https://golang.testcontainers.org/) เพื่อให้เทสปั้น Postgres เองโดยไม่ต้องพึ่ง service container
