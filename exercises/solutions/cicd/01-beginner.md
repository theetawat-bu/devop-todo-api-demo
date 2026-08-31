# เฉลย — ⚙️ CI/CD ระดับ 1

⬅️ [กลับไปที่โจทย์](../../cicd/01-beginner.md)

---

## C1.1 CI รันครั้งแรก

```bash
git remote add origin https://github.com/<you>/devops-todo-api.git
git push -u origin main
```

ไปที่แท็บ **Actions** → เห็น workflow "CI" กำลังรัน

ถ้าไม่รันเลย ให้ตรวจ 3 อย่างตามลำดับ:

1. ไฟล์อยู่ที่ `.github/workflows/*.yml` เป๊ะไหม (ไม่ใช่ `.github/workflow/`)
2. branch ใน `on:` ตรงกับ branch จริงไหม (`main` vs `master`)
3. YAML ถูกต้องไหม — GitHub จะขึ้น error ให้เห็นที่แท็บ Actions

---

## C1.2 อ่าน workflow

```
ci.yml
├── job: quality                 ← มี services: postgres
├── job: validate-k8s            ← ตรวจ kustomize/kubeconform
└── job: docker                  ← needs: quality
```

| คำถาม                       | คำตอบ                                                                                 |
| --------------------------- | ------------------------------------------------------------------------------------- |
| กี่ job                     | 3                                                                                     |
| ใครรอใคร                    | `docker` รอ `quality` (ผ่าน `needs:`); `validate-k8s` รันอิสระ ไม่รอใคร               |
| `services:` คืออะไร         | container ที่ GitHub ปั้นให้ระหว่าง job รัน แล้วดับให้เอง — ในที่นี้คือ Postgres จริง |
| ทำไมต้อง `actions/checkout` | **runner เริ่มต้นด้วยเครื่องเปล่า ไม่มีโค้ดเราเลย** ต้องดึงลงมาก่อนเสมอ               |

`actions/checkout` เป็นข้อผิดพลาดอันดับหนึ่งของคนเริ่มต้น — ลืมใส่แล้วงงว่าทำไม `go build` บอกว่าไม่เจอ `go.mod`

---

## C1.3 ทำให้ CI แดง

```go
// cmd/api/main.go
var x int = "ไม่ใช่ตัวเลข"
```

```
Run go build -o api ./cmd/api
./cmd/api/main.go:12:6: cannot use "ไม่ใช่ตัวเลข" (untyped string constant) as int value in variable declaration
Error: Process completed with exit code 1
```

**ทำไม CI ถึงรู้ว่าล้มเหลว:** ทุก step ดูที่ **exit code** ของคำสั่ง — ไม่ใช่ 0 = ล้มเหลว
`go build` คืน exit code 1 เมื่อ compile ไม่ผ่าน (type error ใน Go เป็น compile error เสมอ ไม่ใช่ step แยกต่างหากแบบ `tsc` ของ TypeScript) → GitHub จับได้เอง

**สิ่งที่ต้องระวัง:** คำสั่งที่พังแต่คืน exit code 0 จะทำให้ CI เขียวหลอก
เช่น `curl` ที่ไม่ใส่ `-f` จะคืน 0 แม้ได้ 500 — นี่คือเหตุผลที่ smoke test ของเราใช้ `curl -fsS`

---

## C1.4 secret vs variable

```yaml
- run: |
    echo "variable: ${{ vars.TEST_VAR }}"
    echo "secret:   ${{ secrets.TEST_SECRET }}"

# variable: hello-world
# secret:   ***
```

|                     | Secret                      | Variable                            |
| ------------------- | --------------------------- | ----------------------------------- |
| แสดงใน log          | `***`                       | ค่าจริง                             |
| อ่านกลับจากหน้าเว็บ | ❌ ไม่ได้เลย                | ✅                                  |
| ใช้กับ              | token, รหัสผ่าน, kubeconfig | URL, ชื่อ environment, feature flag |

**ทำไมควรแยกให้ถูก:** ถ้าเอา URL ไปใส่ใน secret ทั้งหมด เวลาอ่าน log จะเห็นแต่ `***` ทำให้ debug ยากมาก
ใส่ secret เฉพาะสิ่งที่เป็นความลับจริง ๆ

⚠️ **การปิดบังไม่สมบูรณ์แบบ:** ถ้าเอา secret ไปแปลง (เช่น base64 หรือ substring) GitHub จะจำไม่ได้แล้วปล่อยให้แสดงผลออกมา

---

## C1.5 path filter

```yaml
on:
  push:
    branches: [main]
    paths:
      - "cmd/**"
      - "internal/**"
      - "go.mod"
      - "go.sum"
      - "migrations/**"
      - "Dockerfile"
      - ".github/workflows/**"
```

ทดสอบ: แก้ `README.md` แล้ว push → workflow ไม่รัน

**ทำไมมีประโยชน์:** ประหยัดเวลารอและโควตา CI จากการแก้เอกสาร

⚠️ **กับดักสำคัญ:** ถ้าตั้ง required status check ไว้ใน branch protection แล้ว workflow ไม่รันเพราะ path filter
PR นั้นจะ **merge ไม่ได้ตลอดกาล** เพราะรอ check ที่ไม่มีวันมา

ทางแก้: ใช้ job ปลอมที่รันเสมอแล้วรายงานผลสำเร็จ หรือใช้ `paths-ignore` แทนแล้วให้ workflow รันแต่ข้าม step ที่หนัก

---

## C1.6 workflow_dispatch

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      environment:
        description: "จะ deploy ไปที่ไหน"
        type: choice
        options: [dev, staging]
        default: dev
```

ใช้ค่าที่กรอกด้วย `${{ inputs.environment }}`

**ทำไมควรมีติดไว้เสมอ:** เวลาต้องรัน workflow ใหม่โดยไม่มีอะไรจะ commit (เช่น deploy ซ้ำ, ทดสอบหลังแก้ secret)
ถ้าไม่มี จะต้องสร้าง commit เปล่า ๆ ซึ่งทำให้ history สกปรก

---

## C1.7 artifact และ summary

```yaml
- name: เขียนสรุป
  run: |
    echo "### ผลการ build" >> $GITHUB_STEP_SUMMARY
    echo "- commit: \`${{ github.sha }}\`" >> $GITHUB_STEP_SUMMARY
    echo "- เวลา: $(date)" >> $GITHUB_STEP_SUMMARY

- uses: actions/upload-artifact@v4
  with:
    name: build-output
    path: api
    retention-days: 7
```

|               | Summary                                                | Artifact                              |
| ------------- | ------------------------------------------------------ | ------------------------------------- |
| คืออะไร       | markdown ที่แสดงบนหน้า run                             | ไฟล์ให้ดาวน์โหลด                      |
| เหมาะกับ      | ผลลัพธ์ที่อยากให้เห็นทันที (coverage, ขนาด image, URL) | test report, build output, screenshot |
| อยู่นานแค่ไหน | ตราบที่ run ยังอยู่                                    | ตาม `retention-days` (สูงสุด 90 วัน)  |

**เคล็ดลับ:** ใส่ URL ของ environment ที่ deploy ไปใน summary ทำให้คนที่เปิดดู run กดเข้าไปทดสอบได้ทันที
เป็นการปรับปรุงเล็ก ๆ ที่ทีมชอบมาก

---

## 🎯 ต่อยอด

- ลอง `if: always()` กับ step ที่อยากให้รันแม้ step ก่อนหน้าจะพัง (เช่น อัปโหลด log)
- ดู context ทั้งหมดที่ใช้ได้: `- run: echo '${{ toJSON(github) }}'`
- ลองใช้ [act](https://github.com/nektos/act) รัน workflow ในเครื่องตัวเอง
