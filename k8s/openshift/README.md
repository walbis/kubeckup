# OpenShift Deployment Options

Bu klasör OpenShift için iki farklı deployment seçeneği sunar:

## 🎯 Deployment Seçenekleri

### 1. 🚀 Ultra-Simple Deployment (Önerilen)
**Dosyalar:**
- `simple-rbac-minimal.yaml` - Minimal RBAC
- `backup-cronjob-minimal.yaml` - Basit CronJob
- `namespace-simple.yaml` - Basit namespace
- `simple-deployment.md` - Kurulum kılavuzu

**Özellikler:**
- ✅ SCC gereksiz (OpenShift default kullanır)
- ✅ Minimal RBAC (sadece read-only)
- ✅ Default ServiceAccount
- ✅ 5 komutla deployment
- ✅ Dockerfile'da grup izinleri optimize edildi

### 2. 🔒 Enterprise Deployment (Gelişmiş)
**Dosyalar:**
- `rbac-default-sa.yaml` - Detaylı RBAC
- `scc-backup.yaml` - Özel SecurityContextConstraints  
- `backup-cronjob-default-sa.yaml` - Gelişmiş CronJob
- `monitoring-openshift.yaml` - ServiceMonitor & Alerts
- `deployment-instructions.md` - Detaylı kurulum

**Özellikler:**
- 🛡️ Özel SCC tanımı
- 📊 Prometheus monitoring entegrasyonu
- 🔍 Detaylı güvenlik konfigürasyonu
- 📋 Kapsamlı RBAC izinleri

## 🏃‍♂️ Hızlı Başlangıç

### Ultra-Simple (5 Komut)
```bash
oc new-project backup-system
oc apply -f backup-secret-openshift.yaml
oc apply -f configmap-openshift.yaml  
oc apply -f simple-rbac-minimal.yaml
oc apply -f backup-cronjob-minimal.yaml
```

### Enterprise (Daha Güvenli)
```bash
oc apply -f namespace-backup-system.yaml
oc apply -f scc-backup.yaml
oc apply -f rbac-default-sa.yaml
oc apply -f backup-secret-openshift.yaml
oc apply -f configmap-openshift.yaml
oc apply -f backup-cronjob-default-sa.yaml
oc apply -f monitoring-openshift.yaml
```

## 📚 Dokümantasyon

- **Ultra-Simple**: `simple-deployment.md` dosyasını okuyun
- **Enterprise**: `deployment-instructions.md` dosyasını okuyun

## 🤔 Hangisini Seçeyim?

| Durum | Öneri |
|-------|--------|
| Hızlı test/PoC | Ultra-Simple |
| Production ortam | Ultra-Simple (yeterli güvenlik) |
| Enterprise compliance | Enterprise |
| Özel monitoring gerekli | Enterprise |

**Not:** Ultra-Simple versiyonu bile production'da güvenlidir çünkü OpenShift'in kendi güvenlik mekanizmalarını kullanır (`restricted-v2` SCC otomatik uygulanır).