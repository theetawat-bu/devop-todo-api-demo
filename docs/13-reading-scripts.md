# 13 — อ่านสคริปต์ deploy ให้ออกทีละบรรทัด

> เอกสารนี้ไม่ได้สอนให้ "เขียน" แต่สอนให้ **"อ่านออก"** — ซึ่งเป็นทักษะที่ใช้บ่อยกว่าเยอะ
> เพราะเวลาทำงานจริง เราจะเจอ workflow ที่คนอื่นเขียนไว้แล้วมากกว่าเขียนเองตั้งแต่ต้น
>
> **ต้องอ่านมาก่อน:** [06 ภาคลึก](06-github-actions-ci.md) และ [08 ภาคลึก](08-kubernetes.md)

---

## ส่วนที่ 1 — คำสั่ง shell ที่เจอในทุก pipeline

### 1.1 `curl` — ธงที่ต้องดูก่อนเสมอ

```bash
curl -fsS --max-time 70 "$URL/healthz"
```

| flag | ทำอะไร | **ทำไมสำคัญ** |
| --- | --- | --- |
| `-f` | ได้ 4xx/5xx → **exit code ไม่เป็น 0** | ❗ **ถ้าไม่มีตัวนี้ CI จะเขียวแม้เจอ 500** |
| `-s` | ไม่แสดง progress bar | log สะอาด |
| `-S` | แต่ยังแสดง error | ถ้าใส่ `-s` เฉย ๆ error จะหายไปด้วย |
| `--max-time` | เพดานเวลาทั้งคำขอ | กัน job ค้างจนหมด quota |
| `-o /dev/null` | ทิ้ง body | เมื่อสนใจแค่ status |
| `-w '%{http_code}'` | พิมพ์ status code ออกมา | เอาไปเทียบใน `if` ได้ |

**`-fsS` คือชุดมาตรฐานที่ควรใช้ทุกครั้งในสคริปต์อัตโนมัติ** — จำเป็นชุดไปเลย

**สองแบบนี้ใช้คนละงาน:**

```bash
curl -fsS "$URL/healthz"                                    # ต้องผ่าน ไม่ผ่านให้ step แดงเลย
code=$(curl -s -o /dev/null -w '%{http_code}' "$URL" || echo 000)   # เก็บผลไว้ตัดสินใจเอง
```

แบบที่สองใช้ตอนที่เรา**ต้องการ retry** — ถ้าใช้ `-f` มันจะแดงตั้งแต่ครั้งแรกโดยไม่ทันได้ลองซ้ำ
`|| echo 000` คือกันไว้ตอน curl พังระดับ network (เช่น DNS ยังไม่ขึ้น) ซึ่งจะไม่มีค่า http_code ออกมาเลย

### 1.2 loop ที่รอจนกว่าจะพร้อม

นี่คือแพตเทิร์นที่ใช้ใน `deploy-paas.yml` — อ่านทีละบรรทัด:

```bash
code=000                                     # ① ตั้งค่าเริ่มต้น เผื่อ loop ไม่ทำงานเลย
for i in $(seq 1 40); do                     # ② ลองสูงสุด 40 ครั้ง
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 70 "$URL/healthz" || echo 000)
  echo "  ครั้งที่ $i → $code"                # ③ พิมพ์ทุกครั้ง เพื่อให้ดู log ย้อนหลังได้
  [ "$code" = "200" ] && break               # ④ ได้แล้วออกจาก loop ทันที
  sleep 15                                   # ⑤ รอแล้วลองใหม่
done
[ "$code" = "200" ] || { echo "::error::ไม่ผ่าน"; exit 1; }   # ⑥ ตรวจซ้ำหลัง loop
```

**บรรทัด ⑥ คือบรรทัดที่สำคัญที่สุด และเป็นบรรทัดที่คนลืมบ่อยที่สุด**
ถ้าไม่มี — loop ครบ 40 รอบแล้วยังไม่ได้ 200 สคริปต์ก็จะจบแบบ exit 0 → **workflow เขียวทั้งที่ deploy ไม่สำเร็จ**

**เวลารวมที่รอ = 40 × (สูงสุด 70 วิ + 15 วิ)** — ต้องคำนวณให้พอดีกับ `timeout-minutes` ของ job

**ทำไม `[ "$code" = "200" ]` ต้องมีเครื่องหมายคำพูด:** ถ้า `$code` เป็นค่าว่าง (เช่น curl ตายกลางคัน)
`[ = "200" ]` จะเป็น syntax error ส่วน `[ "" = "200" ]` แค่เป็นเท็จตามปกติ
**ครอบตัวแปรด้วย `"` เสมอ** เป็นนิสัยที่กันบั๊กได้มากที่สุดใน bash

### 1.3 `&&` `||` `;` — ต่างกันยังไง

```bash
a && b     # รัน b เฉพาะเมื่อ a สำเร็จ
a || b     # รัน b เฉพาะเมื่อ a ล้มเหลว
a ; b      # รัน b เสมอ
a || true  # กลบความล้มเหลวของ a ไม่ให้ set -e จับ
```

```bash
kubectl diff -k k8s/overlays/cloud || true
```

`kubectl diff` **คืน exit 1 เมื่อมีความต่าง** ซึ่งเป็นเรื่องปกติของทุก deploy
`|| true` จึงจำเป็น ไม่ใช่ความมักง่าย — **แต่ต้องแน่ใจว่ารู้ว่ากำลังกลบอะไรอยู่**

⚠️ กับดัก: `a && b || c` **ไม่ใช่ if-else** — ถ้า `a` สำเร็จแต่ `b` ล้มเหลว `c` ก็จะทำงานด้วย

### 1.4 `$( )` และ pipe

```bash
REPO=$(echo '${{ github.repository }}' | tr '[:upper:]' '[:lower:]')
```

| ส่วน | ทำอะไร |
| --- | --- |
| `$( ... )` | รันคำสั่งข้างใน แล้วแทนที่ด้วยผลลัพธ์ |
| `tr '[:upper:]' '[:lower:]'` | แปลงเป็นตัวพิมพ์เล็กทั้งหมด |

**ทำไมต้องแปลง:** container registry บังคับให้ชื่อ image เป็นตัวพิมพ์เล็ก
แต่ชื่อ GitHub repo มีตัวใหญ่ได้ — ถ้าไม่แปลงจะเจอ `invalid reference format` ตอน push

**ระวัง exit code ใน pipe:** ค่าที่ได้คือของ**คำสั่งสุดท้าย**เท่านั้น

```bash
false | echo "ok"     # exit 0 — ทั้งที่ false ล้มเหลว!
set -o pipefail       # ← เปิดแล้วจะ exit 1 ตามที่ควรจะเป็น
```

### 1.5 heredoc — ส่งข้อความหลายบรรทัดเข้าคำสั่ง

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
EOF
```

| รูปแบบ | ผลต่าง |
| --- | --- |
| `<<EOF` | **แทนค่าตัวแปร** ข้างใน (`$HOME` กลายเป็นค่าจริง) |
| `<<'EOF'` | **ไม่แทนค่า** ส่งไปตรง ๆ ทุกตัวอักษร |

**ใช้ `<<'EOF'` เมื่อส่ง YAML ที่มี `$` อยู่ข้างใน** ไม่งั้น shell จะไปแทนค่าให้โดยไม่ได้ตั้งใจ

### 1.6 การรันเบื้องหลังและ `wait`

```bash
kubectl port-forward svc/todo-api 18080:80 &   # & = รันเบื้องหลัง
sleep 5                                         # ให้เวลาเปิด tunnel
curl localhost:18080/healthz
```

```bash
( for i in $(seq 1 200); do ... done > /tmp/codes.txt ) &   # subshell ทำงานเบื้องหลัง
LOOP=$!                                                      # $! = PID ของงานล่าสุด
kubectl rollout restart deployment/todo-api
wait $LOOP                                                   # รอจนงานเบื้องหลังจบ
```

นี่คือหัวใจของการทดสอบ zero-downtime ใน `k8s-e2e.yml` — **ยิงโหลดค้างไว้ แล้วทำ deploy ทับระหว่างนั้น**
`wait` จำเป็น ไม่งั้นสคริปต์จะจบก่อนที่ loop จะเขียนไฟล์เสร็จ แล้วนับผลผิด

### 1.7 นับผลลัพธ์

```bash
bad=$(grep -vc '^200$' /tmp/codes.txt || true)
test "$bad" -eq 0 || { echo "::error::พลาด $bad ตัว"; exit 1; }
```

| ส่วน | ความหมาย |
| --- | --- |
| `-v` | เอาบรรทัดที่ **ไม่ตรง** |
| `-c` | นับจำนวนแทนการแสดง |
| `^200$` | ทั้งบรรทัดต้องเป็น `200` พอดี (ไม่ใช่ `2001`) |
| `\|\| true` | `grep` คืน exit 1 เมื่อไม่เจออะไรเลย ซึ่งในที่นี้คือผลลัพธ์ที่**ดี** |

บรรทัด `|| true` ตรงนี้เป็นตัวอย่างที่ดีของ "รู้ว่ากำลังกลบอะไร" — เรากลบเฉพาะกรณีที่ grep ไม่เจอ ซึ่งแปลว่าไม่มี request ไหนพลาดเลย

---

## ส่วนที่ 2 — อ่าน `deploy-k8s.yml` ทั้งไฟล์

ตอนนี้มาอ่านของจริงทีละบล็อก

### บล็อก 1 — trigger

```yaml
on:
  push:
    branches: [main]
    paths-ignore: ['**.md', 'docs/**', 'exercises/**']
  workflow_dispatch:
    inputs:
      image-digest:
        description: 'ระบุ digest เพื่อ deploy ย้อนเวอร์ชัน (เว้นว่าง = build ใหม่)'
        required: false
        type: string
```

**อ่านว่า:** ทำงาน 2 กรณี — push เข้า main (แต่ข้ามถ้าแก้แค่เอกสาร) หรือคนกดรันเองพร้อมระบุ digest ได้

**`paths-ignore` ประหยัดอะไร:** แก้ README แล้วไม่ต้อง build ARM image ที่กินเวลา 5 นาที
⚠️ แต่ระวัง — ถ้าตั้งเป็น required status check ไว้ PR ที่แก้แค่ `.md` จะรอ check ที่ไม่มีวันมา

### บล็อก 2 — permissions และ concurrency

```yaml
permissions:
  contents: read
  packages: write

concurrency:
  group: deploy-k8s
  cancel-in-progress: false
```

**`group` ไม่มี `${{ github.ref }}`** — จงใจ เพื่อให้ deploy จากทุก branch เข้าคิวเดียวกัน
ถ้าใส่ ref เข้าไป deploy จาก `main` กับจาก tag จะวิ่งพร้อมกันได้ → สอง `kubectl apply` ยิงเข้าคลัสเตอร์เดียวกัน

**`cancel-in-progress: false`** — ยกเลิก deploy กลางทาง = ทิ้งคลัสเตอร์ไว้ในสถานะที่ไม่มีใครรู้ว่าอยู่ตรงไหน

### บล็อก 3 — job build ที่ข้ามได้

```yaml
build:
  if: ${{ inputs.image-digest == '' }}
  uses: ./.github/workflows/build-push.yml
  with:
    tag-prefix: main
    platforms: linux/amd64,linux/arm64
  secrets: inherit
```

**อ่านว่า:** ถ้าไม่ได้ระบุ digest มา ให้ build ใหม่ / ถ้าระบุมาแล้ว **ข้าม job นี้ไปเลย**

**ตอน trigger เป็น `push`** `inputs` ไม่มีค่า → `inputs.image-digest` = `''` → เงื่อนไขเป็นจริง → build ตามปกติ

นี่คือกลไกที่ทำให้ **deploy ย้อนเวอร์ชันได้โดยไม่ต้อง build ใหม่** ซึ่งสำคัญมาก
เพราะการ build ใหม่จากโค้ดเดิม **ไม่รับประกันว่าจะได้ image เหมือนเดิม** (dependency อาจเปลี่ยน, base image อาจถูก push ทับ)

### บล็อก 4 — เงื่อนไขของ job deploy

```yaml
deploy:
  needs: [build]
  if: always() && (needs.build.result == 'success' || inputs.image-digest != '')
```

**ต้องอ่านสามส่วนแยกกัน:**

| ส่วน | ทำหน้าที่ |
| --- | --- |
| `needs: [build]` | รอ build ให้จบก่อน (ไม่ว่าผลจะเป็นอะไร) |
| `always() &&` | ยกเลิกกฎปริยาย `if: success()` ที่จะข้าม job นี้เมื่อ build ถูก skip |
| `(... \|\| ...)` | ระบุเงื่อนไขที่ยอมรับได้เอง 2 กรณี |

**ทำไมต้องมี `always()`:** เพราะเมื่อ build ถูก skip `needs.build.result` = `'skipped'`
ซึ่งกฎปริยายถือว่า **ไม่สำเร็จ** → job นี้จะถูกข้ามไปด้วย ทั้งที่เราตั้งใจให้ทำงาน

⚠️ ราคาที่จ่ายคือ `always()` ทำให้ job รันแม้ผู้ใช้กด Cancel — ในบริบท deploy ถือว่ารับได้เพราะเราไม่อยากให้หยุดกลางทางอยู่แล้ว

### บล็อก 5 — เลือก image

```yaml
- name: เตรียมชื่อ image
  id: img
  run: |
    REPO=$(echo '${{ github.repository }}' | tr '[:upper:]' '[:lower:]')
    if [ -n "${{ inputs.image-digest }}" ]; then
      echo "image=ghcr.io/$REPO@${{ inputs.image-digest }}" >> "$GITHUB_OUTPUT"
    else
      echo "image=${{ needs.build.outputs.image }}" >> "$GITHUB_OUTPUT"
    fi
```

| ส่วน | ความหมาย |
| --- | --- |
| `id: img` | ตั้งชื่อ step เพื่อให้ step อื่นอ้าง `steps.img.outputs.image` ได้ |
| `[ -n "..." ]` | จริงเมื่อสตริง **ไม่ว่าง** (`-z` คือตรงข้าม) |
| `>> "$GITHUB_OUTPUT"` | เขียนค่าส่งต่อให้ step ถัดไป |
| `@` ไม่ใช่ `:` | ต่อด้วย **digest** ไม่ใช่ tag |

**`ghcr.io/repo@sha256:...` vs `ghcr.io/repo:tag`** — ตัวแรกชี้ image ตัวเดียวเป๊ะตลอดกาล ตัวหลังเปลี่ยนได้ทุกเมื่อ

### บล็อก 6 — kubeconfig

```yaml
- name: เขียน kubeconfig
  run: |
    if [ -z "${{ secrets.KUBE_CONFIG }}" ]; then
      echo "::error::ยังไม่ได้ตั้ง secret KUBE_CONFIG"
      exit 1
    fi
    mkdir -p ~/.kube
    echo "${{ secrets.KUBE_CONFIG }}" | base64 -d > ~/.kube/config
    chmod 600 ~/.kube/config
    kubectl cluster-info
```

อ่านเจตนาทีละบรรทัด:

| บรรทัด | เจตนา |
| --- | --- |
| `if [ -z ... ]` | **ล้มเหลวให้ชัดตั้งแต่ต้น** พร้อมข้อความที่บอกว่าต้องทำอะไร ดีกว่าปล่อยให้ไปพังตอน kubectl แล้วอ่าน error ไม่รู้เรื่อง |
| `base64 -d` | secret เก็บเป็น base64 เพราะ kubeconfig มีหลายบรรทัดและมีอักขระพิเศษ |
| `chmod 600` | ให้เจ้าของอ่านได้คนเดียว — kubectl จะเตือนถ้าสิทธิ์กว้างเกิน |
| `kubectl cluster-info` | **ตรวจว่าต่อได้จริงก่อนทำอะไรต่อ** — fail fast |

**บรรทัดสุดท้ายคือแพตเทิร์นที่ควรเลียนแบบ:** ก่อนจะทำอะไรที่เปลี่ยนแปลงระบบ ให้ยิงคำสั่งอ่านอย่างเดียวสักตัวเพื่อยืนยันว่าเชื่อมต่อได้
ถ้าพังตรงนี้ เราจะรู้ทันทีว่าปัญหาคือ "ต่อไม่ได้" ไม่ใช่ "คำสั่ง deploy ผิด"

### บล็อก 7 — Secret

```yaml
- run: |
    kubectl create namespace todo-app --dry-run=client -o yaml | kubectl apply -f -
    kubectl -n todo-app create secret generic todo-secret \
      --from-literal=DATABASE_URL='${{ secrets.DATABASE_URL }}' \
      --dry-run=client -o yaml | kubectl apply -f -
```

สำนวน `create --dry-run=client -o yaml | apply -f -` = **"สร้างถ้ายังไม่มี อัปเดตถ้ามีแล้ว"**
(อธิบายละเอียดที่ [08 ภาคลึก D2](08-kubernetes.md))

**ทำไมต้องมาก่อน `apply -k`:** pod อ้างถึง Secret นี้ ถ้ายังไม่มี pod จะค้างที่ `CreateContainerConfigError`
และ k8s จะ **รอเงียบ ๆ** ไม่ล้มเหลวทันที → `rollout status` จะ timeout โดยไม่บอกสาเหตุจริง

⚠️ สังเกตว่า `'${{ secrets.DATABASE_URL }}'` ถูกครอบด้วย single quote — ถ้าไม่ครอบ อักขระพิเศษใน connection string (เช่น `&`, `?`) จะทำให้ shell ตีความผิด

### บล็อก 8 — deploy จริง

```yaml
- name: อัปเดต image แล้วรอผลจริง
  id: rollout
  run: |
    IMAGE="${{ steps.img.outputs.image }}"
    kubectl -n todo-app set image deployment/todo-api api="$IMAGE" migrate="$IMAGE"
    kubectl -n todo-app annotate deployment/todo-api \
      kubernetes.io/change-cause="commit ${{ github.sha }} โดย ${{ github.actor }}" --overwrite
    kubectl -n todo-app rollout status deployment/todo-api --timeout=300s
```

| บรรทัด | ทำไมต้องมี |
| --- | --- |
| `api=` **และ** `migrate=` | `migrate` เป็น initContainer — ลืมแล้ว migration จะรันจาก image เก่า |
| `annotate ... change-cause` | โผล่ใน `rollout history` — ตอนต้อง rollback จะรู้ว่าแต่ละเวอร์ชันคือ commit ไหน |
| `rollout status --timeout` | **บรรทัดที่ทำให้ workflow แดงจริงเมื่อ deploy ไม่สำเร็จ** |

**ถ้าไม่มีบรรทัดสุดท้าย:** `set image` คืนค่าทันทีโดยไม่รอ → workflow เขียวภายใน 2 วินาที ทั้งที่ pod กำลังเข้า `CrashLoopBackOff`

### บล็อก 9 — rollback

```yaml
- name: ย้อนกลับอัตโนมัติเมื่อล้มเหลว
  if: failure() && steps.rollout.outcome != 'skipped'
  run: |
    kubectl -n todo-app rollout undo deployment/todo-api
    kubectl -n todo-app rollout status deployment/todo-api --timeout=180s
    kubectl -n todo-app describe pods -l app=todo-api | tail -40
    echo "::error::deploy ล้มเหลวและย้อนกลับเวอร์ชันก่อนหน้าแล้ว"
```

| ส่วน | เหตุผล |
| --- | --- |
| `failure()` | ทำงานเฉพาะเมื่อมีอะไรพัง |
| `steps.rollout.outcome != 'skipped'` | ถ้ายังไม่ทันได้ deploy (เช่นพังตอนเขียน kubeconfig) **ก็ไม่ต้อง rollback** |
| `describe pods \| tail -40` | **เก็บหลักฐาน** — พรุ่งนี้ pod ที่พังถูกลบไปแล้ว แต่ Events อยู่ใน log นี้ |
| `::error::` | ทำให้ข้อความเด่นบนสุดของหน้า run |

**เงื่อนไขที่สองคือรายละเอียดที่แยกสคริปต์ที่คิดมาแล้วออกจากสคริปต์ที่ก็อปมา** — rollback ทั้งที่ยังไม่เคย deploy คือการทำให้สถานการณ์แย่ลง

---

## ส่วนที่ 3 — วิธีอ่าน workflow ที่ไม่เคยเห็นมาก่อน

ลำดับที่ควรทำเวลาเจอ workflow ของคนอื่น:

```
1. ดู `on:`         → มันทำงานเมื่อไร
2. ดู `permissions:` → มันมีสิทธิ์ทำอะไรได้บ้าง  ← บอกได้เยอะกว่าที่คิด
3. ไล่ `needs:`      → วาดลำดับ job
4. หา job ที่ "เปลี่ยนแปลงระบบ" → นั่นคือหัวใจ ที่เหลือคือส่วนประกอบ
5. อ่าน job นั้นทีละ step
6. หา step ที่ตรวจผล → ถ้าไม่มี แปลว่า pipeline นี้เชื่อไม่ได้
```

**ข้อ 2 มีประโยชน์มาก:** workflow ที่มี `permissions: contents: read` อย่างเดียว จะทำอะไรเสียหายไม่ได้เลย
ส่วนตัวที่มี `packages: write` หรือ `id-token: write` คือตัวที่ต้องอ่านละเอียด

**ข้อ 6 คือคำถามที่ควรถามกับทุก pipeline:** *"ถ้า deploy พัง workflow นี้จะรู้ได้ยังไง"*
ถ้าตอบไม่ได้ แปลว่ามันจะขึ้นเขียวเสมอ — และ pipeline ที่ขึ้นเขียวเสมอ ไม่ได้ให้ข้อมูลอะไรกับเราเลย

---

## ส่วนที่ 4 — เช็กลิสต์อ่าน script

ใช้ตรวจสคริปต์ที่กำลังจะ merge

- [ ] `curl` ทุกตัวมี `-f` หรือมีการเช็ค status code เอง
- [ ] loop ที่รอ มีการตรวจผลซ้ำ**หลัง** loop จบ
- [ ] `|| true` ทุกตัวมีคอมเมนต์บอกว่ากลบอะไร
- [ ] ตัวแปรทุกตัวถูกครอบด้วย `"` แล้ว
- [ ] ค่าที่มาจากผู้ใช้ภายนอกส่งผ่าน `env:` ไม่ใช่ฝังใน `run:`
- [ ] job ที่ deploy ตั้ง `cancel-in-progress: false`
- [ ] มี step ที่ตรวจผลปลายทางจริงหลัง deploy
- [ ] มี rollback หรือมีวิธีย้อนกลับที่เขียนไว้ชัดเจน
- [ ] `permissions` แคบที่สุดเท่าที่ทำงานได้

---

➡️ กลับไป [10 — deploy ขึ้น cloud ฟรี](10-deploy-free-cloud.md) แล้วอ่าน workflow อีกรอบ — จะเห็นภาพต่างไปจากรอบแรก
🏋️ ฝึกต่อ: [แบบฝึกหัด CI/CD](../exercises/cicd/01-beginner.md)
