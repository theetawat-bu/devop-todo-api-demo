# เฉลย — 🐳 Docker ระดับ 5

⬅️ [กลับไปที่โจทย์](../../docker/05-expert.md)

> ระดับนี้ **ไม่มีคำตอบเดียว** — ด้านล่างคือแนวคำตอบและเกณฑ์ประเมินตัวเอง

---

## D5.1 SBOM

```bash
docker buildx build --sbom=true --provenance=true --push -t ghcr.io/<you>/todo:v1 .
docker buildx imagetools inspect ghcr.io/<you>/todo:v1 --format '{{ json .SBOM }}' > sbom.json
jq '.predicate.predicate.packages[].name' sbom.json | head
```

**เกณฑ์ผ่านจริง** อยู่ที่คำถามข้อนี้ — *"มี CVE ใหม่ของ library X จะรู้ได้ยังไงว่ากระทบ image ไหนบ้าง"*

คำตอบที่ใช้ได้จริงต้องมี 3 องค์ประกอบ:

1. SBOM ถูกเก็บไว้ที่ที่ค้นได้ (registry attestation หรือ Dependency-Track)
2. มีการเชื่อม SBOM → image digest → environment ที่รัน digest นั้นอยู่
3. มีคนหรือระบบที่รับ CVE feed แล้ว query ได้ภายในเวลาที่กำหนด

**ถ้าตอบว่า "grep ใน go.sum" = ยังไม่ผ่าน** เพราะไม่ครอบคลุม dependency ของ OS layer (alpine packages) และไม่รู้ว่า image ไหน deploy อยู่จริง

---

## D5.2 เซ็นลายเซ็น

```bash
cosign generate-key-pair
cosign sign --key cosign.key ghcr.io/<you>/todo@sha256:…
cosign verify --key cosign.pub ghcr.io/<you>/todo@sha256:…
```

แบบ keyless (ดีกว่า — ไม่ต้องเก็บ key):

```yaml
permissions:
  id-token: write # จำเป็นสำหรับ OIDC
  packages: write
steps:
  - uses: sigstore/cosign-installer@v3
  - run: cosign sign --yes ghcr.io/${{ github.repository }}@${{ steps.build.outputs.digest }}
```

**กันการโจมตีแบบไหน:** คนที่ได้สิทธิ์เขียน registry แอบ push image ปลอมทับ tag เดิม
ถ้าคลัสเตอร์ตรวจลายเซ็นก่อน pull image ปลอมจะถูกปฏิเสธ แม้จะอยู่ใน registry ของเราเองแล้วก็ตาม

**สำคัญ:** เซ็นแล้วไม่ verify = ไม่ได้อะไรเลย ต้องคู่กับ D5.3 เสมอ

---

## D5.3 policy

ตัวอย่าง Kyverno:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-nonroot
spec:
  validationFailureAction: Enforce
  rules:
    - name: check-runasnonroot
      match: { any: [{ resources: { kinds: [Pod] } }] }
      validate:
        message: "ทุก container ต้องตั้ง runAsNonRoot: true — ดูวิธีแก้ที่ wiki/container-security"
        pattern:
          spec:
            containers:
              - securityContext:
                  runAsNonRoot: true
```

**เกณฑ์คุณภาพอยู่ที่ข้อความ error** — ข้อความที่ดีบอก 3 อย่าง: ผิดกฎอะไร, แก้ยังไง, ไปอ่านต่อที่ไหน
policy ที่ปฏิเสธเฉย ๆ โดยไม่บอกวิธีแก้ จะทำให้ทีมพยายามหาทางหลบแทนที่จะทำตาม

**ลำดับการเปิดใช้ที่แนะนำ:** `Audit` ก่อน (แค่บันทึก) → ดูว่ามีอะไรผิดกฎบ้าง → แจ้งทีมพร้อมกำหนดเวลา → ค่อยเปลี่ยนเป็น `Enforce`
เปิด Enforce ทันทีวันแรก = ทีมอื่น deploy ไม่ได้กะทันหัน แล้วจะกลายเป็นศัตรูกับงานความปลอดภัยไปเลย

---

## D5.4 ลดเวลา build 30%

เรียงตามผลลัพธ์ที่มักได้:

| วิธี | ผลที่มักได้ |
| --- | --- |
| `cache-to: type=registry,mode=max` แทน `type=gha` | 20–40% (gha cache มีเพดาน 10GB และช้ากว่า) |
| แยก base image ที่มี system deps | 10–20% |
| ตัด platform ที่ไม่ได้ใช้ออก | 40–60% (ถ้าเคย build multi-arch โดยไม่จำเป็น) |
| `paths-ignore` ไม่ให้ build ตอนแก้แต่ `.md` | ประหยัดทั้ง run |
| cache mount สำหรับ npm | 10–20% |

**เกณฑ์ผ่าน:** ต้องมีตัวเลขจากหน้า Actions จริง ไม่ใช่จากเครื่องตัวเอง
และต้องระบุได้ว่าตอนนี้เวลาส่วนใหญ่หมดไปกับ step ไหน — **ถ้าไม่รู้ว่าคอขวดอยู่ไหน การจูนคือการเดา**

---

## D5.5 นโยบายอัปเดต base image

ตัวอย่าง `renovate.json`:

```json
{
  "extends": ["config:recommended"],
  "packageRules": [
    {
      "matchDatasources": ["docker"],
      "matchUpdateTypes": ["digest", "patch"],
      "automerge": true,
      "schedule": ["before 6am on monday"]
    },
    {
      "matchDatasources": ["docker"],
      "matchUpdateTypes": ["major"],
      "automerge": false
    }
  ]
}
```

ต้องตอบให้ครบ 3 ข้อ:

| คำถาม | คำตอบที่ใช้ได้ |
| --- | --- |
| ใครรับผิดชอบ | ระบุทีม/บทบาท ไม่ใช่ชื่อคน (คนลาออกได้) |
| ทุกกี่วัน | digest/patch อัตโนมัติทุกสัปดาห์, major ทบทวนรายไตรมาส |
| ถ้าพังจะรู้ตอนไหน | CI ต้องรันเทสจริงบน PR ของ Renovate ก่อน automerge |

ข้อสามคือหัวใจ — **automerge โดยไม่มีเทสที่เชื่อถือได้ = ปล่อยให้บอท deploy ของพังอัตโนมัติ**

---

## D5.6 runbook ฉุกเฉิน

โครงที่ใช้ได้จริง:

```markdown
# Runbook: เปลี่ยน image ด่วนเพราะ CVE

## เมื่อไรใช้: CVE ระดับ critical ที่มี exploit แล้ว / เป้าหมาย 30 นาที

1. [2 นาที] ยืนยันผลกระทบ — ค้น SBOM ว่า image ไหนมี package นี้
2. [1 นาที] แจ้ง incident channel + ตั้งคนนำ
3. [5 นาที] แก้ base image digest / อัปเดต dependency แล้วเปิด PR
4. [10 นาที] รอ CI + สแกนซ้ำยืนยันว่าหายจริง
5. [5 นาที] merge → deploy staging → smoke test
6. [5 นาที] deploy production พร้อมเฝ้า error rate
7. [2 นาที] ยืนยันว่าเวอร์ชันเก่าไม่มี pod ไหนรันอยู่แล้ว

## ถ้าแก้ไม่ทัน: [ทางเลือกลดความเสี่ยงชั่วคราว เช่น ปิด feature, จำกัด network]
## ติดต่อ: [ทีม/ช่องทาง]
```

**เกณฑ์ผ่าน:** ซ้อมจริงแล้วจับเวลาได้ และ**แก้ runbook ตรงจุดที่สะดุด**
runbook ที่ไม่เคยซ้อม = นิยายที่เขียนไว้อ่านเล่น

---

## D5.7 ADR เลือก base image

โครง ADR ที่ดี:

```markdown
# ADR-005: เลือก base image สำหรับบริการ Go

## สถานะ: ยอมรับแล้ว (2026-08-12)

## บริบท
มี 12 บริการที่เป็น Go ใช้ base image ต่างกัน 4 แบบ ทำให้แก้ CVE ต้องทำ 4 ที่

## ตัวเลือกที่พิจารณา
| | ขนาด | CVE (HIGH+) | build | debug | หมายเหตุ |
|---|---|---|---|---|---|
| alpine | 15MB | 1-2 | เร็ว | ดี | มี shell/apk ให้ debug ได้ |
| debian-slim | 25MB | 3-5 | เร็ว | ดี | glibc มาตรฐาน แต่ใหญ่กว่าโดยไม่จำเป็นสำหรับ static binary |
| distroless/static | 3-5MB | 0 | เร็ว | ยาก | ไม่มี shell เลย เหมาะกับ Go binary ที่ `CGO_ENABLED=0` |

## การตัดสินใจ
เลือก distroless/static สำหรับทุกบริการ

## เหตุผล
- Go binary ที่ compile ด้วย `CGO_ENABLED=0` ไม่ต้องพึ่ง libc เลย — ไม่มีเหตุผลทาง technical ที่ต้องมี shell หรือ package manager ติดไปด้วย
- ส่วนต่างขนาดกับ alpine เล็กน้อย (~10MB) แต่ CVE ลดจาก base image เหลือศูนย์เพราะไม่มี OS package ให้สแกนเจอเลย
- observability ทีมนี้ดีพอแล้ว (structured JSON log ผ่าน stdout + `/healthz`/`/readyz`) ทำให้ไม่ต้อง exec เข้า container บ่อยเหมือนสมัยที่ debug ด้วยการเข้าไปดูไฟล์ log ในเครื่อง

## ผลที่ตามมา
- debug ต้องพึ่ง `kubectl debug` แนบ ephemeral container แทนการ `exec -it sh` ตรง ๆ — ทีมต้องฝึกใช้เครื่องมือนี้ให้คล่อง
- migration ต้องแยกออกจาก image หลักไปเป็น initContainer เสมอ (ไม่มี shell ให้รัน `migrate ... && ./api` ต่อกัน)

## จะทบทวนเมื่อ
- ทีมพบว่าต้อง debug บ่อยจนเป็นคอขวดของงานจริง → พิจารณากลับไปใช้ alpine ชั่วคราวระหว่างสืบสวนปัญหา
- มี native dependency (cgo) ที่บังคับให้ต้อง `CGO_ENABLED=1` และพึ่ง glibc → ต้องเปลี่ยนไป debian-slim
```

**เกณฑ์ผ่าน:** ส่วน "เหตุผล" มี**ตัวเลขที่วัดมาจริง** ไม่ใช่ความเห็น และมีส่วน "จะทบทวนเมื่อ"
ADR ที่ไม่มีเงื่อนไขทบทวน จะกลายเป็นกฎที่ไม่มีใครกล้าแตะในอีก 3 ปี

---

## 🎯 ประเมินตัวเอง

- [ ] ตอบคำถาม "image ไหนกระทบบ้าง" ได้ภายใน 5 นาทีจริง
- [ ] policy บังคับใช้อยู่จริง ไม่ใช่แค่ Audit
- [ ] มีตัวเลขก่อน-หลังของทุกการปรับปรุงที่อ้าง
- [ ] เคยซ้อม runbook จริงและแก้จุดที่สะดุดแล้ว
- [ ] ADR ที่เขียนมีคนอื่นอ่านแล้วเข้าใจโดยไม่ต้องถาม
