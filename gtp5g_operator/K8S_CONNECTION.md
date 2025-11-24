# GTP5G Operator - 連接 K8s Gthulhu API

## 🔴 重要：正確的 API 連接方式

gtp5g_operator 需要連接到 **K8s 內部的 Gthulhu API**，而不是本地的 localhost:8080。

### 架構說明

```
┌─────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                     │
│                                                           │
│  ┌──────────────────┐         ┌──────────────────┐     │
│  │ gthulhu-api-pod  │◄────────│ gtp5g_operator   │     │
│  │   :8080          │  HTTP   │  (在 host 上)    │     │
│  └──────────────────┘         └──────────────────┘     │
│         │                              ▲                 │
│         │                              │                 │
│         │ ClusterIP                    │ Port-forward    │
│         │ gthulhu-api:80               │ :8081 → :8080  │
│         │                              │                 │
│  ┌──────▼──────────┐                   │                 │
│  │ gthulhu-scheduler│                   │                 │
│  │  (BPF Sched)    │                   │                 │
│  └─────────────────┘                   │                 │
└────────────────────────────────────────┼─────────────────┘
                                         │
                                  localhost:8081
```

## 🚀 快速開始

### 方法 1：使用啟動腳本（推薦）

```bash
# 終端 1: 啟動 port-forward
sudo microk8s.kubectl port-forward \
  $(sudo microk8s.kubectl get pods -l app=gthulhu-api -o jsonpath='{.items[0].metadata.name}') \
  8081:8080

# 終端 2: 啟動 operator
cd /home/ubuntu/Gthulhu/gtp5g_operator
./start_operator.sh
```

### 方法 2：手動啟動

```bash
# 1. 獲取 K8s Gthulhu 的 public key
POD_NAME=$(sudo microk8s.kubectl get pods -l app=gthulhu-api -o jsonpath='{.items[0].metadata.name}')
sudo microk8s.kubectl exec "$POD_NAME" -- cat /app/jwt_public_key.pem > /tmp/k8s_jwt_public_key.pem

# 2. 啟動 port-forward（另一個終端）
sudo microk8s.kubectl port-forward "$POD_NAME" 8081:8080

# 3. 啟動 operator
sudo API_ENDPOINT="http://localhost:8081" \
     PUBLIC_KEY_PATH="/tmp/k8s_jwt_public_key.pem" \
     ./gtp5g_operator
```

## 🔍 驗證策略是否生效

### 查詢當前策略（透過 K8s API）

```bash
# 1. 獲取 JWT token
TOKEN=$(jq -n --arg pk "$(cat /tmp/k8s_jwt_public_key.pem)" '{public_key: $pk}' | \
  curl -s -X POST http://localhost:8081/api/v1/auth/token \
    -H "Content-Type: application/json" -d @- | jq -r '.token')

# 2. 查詢策略
curl -s -X GET http://localhost:8081/api/v1/scheduling/strategies \
  -H "Authorization: Bearer $TOKEN" | jq '.'

# 3. 查詢特定 PID（nr-gnb 和 nr-ue 主進程）
curl -s -X GET http://localhost:8081/api/v1/scheduling/strategies \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.scheduling[] | select(.pid == 365162 or .pid == 365012)'
```

### 使用 Web UI 查看

訪問：http://localhost:8081/static/

## ⚙️ 環境變數配置

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `API_ENDPOINT` | `http://gthulhu-api:80` | Gthulhu API endpoint<br>• K8s 內部: `http://gthulhu-api:80`<br>• Port-forward: `http://localhost:8081` |
| `PUBLIC_KEY_PATH` | `/home/ubuntu/Gthulhu/api/config/jwt_public_key.pem` | JWT public key 路徑 |

## 📊 預期行為

當 operator 正常運作時，你應該會看到：

```
2025/11/23 13:35:04 Starting GTP5G Operator...
2025/11/23 13:35:04 API Endpoint: http://localhost:8081
2025/11/23 13:35:04 Starting trace_pipe parser...
2025/11/23 13:35:14 Successfully sent 22 strategies to Gthulhu API
2025/11/23 13:35:24 Successfully sent 22 strategies to Gthulhu API
```

## 🐛 常見問題

### Q1: "dial tcp: lookup gthulhu-api: server misbehaving"

**原因**: 在 host 上無法解析 K8s 內部 DNS

**解決方案**: 使用 port-forward 方式，設定 `API_ENDPOINT="http://localhost:8081"`

### Q2: "Public key verification failed"

**原因**: 使用了錯誤的 public key

**解決方案**: 
```bash
# 重新獲取 K8s 內的 public key
POD_NAME=$(sudo microk8s.kubectl get pods -l app=gthulhu-api -o jsonpath='{.items[0].metadata.name}')
sudo microk8s.kubectl exec "$POD_NAME" -- cat /app/jwt_public_key.pem > /tmp/k8s_jwt_public_key.pem
```

### Q3: 策略下到 localhost:8080 而非 K8s

**原因**: 之前測試時使用了錯誤的 endpoint

**解決方案**: 
- 確保使用正確的 `API_ENDPOINT`
- 驗證 port-forward 是否正常運作：`curl http://localhost:8081/health`

## 📝 部署到 K8s（未來）

當需要在 K8s 內部運行 operator 時，修改 deployment.yaml：

```yaml
env:
- name: API_ENDPOINT
  value: "http://gthulhu-api:80"  # 使用 K8s service
- name: PUBLIC_KEY_PATH
  value: "/config/jwt_public_key.pem"
```

然後將 public key 掛載為 ConfigMap 或 Secret。
