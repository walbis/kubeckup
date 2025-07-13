# OpenShift Cluster Backup Deployment

OpenShift için optimize edilmiş cluster backup sistemi. Default ServiceAccount kullanır ve minimal RBAC ile çalışır.

## 🎯 Özellikler

- ✅ **Default ServiceAccount**: Özel SA gereksiz
- ✅ **Minimal RBAC**: Tek wildcard kuralı (`apiGroups: ["*"], resources: ["*"], verbs: ["get", "list", "watch"]`)
- ✅ **No Custom SCC**: OpenShift'in `restricted-v2` SCC'sini kullanır
- ✅ **Dockerfile Optimized**: Group permissions OpenShift uyumlu
- ✅ **Production Ready**: 7-günlük otomatik cleanup

## 🚀 Hızlı Deployment

### Basit Deployment (Önerilen)
```bash
# 1. Namespace oluştur
oc new-project backup-system

# 2. Secret'ı yapılandır (önce düzenle!)
oc apply -f backup-secret-openshift.yaml

# 3. ConfigMap uygula
oc apply -f configmap-openshift.yaml  

# 4. RBAC uygula
oc apply -f rbac-default-sa.yaml

# 5. Backup CronJob deploy et
oc apply -f backup-cronjob-default-sa.yaml
```

### Monitoring ile Deployment (İsteğe Bağlı)
```bash
# Yukarıdaki adımlar + monitoring
oc apply -f monitoring-openshift.yaml
```

### Custom SCC ile Deployment (Enterprise)
```bash
# Tüm dosyaları uygula
oc apply -f namespace-backup-system.yaml
oc apply -f scc-backup.yaml
oc apply -f rbac-default-sa.yaml
oc apply -f backup-secret-openshift.yaml
oc apply -f configmap-openshift.yaml
oc apply -f backup-cronjob-default-sa.yaml
oc apply -f monitoring-openshift.yaml
```

## 📁 Dosyalar

| Dosya | Açıklama | Gerekli |
|-------|----------|---------|
| `rbac-default-sa.yaml` | Minimal RBAC (wildcard permissions) | ✅ |
| `backup-cronjob-default-sa.yaml` | Ana backup CronJob | ✅ |
| `backup-secret-openshift.yaml` | Konfigürasyon secrets | ✅ |
| `configmap-openshift.yaml` | OpenShift resource filtering | ✅ |
| `namespace-backup-system.yaml` | Namespace tanımı | ⚪ |
| `scc-backup.yaml` | Custom SecurityContextConstraints | ⚪ |
| `monitoring-openshift.yaml` | Prometheus monitoring | ⚪ |
| `deployment-instructions.md` | Detaylı kurulum kılavuzu | 📖 |

## 🔧 Konfigürasyon

### 1. Secret Düzenle
`backup-secret-openshift.yaml` dosyasında şunları güncelle:
```yaml
stringData:
  cluster-name: "your-openshift-cluster"
  minio-endpoint: "your-minio-endpoint:9000"
  minio-access-key: "your-access-key"
  minio-secret-key: "your-secret-key"
```

### 2. Image Registry Güncelle
`backup-cronjob-default-sa.yaml` dosyasında:
```yaml
image: your-registry.com/cluster-backup:latest
```

## 🎪 Test Et

```bash
# Manuel backup çalıştır
oc create job test-backup --from=cronjob/cluster-backup-default-sa -n backup-system

# Log'ları izle
oc logs -f job/test-backup -n backup-system

# Metrics kontrol et
oc port-forward service/cluster-backup-metrics 8080:8080 -n backup-system
curl http://localhost:8080/metrics
```

## 🏆 Neden Bu Kadar Basit?

### Dockerfile Optimizasyonu
```dockerfile
# OpenShift assigns random UIDs but always uses group 0 (root)
RUN mkdir -p /workspace /data && \
    chmod 775 /workspace /data && \
    chgrp -R root /workspace /data && \
    chmod -R g+rwX /workspace /data
```

### Minimal RBAC
```yaml
# Tek kural - her şeyi oku
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
```

### OpenShift Otomatiği
- **Random UID**: OpenShift atar (1000690000+)
- **Group 0**: Her zaman root grubu
- **restricted-v2**: Otomatik SCC
- **Security**: OpenShift yönetir

Bu yaklaşım OpenShift'in "convention over configuration" felsefesine uygun! 🚀