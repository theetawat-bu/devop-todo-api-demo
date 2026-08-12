# ☸️ Kubernetes — ระดับ 4: ขั้นสูง

> **เป้าหมาย:** GitOps, deployment strategy ขั้นสูง, observability และปัญหาที่เจอเฉพาะตอนระบบใหญ่
> **ต้องมีก่อน:** ผ่านระดับ 3
> **เวลาโดยประมาณ:** 3–5 วัน
> **อ่านประกอบ:** [docs/11 — Enterprise GitOps](../../docs/11-enterprise-gitops.md)

---

### K4.1 ติดตั้ง Argo CD แล้วเปลี่ยนมาใช้ GitOps

**โจทย์:** ติดตั้ง Argo CD ในคลัสเตอร์ แล้วให้มัน sync จาก `k8s/overlays/dev`

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl -n argocd port-forward svc/argocd-server 8080:443
```

**ผ่านเมื่อ:** แก้ replicas ใน git แล้ว push → คลัสเตอร์เปลี่ยนตามเองโดยไม่ต้อง `kubectl apply`

---

### K4.2 พิสูจน์ selfHeal

**โจทย์:** เปิด `selfHeal: true` แล้วลองแก้ replicas ด้วย `kubectl scale`
**ผ่านเมื่อ:** ค่าเด้งกลับไปตรงกับ git ภายในไม่กี่วินาที แล้วอธิบายได้ว่าทำไมนี่คือหัวใจของ GitOps

---

### K4.3 sync wave สำหรับ migration

**โจทย์:** ทำให้ Job migration รันจบก่อน Deployment จะเริ่ม
**คำใบ้:** annotation `argocd.argoproj.io/hook: PreSync` และ `sync-wave`
**ผ่านเมื่อ:** ดูใน Argo UI แล้วเห็นลำดับถูกต้อง และถ้า migration พัง Deployment จะไม่ถูก apply เลย

---

### K4.4 Blue/Green หรือ Canary

**โจทย์:** ทำ canary ส่ง traffic 10% ไปเวอร์ชันใหม่
**ทางเลือก:** Argo Rollouts (ง่ายกว่า) หรือ Ingress canary annotation (`nginx.ingress.kubernetes.io/canary: "true"` + `canary-weight: "10"`)
**ผ่านเมื่อ:** ยิง 100 request แล้วนับได้ประมาณ 10/90 และอธิบายได้ว่าต่างจาก rolling update ยังไง

---

### K4.5 เก็บ metrics ด้วย Prometheus

**โจทย์:** เพิ่ม `/metrics` ในแอปด้วย `prom-client` แล้วให้ Prometheus เก็บ
**คำใบ้:** ติดตั้ง kube-prometheus-stack ด้วย helm แล้วสร้าง `ServiceMonitor`
**ผ่านเมื่อ:** เห็น metric ของแอปใน Prometheus และสร้างกราฟ request rate ใน Grafana ได้

---

### K4.6 HPA จาก custom metric

**โจทย์:** เปลี่ยน HPA จากการดู CPU มาดูจำนวน request ต่อวินาทีแทน
**คำใบ้:** ต้องมี prometheus-adapter เพื่อแปลง metric ของ Prometheus ให้ k8s อ่านได้
**ผ่านเมื่อ:** scale ตาม rps จริง และอธิบายได้ว่าทำไม rps เป็นตัวชี้วัดที่ดีกว่า CPU สำหรับ API ที่รอ I/O เป็นหลัก

---

### K4.7 กระจาย log

**โจทย์:** ส่ง log ของทุก pod เข้า Loki แล้วค้นด้วย Grafana
**ผ่านเมื่อ:** ค้น log ด้วย `X-Request-Id` แล้วเจอ log ของทุก pod ที่เกี่ยวข้องกับ request นั้น

---

### K4.8 ทดสอบว่า node ดับแล้วเกิดอะไร

**โจทย์:** (ต้องมีหลาย node — `minikube start --nodes=3`) ทำให้ node หนึ่งดับ
**คำใบ้:** `kubectl cordon` + `kubectl drain` หรือ `minikube node stop`
**ผ่านเมื่อ:** จับเวลาได้ว่าใช้เวลากี่นาที pod ถึงย้ายไป node อื่น และอธิบายได้ว่าทำไมนานขนาดนั้น (`node-monitor-grace-period` + `tolerationSeconds`)

---

### K4.9 Helm chart

**โจทย์:** แปลง `k8s/base` เป็น Helm chart
**ผ่านเมื่อ:** `helm install` ใช้งานได้จริง แล้วเขียนเปรียบเทียบกับ kustomize — ข้อดี/ข้อเสียของแต่ละแบบ พร้อมเลือกว่าจะใช้อะไรกับโปรเจกต์นี้และเพราะอะไร

---

## ✅ เช็กลิสต์จบระดับ 4

- [ ] deploy ผ่าน git เท่านั้น ไม่ต้อง `kubectl apply` ด้วยมืออีก
- [ ] ปล่อยเวอร์ชันใหม่แบบค่อยเป็นค่อยไปได้ (canary/blue-green)
- [ ] มี metrics และ log ที่ debug ได้จริง
- [ ] เข้าใจพฤติกรรมของคลัสเตอร์เมื่อ node ล่ม

➡️ [เฉลยระดับ 4](../solutions/kubernetes/04-advanced.md) · [ไประดับ 5](05-expert.md)
