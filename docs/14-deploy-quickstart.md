# 14 — Deploy Quickstart (ทำตามได้เลย ไม่ต้องอ่านทฤษฎี)

> คู่มือนี้ไม่อธิบายว่า "ทำไม" — อยากรู้เหตุผลเบื้องหลังแต่ละขั้นตอนไปอ่าน [10 — Deploy ขึ้น Cloud ฟรี](10-deploy-free-cloud.md)
> ไฟล์นี้มีแต่ **ทำอะไร ที่ไหน ค่าอะไร** เรียงเป็นข้อ ๆ ให้ก๊อปวางได้ทันที

เลือกเส้นทางเดียวพอ:

- **[ส่วน A — PaaS (Render + Neon)](#ส่วน-a--paas-render--neon-30-นาที)** — เร็วสุด ได้ URL จริงใน 30 นาที ไม่ต้องดูแล server
- **[ส่วน B — Kubernetes จริง (Oracle Cloud)](#ส่วน-b--kubernetes-จริงบน-oracle-cloud-2-3-ชม)** — ยาวกว่า แต่ได้ควบคุมทุกอย่างเอง (rollback, HPA, rolling update)

ทำ A ก่อนก็ได้ แล้วค่อยทำ B ต่อทีหลัง — ใช้ Neon database ตัวเดียวกัน (คนละ database ข้างในกันไว้)

---

## ส่วน A — PaaS (Render + Neon) — 30 นาที

### A1. สร้างฐานข้อมูลบน Neon

1. เข้า [neon.com](https://neon.com) → สมัครด้วย GitHub (ไม่ต้องใช้บัตร)
2. **Create project** → ตั้งชื่ออะไรก็ได้ เช่น `devops-todo-api` → เลือก region ใกล้ที่สุด
3. ในโปรเจกต์ที่สร้าง กด **Create database** → ตั้งชื่อ `todo_dev`
4. หน้า **Connection Details** → เลือก database `todo_dev` → คัดลอก connection string เส้น **ที่ไม่มี `-pooler`** (direct connection) หน้าตาแบบนี้:

   ```
   postgresql://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/todo_dev?sslmode=require
   ```

5. ทดสอบจากเครื่องตัวเองก่อนว่าต่อได้จริง:

   ```bash
   migrate -path migrations -database "postgresql://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/todo_dev?sslmode=require" up
   ```

   เห็น error → เช็คว่าใส่ `?sslmode=require` แล้วหรือยัง (Neon บังคับ TLS)

### A2. สร้าง Web Service บน Render

1. เข้า [render.com](https://render.com) → สมัครด้วย GitHub
2. **New +** → **Web Service** → แท็บ **Existing Image** (ไม่ใช่ "Build and deploy from a Git repository")
3. Image URL: `ghcr.io/<GITHUB_USERNAME>/devops-todo-api:dev` (ตัวพิมพ์เล็กทั้งหมด แทน `<GITHUB_USERNAME>` ด้วยชื่อ GitHub จริง)

   > **หา Image URL ที่ถูกต้องจากไหน?**
   > รูปแบบเต็มคือ `ghcr.io/<owner>/<repo>:<tag>` — มาจากชื่อ repo บน GitHub (`owner/repo`) ตัวพิมพ์เล็กทั้งหมด ไม่ใช่แค่ username เฉย ๆ (ดู [build-push.yml](../.github/workflows/build-push.yml) บรรทัดที่แปลง `github.repository` เป็นชื่อ image)
   > วิธีเช็คว่า push สำเร็จหรือยังและ tag อะไรมีจริง:
   > 1. ต้อง push ขึ้น `main`/`dev` อย่างน้อย 1 ครั้งให้ CI รัน job build-push ผ่านก่อน (ดู A3 ด้านล่าง)
   > 2. เข้าหน้า repo บน GitHub → แท็บ **Packages** ทางขวามือ (หรือ `https://github.com/<owner>/<repo>/pkgs/container/<repo>`) → จะเห็น image พร้อม tag ทั้งหมดที่ push ไปแล้วจริง (เช่น `dev`, `dev-a1b2c3d`)
   > 3. ก๊อป URL ตรงนั้นมาใช้ได้เลย ไม่ต้องเดา — ถ้ายังไม่เห็น package แปลว่า workflow ยังไม่รันผ่าน (เช็คแท็บ Actions)
4. Instance Type: **Free**
5. Region: **Singapore** (ใกล้ไทยสุด)
6. เลื่อนลงไปที่ **Environment Variables** → เพิ่ม:

   | Key | Value |
   | --- | --- |
   | `DATABASE_URL` | connection string ของ `todo_dev` จากขั้นตอน A1 (มี `?sslmode=require`) |

7. กด **Create Web Service** (ตอนนี้จะ deploy fail ก่อนเพราะยังไม่มี image ที่ tag `dev` ใน GHCR — ปกติ ข้ามไปทำ A3 ต่อ)
8. เข้า **Settings → Build & Deploy** → **Auto-Deploy** → ตั้งเป็น **No** (สำคัญ — ป้องกัน Render deploy ซ้ำกับ workflow)
9. เข้า **Settings → Deploy Hook** → คัดลอก URL เก็บไว้ (จะใช้ในขั้นตอน A4)

### A3. ทำให้ Render ดึง image จาก GHCR ได้

Package บน GHCR ต้องเป็น **public** ไม่งั้น Render ดึงไม่ได้ (ยังไม่มี package ตอนนี้ก็ข้ามไปก่อน ค่อยกลับมาทำหลัง push ครั้งแรก):

1. หลังจาก push ขึ้น `dev` ครั้งแรก (ทำในขั้นตอน A5) จะมี package ปรากฏที่หน้าโปรไฟล์/organization ของ GitHub → แท็บ **Packages**
2. เลือก package `devops-todo-api` → **Package settings** (ด้านขวา) → **Change visibility** → **Public**

### A4. ตั้งค่า Secret/Variable ใน GitHub

ที่ repo → **Settings → Secrets and variables → Actions**

แท็บ **Secrets** → **New repository secret**:

| ชื่อ | ค่า |
| --- | --- |
| `RENDER_DEPLOY_HOOK` | URL จาก A2 ข้อ 9 |

แท็บ **Variables** → **New repository variable**:

| ชื่อ | ค่า |
| --- | --- |
| `DEV_URL` | URL ของ Render เช่น `https://devops-todo-api-xxxx.onrender.com` (**ห้ามมี `/` ท้าย**) |

ยังต้องเปิดสิทธิ์ push package ด้วย: **Settings → Actions → General** → เลื่อนลงไป **Workflow permissions** → เลือก **Read and write permissions** → **Save**

### A5. Deploy จริง

```bash
git checkout -b dev
git push -u origin dev
```

ไปที่แท็บ **Actions** ของ repo — จะเห็น workflow `Deploy → PaaS (dev)` รันอยู่ มี 3 job เรียงกัน: `build` → `deploy` → `verify`

- **`build`** — build image, push ขึ้น `ghcr.io/<user>/devops-todo-api:dev`
- **`deploy`** — ยิง Render Deploy Hook ให้ไปดึง image ตัวใหม่มารัน
- **`verify`** — รอเครื่องตื่น (free tier หลับหลังไม่ใช้ 15 นาที) แล้วยิง `/healthz`, `/readyz`, สร้าง todo ทดสอบจริง

ถ้า workflow เขียวครบทั้ง 3 job แปลว่าเสร็จแล้ว เปิด URL ใน `DEV_URL` จากมือถือ (ปิด wifi) ลองยิง API ดูให้แน่ใจว่าใช้งานได้จริงจากนอกเครื่อง

### เช็คถ้าเจอปัญหา (เฉพาะที่เจอบ่อยสุด)

| อาการ | แก้ยังไง |
| --- | --- |
| Render ฟ้อง `manifest unknown` | package ยังเป็น private หรือยังไม่เคย push ขึ้น `dev` เลย — กลับไปทำ A3 |
| `deploy` job ข้ามไปเฉย ๆ ไม่ยิง Render | ยังไม่ได้ตั้ง secret `RENDER_DEPLOY_HOOK` — เช็ค A4 |
| `verify` fail ที่ `/healthz` ตลอด | เข้า Render dashboard → Logs ดู error จริง มักเป็น `DATABASE_URL` ผิดหรือไม่มี `?sslmode=require` |
| workflow ไม่รันเลยหลัง push | เช็คว่า push ไปแตะ branch `dev` จริง (`git branch` ดู branch ปัจจุบัน) |

รายละเอียดปัญหาอื่นดูที่ [12 — Troubleshooting](12-troubleshooting.md)

---

## ส่วน B — Kubernetes จริงบน Oracle Cloud — 2-3 ชม.

### B1. สร้าง database ที่สองบน Neon

ใช้ project เดิมจาก A1 — กด **Create database** อีกครั้ง ตั้งชื่อ `todo_prod` (แยกจาก `todo_dev` เด็ดขาด อย่าใช้ตัวเดียวกัน)
คัดลอก connection string เส้น **direct** (ไม่มี `-pooler`) เก็บไว้

### B2. สร้าง VM บน Oracle Cloud

1. เข้า [cloud.oracle.com](https://cloud.oracle.com) → สมัคร (ต้องใส่บัตรยืนยันตัวตน แต่ไม่ถูกตัดเงินถ้าอยู่ใน Always Free)
2. **Compute → Instances → Create Instance**
3. Image: **Ubuntu 22.04**
4. Shape: กด **Change Shape** → เลือก **Ampere → VM.Standard.A1.Flex** → ปรับเป็น **2 OCPU / 12 GB**
5. ใส่ SSH public key ของตัวเอง (หรือให้ Oracle generate แล้วโหลด private key เก็บไว้)
6. กด **Create** → รอ 1-2 นาที

ถ้าเจอ **"Out of host capacity"**: ลองสลับ Availability Domain (AD-1/2/3) หรือลองเวลาอื่นของวัน — ARM ฟรีคิวยาว

### B3. เปิด port — ต้องทำ 2 ที่ ไม่งั้นเข้าไม่ได้

**ที่ 1 — VCN Security List:**

1. **Networking → Virtual Cloud Networks** → เลือก VCN ของ instance
2. **Security Lists** → เลือก Default Security List → **Add Ingress Rules**
3. เพิ่ม 2 rule:

   | Source CIDR | Destination Port |
   | --- | --- |
   | `0.0.0.0/0` | `80` |
   | `0.0.0.0/0` | `443` |

**ที่ 2 — iptables ในเครื่อง (SSH เข้าไปทำ):**

```bash
ssh ubuntu@<PUBLIC_IP>

sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo netfilter-persistent save
```

### B4. ติดตั้ง k3s บน VM

ยังอยู่ใน SSH session เดิม:

```bash
curl -sfL https://get.k3s.io | sh -s - \
  --write-kubeconfig-mode 644 \
  --tls-san <PUBLIC_IP> \
  --disable metrics-server

sudo systemctl status k3s        # ต้องเป็น active (running)
sudo k3s kubectl get nodes       # ต้องเห็น node สถานะ Ready
```

แทน `<PUBLIC_IP>` ด้วย public IP ของ instance จริง (ดูได้จากหน้า Instance Details บน Oracle Console)

### B5. ดึง kubeconfig มาไว้บนเครื่องตัวเอง

```bash
# ยังอยู่ใน SSH — ดูไฟล์ก่อน
sudo cat /etc/rancher/k3s/k3s.yaml
```

```bash
# ออกจาก SSH กลับมาเครื่องตัวเอง
scp ubuntu@<PUBLIC_IP>:/etc/rancher/k3s/k3s.yaml ./kubeconfig

# แก้ 127.0.0.1 เป็น public IP จริง (macOS ใช้ -i '' ตามด้านล่าง, Linux ตัด '' ออก)
sed -i '' "s/127.0.0.1/<PUBLIC_IP>/" ./kubeconfig

# ทดสอบว่าต่อได้จากเครื่องตัวเอง
kubectl --kubeconfig=./kubeconfig get nodes
```

ต้องเห็น node ก่อนถึงจะไปขั้นตอนถัดไปได้ — ถ้าต่อไม่ได้ เช็ค B3 ว่าเปิด port ครบทั้ง 2 ที่จริงไหม

### B6. ตั้ง Tailscale ให้ GitHub Actions เชื่อมเข้าคลัสเตอร์อย่างปลอดภัย

(แนะนำแทนการเปิด port 6443 ออกอินเทอร์เน็ตตรง ๆ)

**บน VM (SSH เข้าไปอีกครั้ง):**

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
# เปิดลิงก์ที่ได้ใน browser เพื่อ login แล้วอนุมัติเครื่อง
tailscale ip -4        # จด IP แบบ 100.x.x.x ไว้ — เดี๋ยวใช้แทน public IP
```

**แก้ kubeconfig ให้ชี้ Tailscale IP แทน public IP:**

```bash
sed -i '' "s/<PUBLIC_IP>/<TAILSCALE_IP>/" ./kubeconfig
base64 -i ./kubeconfig | pbcopy    # macOS — คัดลอกเข้า clipboard ไว้แปะที่ GitHub Secret
```

**สร้าง OAuth client ที่ Tailscale:**

1. เข้า [Tailscale Admin Console](https://login.tailscale.com/admin/settings/oauth) → **Settings → OAuth clients**
2. **Generate OAuth client** → scope เลือก `auth_keys` → tag ใส่ `tag:ci`
3. คัดลอก **Client ID** และ **Client Secret** เก็บไว้

### B7. ตั้งค่า Secret/Variable ใน GitHub

ที่ repo → **Settings → Secrets and variables → Actions**

แท็บ **Secrets**:

| ชื่อ | ค่า |
| --- | --- |
| `KUBE_CONFIG` | kubeconfig ที่ base64 แล้วจาก B6 |
| `DATABASE_URL` | connection string ของ `todo_prod` จาก B1 |
| `TS_OAUTH_CLIENT_ID` | จาก B6 |
| `TS_OAUTH_SECRET` | จาก B6 |

แท็บ **Variables**:

| ชื่อ | ค่า |
| --- | --- |
| `PROD_URL` | `http://todo.<PUBLIC_IP>.nip.io` (แทน `<PUBLIC_IP>` ด้วย IP จริง) |

### B8. ตั้งชื่อโดเมนใน manifest

แก้ไฟล์ [k8s/overlays/cloud/kustomization.yaml](../k8s/overlays/cloud/kustomization.yaml) หา `todo.203.0.113.10.nip.io` แล้วแทนที่ IP ตรงนั้นด้วย public IP จริงของ VM (ใช้บริการ [nip.io](https://nip.io) ที่แปลง IP ในชื่อโดเมนกลับเป็น IP นั้นเอง ไม่ต้องซื้อโดเมนจริง)

Commit และ push การแก้ไขนี้ก่อนไปขั้นตอนถัดไป

### B9. Deploy

```bash
git checkout main
git push
```

ไปที่แท็บ **Actions** — workflow `Deploy → Kubernetes (cloud)` จะรัน:

1. build image multi-arch (amd64 + arm64) → push ขึ้น GHCR
2. เชื่อม Tailscale → เขียน kubeconfig
3. สร้าง k8s Secret จาก `DATABASE_URL`
4. `kubectl apply -k k8s/overlays/cloud`
5. `kubectl set image` + `rollout status` (รอจนกว่า pod ใหม่จะพร้อมจริง)
6. ยิงทดสอบ `/healthz`, `/readyz`, สร้าง todo จริงผ่าน `PROD_URL`

ถ้าล้มเหลวตรงไหน workflow จะ **`rollout undo` ให้อัตโนมัติ** กลับไปเวอร์ชันก่อนหน้า

เปิด URL ที่ตั้งไว้ใน `PROD_URL` จากมือถือ (ปิด wifi) ยืนยันว่าใช้งานได้จริง

### เช็คถ้าเจอปัญหา (เฉพาะที่เจอบ่อยสุด)

| อาการ | แก้ยังไง |
| --- | --- |
| `exec format error` | ลืม build arm64 — เช็คว่า `platforms: linux/amd64,linux/arm64` อยู่ใน `deploy-k8s.yml` จริง |
| `kubectl` จาก Actions ต่อไม่ได้ (`connection refused`/`x509`) | kubeconfig ยังชี้ IP เก่า หรือ Tailscale ไม่ได้เชื่อมสำเร็จ — เช็ค B6-B7 |
| Ingress 404 ทั้งที่ pod รันอยู่ | `ingressClassName` ต้องเป็น `traefik` ไม่ใช่ `nginx` (k3s แถม Traefik มาให้แล้ว) |
| เปิด port แล้วยังเข้าไม่ได้ | เปิดแค่ที่เดียว — ต้องเปิดทั้ง VCN Security List **และ** iptables (ดู B3) |
| `Out of host capacity` ตอนสร้าง VM | ลองสลับ Availability Domain หรือเวลาอื่น |

รายละเอียดปัญหาอื่นดูที่ [12 — Troubleshooting](12-troubleshooting.md) และ [10 — หัวข้อ 13](10-deploy-free-cloud.md#13-ปัญหาที่เจอบ่อย)

---

## เช็กลิสต์ก่อนถือว่าเสร็จ

- [ ] `git push` แล้ว workflow ทำงานเองครบทุก job โดยไม่ต้องกดอะไรเพิ่ม
- [ ] เปิด URL จากมือถือ (ปิด wifi) ใช้งานได้จริง ไม่ใช่แค่ localhost
- [ ] สร้าง todo ผ่าน curl แล้วข้อมูลยังอยู่หลัง deploy รอบถัดไป (พิสูจน์ว่า DB ไม่ได้อยู่ใน container ชั่วคราว)
- [ ] ไม่มี `DATABASE_URL` โผล่ในหน้า log ของ Actions เลย (ต้องเป็น Secret ไม่ใช่ Variable)

## ไปต่อ

| อยากรู้เพิ่ม | อ่านที่ |
| --- | --- |
| ทำไมต้องตั้งค่าแบบนี้ เหตุผลเบื้องหลังทุกขั้นตอน | [10 — Deploy ขึ้น Cloud ฟรี](10-deploy-free-cloud.md) |
| อ่าน workflow ทั้ง 5 ไฟล์ให้ออกทุกบรรทัด | [13 — อ่านสคริปต์ deploy](13-reading-scripts.md) |
| เปิด HTTPS ด้วย cert-manager | [10 — หัวข้อ 8.8](10-deploy-free-cloud.md#88-เปิด-https-ทำเมื่อ-http-ใช้ได้แล้ว) |
| ตั้ง rate limit / probe ให้ถูกชั้น | [09 — ตั้งค่าที่ชั้นไหนดี](09-where-to-configure.md) |
