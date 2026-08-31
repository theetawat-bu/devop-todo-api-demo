# เฉลย — ⚙️ CI/CD ระดับ 3

⬅️ [กลับไปที่โจทย์](../../cicd/03-production.md)

---

## C3.1 push ขึ้น GHCR

```yaml
permissions:
  contents: read
  packages: write # ขาดตัวนี้ = denied

- uses: docker/login-action@v3
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}
```

**สาเหตุที่ทำให้ push ไม่ได้ เรียงตามความถี่:**

1. Settings → Actions → General → Workflow permissions ยังเป็น read-only
2. ไม่ได้ใส่ `permissions: packages: write` ใน workflow
3. **ชื่อ image มีตัวพิมพ์ใหญ่** — registry บังคับตัวพิมพ์เล็กทั้งหมด

ถ้าชื่อ GitHub ของคุณมีตัวใหญ่:

```yaml
- run: echo "IMAGE=$(echo ${{ github.repository }} | tr '[:upper:]' '[:lower:]')" >> $GITHUB_ENV
```

**ทำไม `GITHUB_TOKEN` ดีกว่า PAT:** มันถูกสร้างใหม่ทุก run และเพิกถอนเมื่อ run จบ
ถ้าหลุดไปก็ใช้ไม่ได้แล้ว ต่างจาก PAT ที่อยู่ยาวเป็นเดือนเป็นปี

---

## C3.2 tag strategy

```bash
git tag v1.0.0 && git push origin v1.0.0
```

`metadata-action` จะสร้าง: `1.0.0`, `1.0`, `latest`, `sha-a1b2c3d`

| tag | เปลี่ยนทับไหม | ใช้ตอนไหน |
| --- | --- | --- |
| `latest` | ✅ ทับตลอด | ทดลองในเครื่อง — **ห้ามใช้ deploy** |
| `1.0` | ✅ ทับเมื่อออก 1.0.x | ผู้ใช้ที่อยากได้ patch อัตโนมัติ |
| `1.0.0` | ❌ (ถ้าตั้ง immutability) | **deploy ได้** |
| `sha-a1b2c3d` | ❌ | **deploy ได้ — ตามรอยกลับไปหา commit ได้** |
| `@sha256:...` | ❌ เปลี่ยนไม่ได้เลย | **ดีที่สุดสำหรับ deploy** |

**ทำไม `latest` ถึงอันตราย:**

```
จันทร์  deploy latest → ได้ v1.0.0
พุธ     มีคน push latest ใหม่
ศุกร์   pod restart เอง → ดึง latest → ได้ v1.2.0 โดยไม่มีใครสั่ง deploy
```

ระบบเปลี่ยนเวอร์ชันเองโดยไม่มีใครรู้ และ rollback ก็ไม่ได้เพราะไม่รู้ว่าเวอร์ชันเดิมคืออะไร

---

## C3.3 deploy ขึ้น cloud ฟรี

ทำตาม [docs/10](../../../docs/10-deploy-free-cloud.md) — สรุปจุดที่คนพลาดบ่อย:

| ปัญหา | สาเหตุ |
| --- | --- |
| `/readyz` ได้ 503 | ลืม `?sslmode=require` ใน connection string ของ Neon |
| Render ดึง image ไม่ได้ | package บน GHCR ยังเป็น private |
| deploy แล้วเว็บยังเป็นของเก่า | ยังไม่ได้ปิด Auto-Deploy ของ Render |
| `no open ports detected` | env `PORT` ไม่ตรงกับที่แอปฟัง |

**ตรวจว่าสำเร็จจริง:** เปิดจากมือถือโดยปิด wifi — ถ้าใช้ได้แปลว่าออกอินเทอร์เน็ตจริง ไม่ใช่แค่ในเครื่อง

---

## C3.4 พิสูจน์ gate

```bash
git checkout dev
echo 'var x int = "พัง"' >> cmd/api/main.go
git commit -am "test: ทำให้ CI พัง"
git push
```

ผลที่ต้องได้:

```
test    ❌ แดง
build   ⏭️  skipped
deploy  ⏭️  skipped
verify  ⏭️  skipped
```

แล้วเปิดเว็บ → **ยังเป็นเวอร์ชันเก่า**

**ทำไมข้อนี้สำคัญที่สุดในระดับ 3:** pipeline ที่ไม่เคยแดงเลย คือ pipeline ที่ยังไม่รู้ว่าตัวเองทำงานจริงไหม
คุณกำลังฝากความปลอดภัยของ production ไว้กับกลไกที่ไม่เคยทดสอบ

กลไกที่ทำให้ทำงาน:

```yaml
build:
  needs: test # test แดง → job นี้ไม่รันเลย
```

⚠️ ระวัง `if: always()` — ถ้าใส่ผิดที่ job จะรันต่อแม้ test พัง (ทำลาย gate ทั้งหมด)

---

## C3.5 พิสูจน์ verify

```go
// ใส่ชั่วคราวใน cmd/api/main.go ต้นฟังก์ชัน main()
panic("จงใจทำให้พังตอน runtime")
```

`go build` ผ่าน (เพราะ syntax และ type ถูกหมด) แต่แอปตายตอนสตาร์ท

ผลที่ต้องได้:

```
test    ✅
build   ✅
deploy  ✅  (deploy hook คืน 200 — แค่แปลว่า "รับคำสั่งแล้ว")
verify  ❌  healthz ไม่ผ่านหลังรอ 10 นาที
```

**ถ้า `verify` เขียว = verify ของคุณตรวจไม่จริง** สาเหตุที่พบบ่อย:

| ปัญหา | แก้ |
| --- | --- |
| `curl` ไม่มี `-f` | ได้ exit 0 แม้เจอ 500 → ใส่ `-f` |
| ไม่ได้เช็ค `$code` | เก็บค่าแล้ว `test "$code" = "200"` |
| loop จบแล้วไม่ตรวจซ้ำ | ต้องมี `|| exit 1` หลัง loop |
| รอสั้นเกิน | free tier cold start ใช้เวลาถึง 60 วิ |

**บทเรียนใหญ่:** deploy hook ตอบ 200 ≠ deploy สำเร็จ
มันแปลแค่ว่า "ได้รับคำสั่งแล้ว" — ต้องมีคนไปดูปลายทางเสมอ

---

## C3.6 approval gate

```yaml
deploy-prod:
  environment:
    name: production
    url: https://app.example.com
```

Settings → Environments → production → Required reviewers → เลือกคน

workflow จะหยุดรอ พร้อมส่งอีเมล/notification ให้ผู้อนุมัติ

**ประโยชน์ที่มากกว่าการกดอนุมัติ:**

- environment เก็บ secret แยกได้ → secret ของ prod ไม่หลุดไปให้ job ของ dev ใช้
- ตั้ง deployment branch policy ได้ (เช่น deploy prod ได้จาก `main` เท่านั้น)
- มีประวัติว่าใครอนุมัติ deploy ครั้งไหน — ใช้ตอน audit

---

## C3.7 rollback อัตโนมัติ

```yaml
- name: Deploy
  id: deploy
  run: |
    kubectl -n todo-app set image deployment/todo-api api=$IMAGE
    kubectl -n todo-app rollout status deployment/todo-api --timeout=180s

- name: Rollback เมื่อล้มเหลว
  if: failure() && steps.deploy.outcome == 'failure'
  run: |
    kubectl -n todo-app rollout undo deployment/todo-api
    kubectl -n todo-app rollout status deployment/todo-api --timeout=120s
    echo "::error::deploy ล้มเหลว — ย้อนกลับเวอร์ชันก่อนหน้าแล้ว"
```

**`rollout status --timeout` คือกุญแจ** — `set image` คืนค่าทันทีโดยไม่รอผล
ถ้าไม่มีบรรทัดนี้ workflow จะเขียวทั้งที่ pod เข้า `CrashLoopBackOff` อยู่

⚠️ **ข้อควรระวังเมื่อใช้ Argo CD:** `rollout undo` ทำให้คลัสเตอร์ไม่ตรงกับ git
ถ้าเปิด `selfHeal` Argo จะดึงกลับไปเวอร์ชันพังภายในไม่กี่วินาที
→ ในระบบ GitOps ต้อง rollback ด้วย `git revert` แทน

---

## C3.8 สแกนความปลอดภัย

```yaml
- name: govulncheck
  run: |
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...

- name: Trivy
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.IMAGE }}
    severity: HIGH,CRITICAL
    exit-code: "1"
    ignore-unfixed: true # ไม่นับตัวที่ยังไม่มี patch

- name: gitleaks
  uses: gitleaks/gitleaks-action@v2
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

ทดสอบ gitleaks:

```bash
echo 'const key = "AKIAIOSFODNN7EXAMPLE"' >> cmd/api/scratch.go
git commit -am "test" && git push     # gitleaks ต้องจับได้
```

**`ignore-unfixed: true` สำคัญ:** ถ้าไม่ใส่ CI จะแดงเพราะช่องโหว่ที่ upstream ยังไม่มี patch — ซึ่งเราแก้อะไรไม่ได้
**`govulncheck` ต่างจาก Trivy ยังไง:** `govulncheck` วิเคราะห์ **call graph** จริงของโค้ดเรา — เตือนเฉพาะ vulnerability ที่โค้ดเราเรียกใช้ฟังก์ชันที่มีช่องโหว่จริง ๆ ไม่ใช่แค่ "module นี้มี CVE อยู่ในนั้น" เหมือน dependency scanner ทั่วไป ทำให้ noise น้อยกว่ามาก
ผลคือทีมจะชินกับการเห็น CI แดงแล้วเมิน ซึ่งทำลายคุณค่าของการสแกนทั้งหมด

**สิ่งที่ต้องจำ:** secret ที่เคย commit ลง git **ถือว่าหลุดถาวร** แม้จะ force push ลบไปแล้ว
เพราะอาจถูก clone/cache ไปแล้ว → ต้อง**เพิกถอน (revoke) ทันที** ไม่ใช่แค่ลบออกจาก history

---

## C3.9 แจ้งเตือน

```yaml
- name: แจ้งเตือน Discord
  if: always()
  run: |
    STATUS="${{ job.status }}"
    COLOR=$([ "$STATUS" = "success" ] && echo 3066993 || echo 15158332)
    curl -fsS -H "Content-Type: application/json" -d "{
      \"embeds\": [{
        \"title\": \"Deploy $STATUS\",
        \"color\": $COLOR,
        \"fields\": [
          {\"name\":\"repo\",\"value\":\"${{ github.repository }}\"},
          {\"name\":\"commit\",\"value\":\"${{ github.sha }}\"},
          {\"name\":\"ดู run\",\"value\":\"${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}\"}
        ]
      }]
    }" ${{ secrets.DISCORD_WEBHOOK }}
```

**สิ่งที่ต้องมีในข้อความแจ้งเตือน:** ลิงก์กลับไปหน้า run
แจ้งเตือนที่บอกแค่ "deploy failed" โดยไม่มีลิงก์ = คนต้องไปหาเองทุกครั้ง ซึ่งจบลงด้วยการไม่มีใครดู

**อย่าแจ้งเตือนทุกอย่าง** — แจ้งเฉพาะ deploy และเฉพาะเมื่อล้มเหลว (หรือสำเร็จของ prod)
ถ้าแจ้งทุก CI run ที่ผ่าน ช่องจะเต็มไปด้วยเสียงรบกวนจนไม่มีใครอ่านตอนที่เรื่องจริงเกิดขึ้น

---

## 🎯 ต่อยอด

- เพิ่ม deployment status API ให้ GitHub แสดงว่า commit ไหนอยู่บน environment ไหน
- ทำ `workflow_dispatch` ที่รับ input เป็นเวอร์ชันที่จะ rollback ไป
- วัด lead time (commit → production) แล้วดูว่าใช้เวลาส่วนใหญ่ไปกับอะไร
