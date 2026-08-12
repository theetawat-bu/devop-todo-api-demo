# 07 — CD: Build → Push GHCR → Deploy

ไฟล์: `.github/workflows/cd.yml`

## CI vs CD

| | CI | CD |
|---|---|---|
| ตอบคำถาม | "โค้ดนี้พังไหม" | "เอาโค้ดนี้ขึ้นให้ผู้ใช้ยังไง" |
| trigger | ทุก push / PR | merge เข้า main / ติด tag |
| ผลลัพธ์ | ✅ / ❌ | image ใน registry + ระบบที่อัปเดตแล้ว |

- **Continuous Delivery** = พร้อม deploy ตลอด แต่คนกดปุ่มเอง
- **Continuous Deployment** = ผ่านแล้วขึ้น production เอง ไม่มีคนกด

workflow นี้ทำ Delivery เป็นหลัก (ใส่ approval ที่ environment ได้)

## ตั้งค่าครั้งแรก

1. แก้ `k8s/base/api.yaml` เปลี่ยน `ghcr.io/OWNER/REPO` เป็น repo จริงของคุณ (ตัวพิมพ์เล็กทั้งหมด)
2. push โค้ดขึ้น GitHub
3. **Settings → Actions → General → Workflow permissions** → เลือก *Read and write permissions*
4. รอ workflow รันจบ → ดู image ที่แท็บ **Packages** ของ repo

ไม่ต้องสร้าง token อะไรเลย — `GITHUB_TOKEN` ที่ GitHub ให้มาอัตโนมัติพอแล้ว

## Permissions

```yaml
permissions:
  contents: read
  packages: write
```

ให้สิทธิ์เท่าที่ต้องใช้ (least privilege) — `packages: write` คือสิทธิ์ push ขึ้น GHCR

## Login เข้า registry

```yaml
- uses: docker/login-action@v3
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}
```

`secrets.GITHUB_TOKEN` เป็น token ชั่วคราวที่ GitHub สร้างให้ทุก run และเพิกถอนเมื่อจบ — ปลอดภัยกว่า PAT ที่เราสร้างเองเยอะ

## Tag strategy — เรื่องที่คนมองข้าม

```yaml
- uses: docker/metadata-action@v5
  with:
    images: ghcr.io/${{ github.repository }}
    tags: |
      type=ref,event=branch                                  # main
      type=sha,prefix=sha-,format=short                      # sha-a1b2c3d
      type=semver,pattern={{version}}                        # 1.2.3  (เมื่อ push tag v1.2.3)
      type=semver,pattern={{major}}.{{minor}}                # 1.2
      type=raw,value=latest,enable={{is_default_branch}}     # latest
```

**อย่า deploy ด้วย `latest`** เพราะ `latest` วันนี้กับพรุ่งนี้คนละ image → ไม่รู้ว่า production รันอะไรอยู่ → rollback ไม่ได้

ให้ deploy ด้วย tag ที่ระบุตัวตนได้ (`sha-a1b2c3d`) หรือดีที่สุดคือ **digest**:

```yaml
IMAGE="ghcr.io/${{ env.IMAGE_NAME }}@${{ needs.build-and-push.outputs.digest }}"
```

digest (`sha256:...`) คือ hash ของ image เอง เปลี่ยนแปลงไม่ได้เลย = deploy ซ้ำได้ผลเหมือนเดิม 100%
นี่คือหลัก **immutable deployment**

## ส่งค่าข้าม job

```yaml
outputs:
  digest: ${{ steps.build.outputs.digest }}
```

แล้ว job ถัดไปอ่านด้วย `needs.build-and-push.outputs.digest` — job แต่ละตัวรันคนละเครื่อง ส่งค่าข้ามกันต้องผ่าน outputs

## Deploy ขึ้น Kubernetes

```yaml
- run: kubectl apply -k k8s/overlays/production
- run: |
    IMAGE="ghcr.io/${{ env.IMAGE_NAME }}@${{ needs.build-and-push.outputs.digest }}"
    kubectl -n todo-app set image deployment/todo-api api=$IMAGE
    kubectl -n todo-app rollout status deployment/todo-api --timeout=180s

- name: Rollback on failure
  if: failure()
  run: kubectl -n todo-app rollout undo deployment/todo-api
```

3 บรรทัดนี้คือหัวใจ:

1. `apply -k` — ซิงก์ manifest ทั้งหมด (config, service, ingress)
2. `set image` — เปลี่ยนเฉพาะ image แล้ว k8s เริ่ม rolling update
3. `rollout status --timeout` — **รอจนสำเร็จจริง** ถ้าเกินเวลาให้ถือว่าล้มเหลว

ถ้าไม่มีข้อ 3 workflow จะขึ้นเขียวทั้งที่ pod เข้า CrashLoopBackOff อยู่ — CD หลอกตัวเองแบบคลาสสิก
`if: failure()` ทำให้ย้อนกลับเวอร์ชันก่อนหน้าอัตโนมัติ

## เตรียม secret `KUBE_CONFIG`

```bash
cat ~/.kube/config | base64 | pbcopy      # macOS
# นำไปวางที่ Settings → Secrets and variables → Actions → New repository secret
# ชื่อ: KUBE_CONFIG
```

ถ้ายังไม่ตั้ง job deploy จะข้ามไปพร้อม warning — ฝึกเฉพาะส่วน build/push ก่อนได้

⚠️ อย่าใช้ kubeconfig ของ admin ในงานจริง ให้สร้าง ServiceAccount ที่มีสิทธิ์เฉพาะ namespace นั้น
ถ้าคลัสเตอร์อยู่บนคลาวด์ ใช้ **OIDC federation** จะดีกว่า เพราะไม่ต้องเก็บ credential ระยะยาวไว้เลย

## Environment & approval

```yaml
environment: production
```

ไปตั้งที่ **Settings → Environments → production** แล้วเพิ่ม *Required reviewers*
→ workflow จะหยุดรอคนกด Approve ก่อน deploy = ได้ Continuous **Delivery** พร้อมประตูกั้น

## Deployment strategy ที่ควรรู้จัก

| แบบ | วิธีทำ | ข้อดี |
|---|---|---|
| **Rolling** (ใช้ในโปรเจกต์นี้) | ค่อย ๆ เปลี่ยน pod ทีละตัว | ง่าย ไม่ downtime |
| **Blue/Green** | ยกชุดใหม่ขึ้นคู่ขนาน แล้วสลับ traffic ทีเดียว | rollback เร็วมาก แต่เปลืองทรัพยากร 2 เท่า |
| **Canary** | ส่ง traffic 5% ไปเวอร์ชันใหม่ก่อน ดูแล้วค่อยเพิ่ม | เจอปัญหาโดยกระทบคนน้อย |

ใน `k8s/base/api.yaml` เราตั้ง `maxUnavailable: 0` + `maxSurge: 1` = สร้าง pod ใหม่ให้พร้อมก่อน ค่อยฆ่าตัวเก่า → ไม่มี downtime

➡️ ต่อไป: [08 — Kubernetes](08-kubernetes.md)
