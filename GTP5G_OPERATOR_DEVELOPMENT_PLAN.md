# GTP5G Operator 開發計畫

> **Issue**: #11 - gtp5g operator  
> **負責人**: ianchen0119  
> **開發方法**: Test-Driven Development (TDD)  
> **測試環境**: Docker + Kubernetes (minikube/kind)  
> **預計週期**: 6 週

---

## 📊 開發進度總覽

| 階段 | 狀態 | 開始日期 | 完成日期 | 進度 |
|------|------|----------|----------|------|
| [階段 0: 環境準備](#階段-0-環境準備) | ⬜ 未開始 | - | - | 0% |
| [階段 1: 項目初始化與設計](#階段-1-項目初始化與設計) | ⬜ 未開始 | - | - | 0% |
| [階段 2: CRD 與 API 開發 (TDD)](#階段-2-crd-與-api-開發-tdd) | ⬜ 未開始 | - | - | 0% |
| [階段 3: Controller 實現 (TDD)](#階段-3-controller-實現-tdd) | ⬜ 未開始 | - | - | 0% |
| [階段 4: Installer 容器開發](#階段-4-installer-容器開發) | ⬜ 未開始 | - | - | 0% |
| [階段 5: Helm Chart 整合](#階段-5-helm-chart-整合) | ⬜ 未開始 | - | - | 0% |
| [階段 6: E2E 測試與文檔](#階段-6-e2e-測試與文檔) | ⬜ 未開始 | - | - | 0% |
| [階段 7: PR 提交與 Review](#階段-7-pr-提交與-review) | ⬜ 未開始 | - | - | 0% |

**狀態圖例**: ⬜ 未開始 | 🔄 進行中 | ✅ 已完成 | ⚠️ 阻塞

---

## 階段 0: 環境準備

**目標**: 設置完整的開發和測試環境  
**預計時間**: 1-2 天  
**狀態**: ⬜ 未開始

### 0.1 開發工具檢查

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 確認 Go 1.22+ 已安裝
  ```bash
  go version
  # 預期輸出: go version go1.22.x
  ```
  
- [ ] 確認 Docker 已安裝並運行
  ```bash
  docker --version
  docker ps
  ```
  
- [ ] 確認 kubectl 已安裝
  ```bash
  kubectl version --client
  ```
  
- [ ] 確認 Helm 3.x 已安裝
  ```bash
  helm version
  ```

- [ ] 安裝 kubebuilder (用於生成 Operator 框架)
  ```bash
  # Linux/Mac
  curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
  chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
  
  # Windows (使用 WSL 或下載預編譯版本)
  ```

- [ ] 安裝 kind (Kubernetes in Docker)
  ```bash
  go install sigs.k8s.io/kind@latest
  ```

#### 完成標準
- [ ] 所有命令都能成功執行
- [ ] 記錄工具版本信息

**完成日期**: ____________  
**備註**: 
```
記錄實際安裝的版本：
- Go: 
- Docker: 
- kubectl: 
- Helm: 
- kubebuilder: 
- kind: 
```

---

### 0.2 Fork 並克隆項目

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 在 GitHub 上 Fork Gthulhu 項目
  - URL: https://github.com/Gthulhu/Gthulhu
  
- [ ] 克隆到本地
  ```bash
  cd C:\Users\thc1006\Downloads\open-source
  git clone https://github.com/YOUR_USERNAME/Gthulhu.git gthulhu-gtp5g
  cd gthulhu-gtp5g
  ```

- [ ] 添加 upstream remote
  ```bash
  git remote add upstream https://github.com/Gthulhu/Gthulhu.git
  git remote -v
  ```

- [ ] 創建 feature branch
  ```bash
  git checkout main
  git pull upstream main
  git checkout -b feature/gtp5g-operator
  ```

#### 完成標準
- [ ] 成功克隆項目
- [ ] upstream remote 設置正確
- [ ] feature branch 創建成功

**完成日期**: ____________  
**Commit Hash**: ____________

---

### 0.3 初始化開發環境

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 安裝項目依賴
  ```bash
  make dep
  ```

- [ ] 初始化 submodules
  ```bash
  git submodule init
  git submodule sync
  git submodule update
  ```

- [ ] 構建 scx 依賴
  ```bash
  cd scx
  meson setup build --prefix ~
  meson compile -C build
  cd ..
  ```

- [ ] 構建 libbpfgo
  ```bash
  cd libbpfgo
  make
  cd ..
  ```

- [ ] 驗證構建
  ```bash
  make build
  make lint
  ```

#### 完成標準
- [ ] 所有依賴安裝成功
- [ ] 項目能夠成功構建
- [ ] 無 linting 錯誤

**完成日期**: ____________  
**構建輸出**: 
```
記錄任何錯誤或警告：


```

---

### 0.4 Docker 測試環境設置

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建本地 Kubernetes 集群 (kind)
  ```bash
  # 創建集群配置文件
  cat <<EOF > kind-config.yaml
  kind: Cluster
  apiVersion: kind.x-k8s.io/v1alpha4
  nodes:
  - role: control-plane
  - role: worker
    extraMounts:
    - hostPath: /lib/modules
      containerPath: /lib/modules
      readOnly: true
  - role: worker
    extraMounts:
    - hostPath: /lib/modules
      containerPath: /lib/modules
      readOnly: true
  EOF
  
  # 創建集群
  kind create cluster --name gtp5g-test --config kind-config.yaml
  ```

- [ ] 驗證集群運行
  ```bash
  kubectl cluster-info
  kubectl get nodes
  ```

- [ ] 設置本地 Docker registry (用於測試鏡像)
  ```bash
  docker run -d -p 5000:5000 --restart=always --name registry registry:2
  
  # 連接 registry 到 kind 網絡
  docker network connect kind registry
  ```

- [ ] 配置 kubectl context
  ```bash
  kubectl config use-context kind-gtp5g-test
  ```

#### 完成標準
- [ ] kind 集群成功創建
- [ ] 2 個 worker 節點正常運行
- [ ] 本地 registry 可訪問

**完成日期**: ____________  
**集群信息**: 
```bash
# 記錄集群狀態
kubectl get nodes -o wide


```

---

### 0.5 TDD 工具鏈設置

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 安裝 Go 測試工具
  ```bash
  # Testify - 斷言庫
  go get github.com/stretchr/testify
  
  # Gomega - BDD 風格斷言
  go get github.com/onsi/gomega
  
  # Ginkgo - BDD 測試框架
  go get github.com/onsi/ginkgo/v2/ginkgo
  go install github.com/onsi/ginkgo/v2/ginkgo
  ```

- [ ] 安裝 controller-runtime 測試工具
  ```bash
  go get sigs.k8s.io/controller-runtime/pkg/envtest
  ```

- [ ] 設置測試覆蓋率工具
  ```bash
  go install github.com/axw/gocov/gocov@latest
  go install github.com/AlekSi/gocov-xml@latest
  ```

- [ ] 創建測試配置文件
  ```bash
  cat <<EOF > .coveragerc
  [run]
  branch = True
  source = .
  omit = 
      */test/*
      */mock/*
  EOF
  ```

#### 完成標準
- [ ] 所有測試工具安裝成功
- [ ] 能運行示例測試

**完成日期**: ____________  

---

### ✅ 階段 0 完成檢查清單

- [ ] 所有開發工具已安裝並驗證
- [ ] 項目已 fork 並克隆到本地
- [ ] feature branch 已創建
- [ ] 所有依賴已安裝
- [ ] 項目可成功構建
- [ ] Docker + kind 測試環境已就緒
- [ ] TDD 工具鏈已配置

**階段完成日期**: ____________  
**總耗時**: ____________  
**遇到的問題**: 
```


```

---

## 階段 1: 項目初始化與設計

**目標**: 使用 kubebuilder 初始化 Operator 項目並完成設計文檔  
**預計時間**: 2-3 天  
**狀態**: ⬜ 未開始

### 1.1 使用 kubebuilder 初始化項目

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 在項目根目錄創建 operator 子目錄
  ```bash
  mkdir -p operators/gtp5g-operator
  cd operators/gtp5g-operator
  ```

- [ ] 初始化 kubebuilder 項目
  ```bash
  kubebuilder init \
    --domain gthulhu.io \
    --repo github.com/Gthulhu/Gthulhu/operators/gtp5g-operator \
    --project-name gtp5g-operator
  ```

- [ ] 創建 API 和 Controller
  ```bash
  kubebuilder create api \
    --group operator \
    --version v1alpha1 \
    --kind GTP5GModule \
    --resource \
    --controller
  ```

- [ ] 驗證生成的文件結構
  ```bash
  tree -L 3
  ```

#### 完成標準
- [ ] kubebuilder 項目初始化成功
- [ ] GTP5GModule CRD 框架已生成
- [ ] Controller 框架已生成
- [ ] 目錄結構符合預期

**完成日期**: ____________  
**生成的文件**: 
```
operators/gtp5g-operator/
├── api/
│   └── v1alpha1/
│       ├── gtp5gmodule_types.go
│       └── zz_generated.deepcopy.go
├── controllers/
│   ├── gtp5gmodule_controller.go
│   └── suite_test.go
├── config/
│   ├── crd/
│   ├── manager/
│   ├── rbac/
│   └── samples/
├── Dockerfile
├── Makefile
├── PROJECT
└── main.go
```

**Commit**: 
```bash
git add operators/gtp5g-operator
git commit -m "chore(operator): initialize gtp5g-operator with kubebuilder

- Initialize kubebuilder project for gtp5g operator
- Create GTP5GModule CRD scaffolding
- Generate controller framework

Part of #11"
```

**Commit Hash**: ____________

---

### 1.2 設計文檔撰寫

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建設計文檔目錄
  ```bash
  mkdir -p docs/design
  ```

- [ ] 撰寫架構設計文檔
  - [ ] 創建 `docs/design/gtp5g-operator-architecture.md`
  - [ ] 包含系統架構圖（可用 Mermaid）
  - [ ] 定義組件職責
  - [ ] 數據流圖

- [ ] 撰寫 API 設計文檔
  - [ ] 創建 `docs/design/gtp5g-operator-api.md`
  - [ ] 定義 CRD Spec 字段
  - [ ] 定義 Status 字段
  - [ ] 示例 YAML

- [ ] 撰寫部署設計文檔
  - [ ] 創建 `docs/design/gtp5g-operator-deployment.md`
  - [ ] DaemonSet 設計
  - [ ] Installer 容器設計
  - [ ] RBAC 設計

#### 完成標準
- [ ] 所有設計文檔已創建
- [ ] 文檔包含清晰的圖表
- [ ] API 設計已定義完整
- [ ] 通過團隊 review（如適用）

**完成日期**: ____________  

**Commit**: 
```bash
git add docs/design/
git commit -m "docs(operator): add gtp5g operator design documentation

- Add architecture design document
- Define GTP5GModule CRD API specification
- Document deployment strategy

Part of #11"
```

**Commit Hash**: ____________

---

### 1.3 編寫初始測試框架

**狀態**: ⬜ 未開始

#### 任務清單 (TDD 第一步)
- [ ] 創建測試目錄結構
  ```bash
  cd operators/gtp5g-operator
  mkdir -p test/{unit,integration,e2e}
  ```

- [ ] 編寫 CRD 驗證測試（先寫測試！）
  - [ ] 創建 `api/v1alpha1/gtp5gmodule_types_test.go`
  ```go
  package v1alpha1_test
  
  import (
      "testing"
      . "github.com/onsi/ginkgo/v2"
      . "github.com/onsi/gomega"
  )
  
  func TestGTP5GModuleAPI(t *testing.T) {
      RegisterFailHandler(Fail)
      RunSpecs(t, "GTP5GModule API Suite")
  }
  
  var _ = Describe("GTP5GModule", func() {
      Context("When creating a GTP5GModule", func() {
          It("should have valid default values", func() {
              // TODO: 實現測試
          })
          
          It("should validate required fields", func() {
              // TODO: 實現測試
          })
      })
  })
  ```

- [ ] 編寫 Controller 基礎測試（先寫測試！）
  - [ ] 創建 `controllers/gtp5gmodule_controller_test.go`
  ```go
  var _ = Describe("GTP5GModule Controller", func() {
      Context("When reconciling a GTP5GModule", func() {
          It("should create a DaemonSet", func() {
              // TODO: 實現測試
          })
      })
  })
  ```

- [ ] 運行測試（預期失敗 - Red phase）
  ```bash
  make test
  ```

#### 完成標準
- [ ] 測試框架已搭建
- [ ] 測試能運行（即使失敗）
- [ ] 測試覆蓋主要場景

**完成日期**: ____________  
**測試輸出**: 
```
# 記錄初始測試運行結果（預期失敗）


```

**Commit**: 
```bash
git add test/ api/v1alpha1/*_test.go controllers/*_test.go
git commit -m "test(operator): add initial test framework for TDD

- Create test directory structure
- Add GTP5GModule API validation tests (failing)
- Add controller reconciliation tests (failing)

Following TDD red-green-refactor cycle.

Part of #11"
```

**Commit Hash**: ____________

---

### ✅ 階段 1 完成檢查清單

- [ ] kubebuilder 項目初始化完成
- [ ] 設計文檔已撰寫並 review
- [ ] 測試框架已建立（TDD Red phase）
- [ ] 所有更改已 commit

**階段完成日期**: ____________  
**總耗時**: ____________  

---

## 階段 2: CRD 與 API 開發 (TDD)

**目標**: 實現 GTP5GModule CRD 的完整 API 定義  
**預計時間**: 2-3 天  
**狀態**: ⬜ 未開始  
**TDD 循環**: Red → Green → Refactor

### 2.1 TDD Red Phase - 編寫 API 測試

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 測試用例 1: 必填字段驗證
  ```go
  // api/v1alpha1/gtp5gmodule_validation_test.go
  var _ = Describe("GTP5GModule Validation", func() {
      It("should reject GTP5GModule without version", func() {
          module := &GTP5GModule{
              Spec: GTP5GModuleSpec{
                  // Version 未設置
              },
          }
          err := k8sClient.Create(ctx, module)
          Expect(err).To(HaveOccurred())
          Expect(err.Error()).To(ContainSubstring("version is required"))
      })
  })
  ```

- [ ] 測試用例 2: 默認值設置
  ```go
  It("should set default node selector if not provided", func() {
      module := &GTP5GModule{
          ObjectMeta: metav1.ObjectMeta{
              Name:      "test-module",
              Namespace: "default",
          },
          Spec: GTP5GModuleSpec{
              Version: "v0.8.3",
          },
      }
      Expect(k8sClient.Create(ctx, module)).To(Succeed())
      
      // 獲取創建的對象
      created := &GTP5GModule{}
      Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(module), created)).To(Succeed())
      
      // 驗證默認值
      Expect(created.Spec.NodeSelector).ToNot(BeNil())
  })
  ```

- [ ] 測試用例 3: 狀態更新
  ```go
  It("should update status phase correctly", func() {
      // TODO: 實現狀態更新測試
  })
  ```

- [ ] 運行測試確認失敗
  ```bash
  cd operators/gtp5g-operator
  make test
  # 預期輸出: FAIL (因為實現尚未完成)
  ```

#### 完成標準
- [ ] 至少 5 個測試用例
- [ ] 測試覆蓋所有 API 字段
- [ ] 測試運行失敗（Red phase）

**完成日期**: ____________  
**測試結果**: 
```
# 記錄失敗的測試數量和原因


```

**Commit**: 
```bash
git add api/v1alpha1/*_test.go
git commit -m "test(operator): add comprehensive GTP5GModule API tests (RED phase)

- Add validation tests for required fields
- Add default value tests
- Add status update tests

Tests are failing as expected in TDD red phase.

Part of #11"
```

**Commit Hash**: ____________

---

### 2.2 TDD Green Phase - 實現 API

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 實現 GTP5GModuleSpec 結構
  ```go
  // api/v1alpha1/gtp5gmodule_types.go
  
  // GTP5GModuleSpec defines the desired state of GTP5GModule
  type GTP5GModuleSpec struct {
      // Version 是 gtp5g 模塊的版本 (git tag)
      // +kubebuilder:validation:Required
      // +kubebuilder:validation:Pattern=^v[0-9]+\.[0-9]+\.[0-9]+$
      Version string `json:"version"`
      
      // KernelVersion 指定目標內核版本（可選，默認自動檢測）
      // +optional
      KernelVersion string `json:"kernelVersion,omitempty"`
      
      // NodeSelector 選擇要安裝模塊的節點
      // +optional
      NodeSelector map[string]string `json:"nodeSelector,omitempty"`
      
      // Image 是安裝器容器鏡像（可選）
      // +optional
      Image string `json:"image,omitempty"`
  }
  ```

- [ ] 實現 GTP5GModuleStatus 結構
  ```go
  // GTP5GModuleStatus defines the observed state of GTP5GModule
  type GTP5GModuleStatus struct {
      // Phase 表示當前狀態
      // +optional
      Phase ModulePhase `json:"phase,omitempty"`
      
      // InstalledNodes 是已成功安裝模塊的節點列表
      // +optional
      InstalledNodes []string `json:"installedNodes,omitempty"`
      
      // FailedNodes 是安裝失敗的節點列表
      // +optional
      FailedNodes []NodeFailure `json:"failedNodes,omitempty"`
      
      // Message 提供人類可讀的狀態信息
      // +optional
      Message string `json:"message,omitempty"`
      
      // LastUpdateTime 是最後更新時間
      // +optional
      LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
  }
  
  // ModulePhase 是模塊的生命週期階段
  // +kubebuilder:validation:Enum=Pending;Installing;Installed;Failed
  type ModulePhase string
  
  const (
      ModulePhasePending    ModulePhase = "Pending"
      ModulePhaseInstalling ModulePhase = "Installing"
      ModulePhaseInstalled  ModulePhase = "Installed"
      ModulePhaseFailed     ModulePhase = "Failed"
  )
  
  // NodeFailure 記錄節點安裝失敗信息
  type NodeFailure struct {
      NodeName string `json:"nodeName"`
      Reason   string `json:"reason"`
  }
  ```

- [ ] 實現 Webhook 驗證（可選但推薦）
  ```bash
  kubebuilder create webhook \
    --group operator \
    --version v1alpha1 \
    --kind GTP5GModule \
    --defaulting \
    --programmatic-validation
  ```

- [ ] 實現默認值邏輯
  ```go
  // api/v1alpha1/gtp5gmodule_webhook.go
  
  func (r *GTP5GModule) Default() {
      if r.Spec.NodeSelector == nil {
          r.Spec.NodeSelector = map[string]string{
              "gtp5g.gthulhu.io/enabled": "true",
          }
      }
      
      if r.Spec.Image == "" {
          r.Spec.Image = "localhost:5000/gtp5g-installer:latest"
      }
  }
  ```

- [ ] 實現驗證邏輯
  ```go
  func (r *GTP5GModule) ValidateCreate() error {
      if r.Spec.Version == "" {
          return fmt.Errorf("version is required")
      }
      return nil
  }
  ```

- [ ] 重新生成 CRD manifests
  ```bash
  make manifests
  ```

- [ ] 運行測試確認通過
  ```bash
  make test
  # 預期輸出: PASS
  ```

#### 完成標準
- [ ] 所有 API 字段已實現
- [ ] Webhook 驗證已實現
- [ ] CRD manifests 已生成
- [ ] 所有測試通過（Green phase）

**完成日期**: ____________  
**測試結果**: 
```
# 記錄通過的測試數量
=== RUN   TestGTP5GModuleAPI
Running Suite: GTP5GModule API Suite
Ran X specs in Y seconds
SUCCESS! -- X Passed | 0 Failed


```

**Commit**: 
```bash
git add api/v1alpha1/ config/crd/
git commit -m "feat(operator): implement GTP5GModule CRD API (GREEN phase)

- Define GTP5GModuleSpec with validation
- Define GTP5GModuleStatus with phase tracking
- Implement webhook for defaulting and validation
- Generate CRD manifests

All tests passing.

Part of #11"
```

**Commit Hash**: ____________

---

### 2.3 TDD Refactor Phase - 代碼優化

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 代碼審查清單
  - [ ] 移除重複代碼
  - [ ] 改進變量命名
  - [ ] 添加必要的註釋
  - [ ] 符合 Go 代碼規範

- [ ] 運行 linter
  ```bash
  make lint
  ```

- [ ] 優化測試代碼
  - [ ] 提取共用測試 helper
  - [ ] 改進測試可讀性
  - [ ] 添加表格驅動測試

- [ ] 確保測試仍然通過
  ```bash
  make test
  ```

#### 完成標準
- [ ] 無 linting 錯誤
- [ ] 代碼符合 Go 最佳實踐
- [ ] 測試仍然全部通過

**完成日期**: ____________  

**Commit**: 
```bash
git add api/v1alpha1/
git commit -m "refactor(operator): optimize GTP5GModule API code (REFACTOR phase)

- Extract common validation logic
- Improve code documentation
- Optimize test helpers

Part of #11"
```

**Commit Hash**: ____________

---

### 2.4 Docker 測試 - CRD 安裝

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 構建並安裝 CRD 到測試集群
  ```bash
  cd operators/gtp5g-operator
  
  # 生成 CRD YAML
  make manifests
  
  # 安裝到 kind 集群
  kubectl apply -f config/crd/bases/operator.gthulhu.io_gtp5gmodules.yaml
  ```

- [ ] 驗證 CRD 安裝
  ```bash
  kubectl get crd gtp5gmodules.operator.gthulhu.io
  kubectl describe crd gtp5gmodules.operator.gthulhu.io
  ```

- [ ] 測試創建示例 CR
  ```bash
  cat <<EOF | kubectl apply -f -
  apiVersion: operator.gthulhu.io/v1alpha1
  kind: GTP5GModule
  metadata:
    name: test-gtp5g
  spec:
    version: v0.8.3
  EOF
  ```

- [ ] 驗證 CR 創建
  ```bash
  kubectl get gtp5gmodules
  kubectl describe gtp5gmodule test-gtp5g
  ```

- [ ] 測試驗證規則
  ```bash
  # 測試無效版本（應該失敗）
  cat <<EOF | kubectl apply -f -
  apiVersion: operator.gthulhu.io/v1alpha1
  kind: GTP5GModule
  metadata:
    name: invalid-gtp5g
  spec:
    version: invalid
  EOF
  # 預期: Error from server (Invalid)
  ```

- [ ] 清理測試資源
  ```bash
  kubectl delete gtp5gmodule --all
  kubectl delete -f config/crd/bases/operator.gthulhu.io_gtp5gmodules.yaml
  ```

#### 完成標準
- [ ] CRD 成功安裝到集群
- [ ] 能創建有效的 CR
- [ ] 驗證規則正常工作
- [ ] 無效輸入被正確拒絕

**完成日期**: ____________  
**測試輸出**: 
```
# 記錄 kubectl 命令輸出


```

---

### ✅ 階段 2 完成檢查清單

- [ ] TDD Red Phase: 測試已編寫並失敗
- [ ] TDD Green Phase: 實現完成，測試通過
- [ ] TDD Refactor Phase: 代碼已優化
- [ ] Docker 測試: CRD 在 kind 集群中驗證通過
- [ ] 所有更改已 commit

**階段完成日期**: ____________  
**總耗時**: ____________  
**測試覆蓋率**: ____________%

---

## 階段 3: Controller 實現 (TDD)

**目標**: 實現 GTP5GModule Controller 的 Reconcile 邏輯  
**預計時間**: 3-4 天  
**狀態**: ⬜ 未開始  
**TDD 循環**: Red → Green → Refactor

### 3.1 TDD Red Phase - 編寫 Controller 測試

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 設置 Controller 測試環境
  ```go
  // controllers/suite_test.go
  var (
      k8sClient client.Client
      testEnv   *envtest.Environment
      ctx       context.Context
      cancel    context.CancelFunc
  )
  
  var _ = BeforeSuite(func() {
      ctx, cancel = context.WithCancel(context.Background())
      
      By("bootstrapping test environment")
      testEnv = &envtest.Environment{
          CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
          ErrorIfCRDPathMissing: true,
      }
      
      cfg, err := testEnv.Start()
      Expect(err).NotTo(HaveOccurred())
      
      k8sClient, err = client.New(cfg, client.Options{})
      Expect(err).NotTo(HaveOccurred())
  })
  
  var _ = AfterSuite(func() {
      cancel()
      By("tearing down the test environment")
      err := testEnv.Stop()
      Expect(err).NotTo(HaveOccurred())
  })
  ```

- [ ] 測試用例 1: DaemonSet 創建
  ```go
  var _ = Describe("GTP5GModule Controller", func() {
      Context("When reconciling a GTP5GModule", func() {
          It("should create a DaemonSet for installer", func() {
              module := &operatorv1alpha1.GTP5GModule{
                  ObjectMeta: metav1.ObjectMeta{
                      Name:      "test-module",
                      Namespace: "default",
                  },
                  Spec: operatorv1alpha1.GTP5GModuleSpec{
                      Version: "v0.8.3",
                  },
              }
              
              Expect(k8sClient.Create(ctx, module)).To(Succeed())
              
              // 等待 reconciliation
              Eventually(func() bool {
                  dsList := &appsv1.DaemonSetList{}
                  err := k8sClient.List(ctx, dsList, client.InNamespace("default"))
                  if err != nil {
                      return false
                  }
                  return len(dsList.Items) > 0
              }, timeout, interval).Should(BeTrue())
              
              // 驗證 DaemonSet
              ds := &appsv1.DaemonSet{}
              err := k8sClient.Get(ctx, types.NamespacedName{
                  Name:      "gtp5g-installer-test-module",
                  Namespace: "default",
              }, ds)
              Expect(err).NotTo(HaveOccurred())
              Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
              Expect(ds.Spec.Template.Spec.Containers[0].Name).To(Equal("installer"))
          })
      })
  })
  ```

- [ ] 測試用例 2: 狀態更新
  ```go
  It("should update status to Installing when DaemonSet is created", func() {
      // TODO: 實現狀態更新測試
  })
  ```

- [ ] 測試用例 3: DaemonSet 更新
  ```go
  It("should update DaemonSet when spec changes", func() {
      // TODO: 實現更新測試
  })
  ```

- [ ] 測試用例 4: 資源清理
  ```go
  It("should delete DaemonSet when GTP5GModule is deleted", func() {
      // TODO: 實現 finalizer 測試
  })
  ```

- [ ] 運行測試確認失敗
  ```bash
  make test
  ```

#### 完成標準
- [ ] 至少 8 個測試用例
- [ ] 覆蓋所有 reconcile 場景
- [ ] 測試運行失敗（Red phase）

**完成日期**: ____________  

**Commit**: 
```bash
git add controllers/*_test.go
git commit -m "test(operator): add controller reconciliation tests (RED phase)

- Add DaemonSet creation tests
- Add status update tests
- Add resource cleanup tests

Tests failing as expected in TDD red phase.

Part of #11"
```

**Commit Hash**: ____________

---

### 3.2 TDD Green Phase - 實現 Controller

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 實現 Reconcile 函數框架
  ```go
  // controllers/gtp5gmodule_controller.go
  
  func (r *GTP5GModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
      log := log.FromContext(ctx)
      
      // 獲取 GTP5GModule 實例
      module := &operatorv1alpha1.GTP5GModule{}
      if err := r.Get(ctx, req.NamespacedName, module); err != nil {
          if apierrors.IsNotFound(err) {
              return ctrl.Result{}, nil
          }
          return ctrl.Result{}, err
      }
      
      // 處理刪除
      if !module.DeletionTimestamp.IsZero() {
          return r.handleDeletion(ctx, module)
      }
      
      // 添加 finalizer
      if !containsString(module.Finalizers, finalizerName) {
          module.Finalizers = append(module.Finalizers, finalizerName)
          if err := r.Update(ctx, module); err != nil {
              return ctrl.Result{}, err
          }
      }
      
      // Reconcile DaemonSet
      if err := r.reconcileDaemonSet(ctx, module); err != nil {
          return ctrl.Result{}, err
      }
      
      // 更新狀態
      if err := r.updateStatus(ctx, module); err != nil {
          return ctrl.Result{}, err
      }
      
      return ctrl.Result{}, nil
  }
  ```

- [ ] 實現 DaemonSet 創建/更新邏輯
  ```go
  func (r *GTP5GModuleReconciler) reconcileDaemonSet(
      ctx context.Context,
      module *operatorv1alpha1.GTP5GModule,
  ) error {
      // 構造期望的 DaemonSet
      desired := r.constructDaemonSet(module)
      
      // 檢查是否已存在
      existing := &appsv1.DaemonSet{}
      err := r.Get(ctx, types.NamespacedName{
          Name:      desired.Name,
          Namespace: desired.Namespace,
      }, existing)
      
      if err != nil && apierrors.IsNotFound(err) {
          // 創建新的 DaemonSet
          if err := r.Create(ctx, desired); err != nil {
              return fmt.Errorf("failed to create DaemonSet: %w", err)
          }
          return nil
      } else if err != nil {
          return err
      }
      
      // 更新現有 DaemonSet
      if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
          existing.Spec = desired.Spec
          if err := r.Update(ctx, existing); err != nil {
              return fmt.Errorf("failed to update DaemonSet: %w", err)
          }
      }
      
      return nil
  }
  ```

- [ ] 實現 DaemonSet 構造函數
  ```go
  func (r *GTP5GModuleReconciler) constructDaemonSet(
      module *operatorv1alpha1.GTP5GModule,
  ) *appsv1.DaemonSet {
      labels := map[string]string{
          "app":                          "gtp5g-installer",
          "gtp5g.gthulhu.io/module-name": module.Name,
      }
      
      return &appsv1.DaemonSet{
          ObjectMeta: metav1.ObjectMeta{
              Name:      fmt.Sprintf("gtp5g-installer-%s", module.Name),
              Namespace: module.Namespace,
              OwnerReferences: []metav1.OwnerReference{
                  *metav1.NewControllerRef(module, operatorv1alpha1.GroupVersion.WithKind("GTP5GModule")),
              },
          },
          Spec: appsv1.DaemonSetSpec{
              Selector: &metav1.LabelSelector{
                  MatchLabels: labels,
              },
              Template: corev1.PodTemplateSpec{
                  ObjectMeta: metav1.ObjectMeta{
                      Labels: labels,
                  },
                  Spec: corev1.PodSpec{
                      HostPID:            true,
                      ServiceAccountName: "gtp5g-operator",
                      Containers: []corev1.Container{
                          {
                              Name:  "installer",
                              Image: module.Spec.Image,
                              SecurityContext: &corev1.SecurityContext{
                                  Privileged: pointer.Bool(true),
                                  Capabilities: &corev1.Capabilities{
                                      Add: []corev1.Capability{
                                          "SYS_ADMIN",
                                          "SYS_MODULE",
                                      },
                                  },
                              },
                              Env: []corev1.EnvVar{
                                  {
                                      Name:  "GTP5G_VERSION",
                                      Value: module.Spec.Version,
                                  },
                                  {
                                      Name:  "KERNEL_VERSION",
                                      Value: module.Spec.KernelVersion,
                                  },
                              },
                              VolumeMounts: []corev1.VolumeMount{
                                  {
                                      Name:      "lib-modules",
                                      MountPath: "/lib/modules",
                                      ReadOnly:  true,
                                  },
                                  {
                                      Name:      "usr-src",
                                      MountPath: "/usr/src",
                                  },
                              },
                          },
                      },
                      Volumes: []corev1.Volume{
                          {
                              Name: "lib-modules",
                              VolumeSource: corev1.VolumeSource{
                                  HostPath: &corev1.HostPathVolumeSource{
                                      Path: "/lib/modules",
                                  },
                              },
                          },
                          {
                              Name: "usr-src",
                              VolumeSource: corev1.VolumeSource{
                                  HostPath: &corev1.HostPathVolumeSource{
                                      Path: "/usr/src",
                                  },
                              },
                          },
                      },
                      NodeSelector: module.Spec.NodeSelector,
                  },
              },
          },
      }
  }
  ```

- [ ] 實現狀態更新邏輯
  ```go
  func (r *GTP5GModuleReconciler) updateStatus(
      ctx context.Context,
      module *operatorv1alpha1.GTP5GModule,
  ) error {
      // 獲取 DaemonSet
      ds := &appsv1.DaemonSet{}
      err := r.Get(ctx, types.NamespacedName{
          Name:      fmt.Sprintf("gtp5g-installer-%s", module.Name),
          Namespace: module.Namespace,
      }, ds)
      if err != nil {
          return err
      }
      
      // 更新狀態
      newStatus := module.Status.DeepCopy()
      
      if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
          newStatus.Phase = operatorv1alpha1.ModulePhaseInstalled
          newStatus.Message = "All nodes have gtp5g installed"
      } else if ds.Status.NumberReady > 0 {
          newStatus.Phase = operatorv1alpha1.ModulePhaseInstalling
          newStatus.Message = fmt.Sprintf("%d/%d nodes ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
      } else {
          newStatus.Phase = operatorv1alpha1.ModulePhasePending
          newStatus.Message = "Waiting for installer pods"
      }
      
      newStatus.LastUpdateTime = metav1.Now()
      
      if !equality.Semantic.DeepEqual(&module.Status, newStatus) {
          module.Status = *newStatus
          if err := r.Status().Update(ctx, module); err != nil {
              return err
          }
      }
      
      return nil
  }
  ```

- [ ] 實現刪除處理
  ```go
  func (r *GTP5GModuleReconciler) handleDeletion(
      ctx context.Context,
      module *operatorv1alpha1.GTP5GModule,
  ) (ctrl.Result, error) {
      if containsString(module.Finalizers, finalizerName) {
          // 清理資源
          // ...
          
          // 移除 finalizer
          module.Finalizers = removeString(module.Finalizers, finalizerName)
          if err := r.Update(ctx, module); err != nil {
              return ctrl.Result{}, err
          }
      }
      return ctrl.Result{}, nil
  }
  ```

- [ ] 運行測試確認通過
  ```bash
  make test
  ```

#### 完成標準
- [ ] Reconcile 邏輯完整實現
- [ ] DaemonSet 創建/更新/刪除正常
- [ ] 狀態更新正確
- [ ] 所有測試通過

**完成日期**: ____________  

**Commit**: 
```bash
git add controllers/
git commit -m "feat(operator): implement GTP5GModule controller (GREEN phase)

- Implement reconcile loop
- Add DaemonSet creation and update logic
- Add status tracking
- Add finalizer for cleanup

All tests passing.

Part of #11"
```

**Commit Hash**: ____________

---

### 3.3 TDD Refactor Phase

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 提取常用函數到 helper
- [ ] 優化錯誤處理
- [ ] 改進日誌記錄
- [ ] 代碼審查和清理
- [ ] 運行 linter
  ```bash
  make lint
  ```
- [ ] 確保測試仍通過
  ```bash
  make test
  ```

**完成日期**: ____________  

**Commit**: 
```bash
git commit -am "refactor(operator): optimize controller code (REFACTOR phase)

- Extract helper functions
- Improve error handling
- Add structured logging

Part of #11"
```

**Commit Hash**: ____________

---

### 3.4 Docker 測試 - Controller 運行

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 構建 controller 鏡像
  ```bash
  cd operators/gtp5g-operator
  make docker-build IMG=localhost:5000/gtp5g-operator:test
  docker push localhost:5000/gtp5g-operator:test
  ```

- [ ] 安裝 CRD
  ```bash
  make install
  ```

- [ ] 部署 controller 到 kind 集群
  ```bash
  make deploy IMG=localhost:5000/gtp5g-operator:test
  ```

- [ ] 驗證 controller 運行
  ```bash
  kubectl get pods -n gtp5g-operator-system
  kubectl logs -n gtp5g-operator-system -l control-plane=controller-manager -f
  ```

- [ ] 創建測試 GTP5GModule
  ```bash
  kubectl apply -f config/samples/operator_v1alpha1_gtp5gmodule.yaml
  ```

- [ ] 觀察 reconciliation
  ```bash
  kubectl get gtp5gmodule -w
  kubectl get daemonset
  ```

- [ ] 驗證狀態更新
  ```bash
  kubectl describe gtp5gmodule sample-gtp5g
  ```

- [ ] 清理
  ```bash
  kubectl delete -f config/samples/operator_v1alpha1_gtp5gmodule.yaml
  make undeploy
  ```

#### 完成標準
- [ ] Controller 成功部署
- [ ] DaemonSet 自動創建
- [ ] 狀態正確更新
- [ ] 無錯誤日誌

**完成日期**: ____________  
**測試日誌**: 
```
# 記錄關鍵日誌


```

---

### ✅ 階段 3 完成檢查清單

- [ ] TDD Red/Green/Refactor 循環完成
- [ ] Controller 完整實現
- [ ] Docker 測試通過
- [ ] 所有更改已 commit

**階段完成日期**: ____________  
**總耗時**: ____________  

---

## 階段 4: Installer 容器開發

**目標**: 開發 gtp5g 內核模塊安裝器容器  
**預計時間**: 3-4 天  
**狀態**: ⬜ 未開始

### 4.1 創建 Installer 項目結構

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建目錄
  ```bash
  mkdir -p operators/gtp5g-operator/installer
  cd operators/gtp5g-operator/installer
  ```

- [ ] 創建 Go module
  ```bash
  go mod init github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/installer
  ```

- [ ] 創建目錄結構
  ```bash
  mkdir -p cmd/installer
  mkdir -p pkg/{detector,builder,loader,monitor}
  mkdir -p test
  ```

**完成日期**: ____________

---

### 4.2 TDD - 內核版本檢測器

**狀態**: ⬜ 未開始

#### Red Phase - 編寫測試
- [ ] 創建 `pkg/detector/kernel_test.go`
  ```go
  package detector_test
  
  import (
      "testing"
      "github.com/stretchr/testify/assert"
      "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/installer/pkg/detector"
  )
  
  func TestDetectKernelVersion(t *testing.T) {
      version, err := detector.DetectKernelVersion()
      assert.NoError(t, err)
      assert.NotEmpty(t, version)
      assert.Regexp(t, `^\d+\.\d+\.\d+`, version)
  }
  
  func TestIsKernelSupported(t *testing.T) {
      tests := []struct {
          name      string
          version   string
          supported bool
      }{
          {"Valid 5.4", "5.4.0-100-generic", true},
          {"Valid 5.15", "5.15.0-56-generic", true},
          {"Too old", "4.15.0-20-generic", false},
      }
      
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              result := detector.IsKernelSupported(tt.version)
              assert.Equal(t, tt.supported, result)
          })
      }
  }
  ```

- [ ] 運行測試（預期失敗）
  ```bash
  go test ./pkg/detector/...
  ```

#### Green Phase - 實現
- [ ] 創建 `pkg/detector/kernel.go`
  ```go
  package detector
  
  import (
      "fmt"
      "os/exec"
      "strings"
      "github.com/Masterminds/semver/v3"
  )
  
  var minKernelVersion = semver.MustParse("5.0.0")
  
  func DetectKernelVersion() (string, error) {
      cmd := exec.Command("uname", "-r")
      output, err := cmd.Output()
      if err != nil {
          return "", fmt.Errorf("failed to detect kernel version: %w", err)
      }
      return strings.TrimSpace(string(output)), nil
  }
  
  func IsKernelSupported(version string) bool {
      // 解析版本號（去除後綴如 -generic）
      parts := strings.Split(version, "-")
      v, err := semver.NewVersion(parts[0])
      if err != nil {
          return false
      }
      return v.Compare(minKernelVersion) >= 0
  }
  ```

- [ ] 運行測試（預期通過）
  ```bash
  go test ./pkg/detector/...
  ```

**完成日期**: ____________  

**Commit**: 
```bash
git add installer/pkg/detector/
git commit -m "feat(installer): add kernel version detector

- Implement kernel version detection
- Add kernel compatibility check
- Add comprehensive tests

Part of #11"
```

**Commit Hash**: ____________

---

### 4.3 TDD - gtp5g 構建器

**狀態**: ⬜ 未開始

#### Red Phase - 編寫測試
- [ ] 創建 `pkg/builder/gtp5g_test.go`
  ```go
  package builder_test
  
  import (
      "testing"
      "github.com/stretchr/testify/assert"
  )
  
  func TestCloneGTP5G(t *testing.T) {
      b := builder.NewBuilder()
      err := b.Clone("v0.8.3", "/tmp/gtp5g-test")
      assert.NoError(t, err)
      // 驗證目錄存在
  }
  
  func TestBuildModule(t *testing.T) {
      // TODO: 實現
  }
  ```

#### Green Phase - 實現
- [ ] 創建 `pkg/builder/gtp5g.go`
  ```go
  package builder
  
  import (
      "fmt"
      "os"
      "os/exec"
  )
  
  type Builder struct {
      workDir string
  }
  
  func NewBuilder() *Builder {
      return &Builder{
          workDir: "/tmp/gtp5g",
      }
  }
  
  func (b *Builder) Clone(version, dest string) error {
      // 如果目錄已存在，先刪除
      if _, err := os.Stat(dest); err == nil {
          os.RemoveAll(dest)
      }
      
      cmd := exec.Command("git", "clone",
          "-b", version,
          "--depth", "1",
          "https://github.com/free5gc/gtp5g.git",
          dest)
      
      output, err := cmd.CombinedOutput()
      if err != nil {
          return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
      }
      
      b.workDir = dest
      return nil
  }
  
  func (b *Builder) Build(kernelVersion string) error {
      cmd := exec.Command("make")
      cmd.Dir = b.workDir
      if kernelVersion != "" {
          cmd.Env = append(os.Environ(), fmt.Sprintf("KVER=%s", kernelVersion))
      }
      
      output, err := cmd.CombinedOutput()
      if err != nil {
          return fmt.Errorf("make failed: %w\nOutput: %s", err, output)
      }
      return nil
  }
  
  func (b *Builder) Install() error {
      cmd := exec.Command("make", "install")
      cmd.Dir = b.workDir
      
      output, err := cmd.CombinedOutput()
      if err != nil {
          return fmt.Errorf("make install failed: %w\nOutput: %s", err, output)
      }
      return nil
  }
  ```

**完成日期**: ____________  

**Commit**: 
```bash
git add installer/pkg/builder/
git commit -m "feat(installer): add gtp5g module builder

- Implement git clone with version
- Implement kernel module compilation
- Add installation logic

Part of #11"
```

**Commit Hash**: ____________

---

### 4.4 TDD - 模塊加載器

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 編寫測試 `pkg/loader/loader_test.go`
- [ ] 實現 `pkg/loader/loader.go`
  ```go
  func LoadModule(name string) error
  func UnloadModule(name string) error
  func IsModuleLoaded(name string) (bool, error)
  ```
- [ ] 測試通過

**完成日期**: ____________  
**Commit Hash**: ____________

---

### 4.5 主程序實現

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建 `cmd/installer/main.go`
  ```go
  package main
  
  import (
      "context"
      "log"
      "os"
      "os/signal"
      "syscall"
      "time"
      
      "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/installer/pkg/detector"
      "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/installer/pkg/builder"
      "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/installer/pkg/loader"
      "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/installer/pkg/monitor"
  )
  
  func main() {
      log.Println("GTP5G Installer starting...")
      
      // 獲取環境變量
      version := getEnv("GTP5G_VERSION", "v0.8.3")
      kernelVersion := os.Getenv("KERNEL_VERSION")
      
      // 檢測內核版本
      if kernelVersion == "" {
          detected, err := detector.DetectKernelVersion()
          if err != nil {
              log.Fatalf("Failed to detect kernel version: %v", err)
          }
          kernelVersion = detected
      }
      
      log.Printf("Kernel version: %s", kernelVersion)
      
      if !detector.IsKernelSupported(kernelVersion) {
          log.Fatalf("Kernel version %s is not supported", kernelVersion)
      }
      
      // 檢查模塊是否已加載
      loaded, err := loader.IsModuleLoaded("gtp5g")
      if err != nil {
          log.Printf("Warning: failed to check module status: %v", err)
      }
      
      if !loaded {
          // 構建並安裝模塊
          if err := installModule(version, kernelVersion); err != nil {
              log.Fatalf("Failed to install module: %v", err)
          }
      } else {
          log.Println("gtp5g module already loaded")
      }
      
      // 啟動監控器
      ctx, cancel := context.WithCancel(context.Background())
      defer cancel()
      
      go monitor.Start(ctx)
      
      // 等待信號
      sigChan := make(chan os.Signal, 1)
      signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
      <-sigChan
      
      log.Println("Shutting down...")
  }
  
  func installModule(version, kernelVersion string) error {
      log.Printf("Installing gtp5g %s for kernel %s", version, kernelVersion)
      
      b := builder.NewBuilder()
      
      // 克隆
      log.Println("Cloning gtp5g repository...")
      if err := b.Clone(version, "/tmp/gtp5g"); err != nil {
          return err
      }
      
      // 構建
      log.Println("Building gtp5g module...")
      if err := b.Build(kernelVersion); err != nil {
          return err
      }
      
      // 安裝
      log.Println("Installing gtp5g module...")
      if err := b.Install(); err != nil {
          return err
      }
      
      // 加載
      log.Println("Loading gtp5g module...")
      if err := loader.LoadModule("gtp5g"); err != nil {
          return err
      }
      
      log.Println("gtp5g module installed successfully")
      return nil
  }
  
  func getEnv(key, defaultValue string) string {
      if value := os.Getenv(key); value != "" {
          return value
      }
      return defaultValue
  }
  ```

**完成日期**: ____________  

---

### 4.6 Dockerfile 創建

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建 `installer/Dockerfile`
  ```dockerfile
  FROM ubuntu:22.04
  
  # 安裝構建依賴
  RUN apt-get update && apt-get install -y \
      build-essential \
      git \
      linux-headers-generic \
      kmod \
      ca-certificates \
      && rm -rf /var/lib/apt/lists/*
  
  # 複製編譯好的二進制
  COPY bin/installer /usr/local/bin/installer
  
  # 設置工作目錄
  WORKDIR /workspace
  
  # 運行 installer
  ENTRYPOINT ["/usr/local/bin/installer"]
  ```

- [ ] 創建構建腳本 `installer/build.sh`
  ```bash
  #!/bin/bash
  set -e
  
  # 構建 Go 二進制
  CGO_ENABLED=0 go build -o bin/installer ./cmd/installer
  
  # 構建 Docker 鏡像
  docker build -t localhost:5000/gtp5g-installer:latest .
  
  # 推送到本地 registry
  docker push localhost:5000/gtp5g-installer:latest
  ```

**完成日期**: ____________  

**Commit**: 
```bash
git add installer/
git commit -m "feat(installer): add main program and Dockerfile

- Implement installer main logic
- Add Dockerfile for container build
- Add build script

Part of #11"
```

**Commit Hash**: ____________

---

### 4.7 Docker 測試 - Installer 容器

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 構建鏡像
  ```bash
  cd installer
  chmod +x build.sh
  ./build.sh
  ```

- [ ] 在 kind 節點上測試
  ```bash
  # 獲取 kind 節點容器
  docker exec -it gtp5g-test-worker bash
  
  # 在節點內運行 installer
  docker run --rm \
    --privileged \
    -v /lib/modules:/lib/modules:ro \
    -v /usr/src:/usr/src \
    -e GTP5G_VERSION=v0.8.3 \
    localhost:5000/gtp5g-installer:latest
  ```

- [ ] 驗證模塊已加載
  ```bash
  lsmod | grep gtp5g
  ```

- [ ] 清理
  ```bash
  rmmod gtp5g
  ```

**完成日期**: ____________  
**測試輸出**: 
```


```

---

### ✅ 階段 4 完成檢查清單

- [ ] Installer 所有組件實現完成
- [ ] Docker 鏡像構建成功
- [ ] 容器測試通過
- [ ] 所有更改已 commit

**階段完成日期**: ____________  
**總耗時**: ____________  

---

## 階段 5: Helm Chart 整合

**目標**: 將 gtp5g-operator 整合到 Gthulhu Helm Chart  
**預計時間**: 2-3 天  
**狀態**: ⬜ 未開始

### 5.1 更新 Chart Values

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 備份原始 values.yaml
  ```bash
  cp chart/gthulhu/values.yaml chart/gthulhu/values.yaml.backup
  ```

- [ ] 添加 gtp5g-operator 配置到 `chart/gthulhu/values.yaml`
  ```yaml
  # GTP5G Operator Configuration
  gtp5gOperator:
    # 是否啟用 gtp5g operator
    enabled: false
    
    # Operator 鏡像配置
    operator:
      image:
        repository: localhost:5000/gtp5g-operator
        tag: "latest"
        pullPolicy: Always
      
      resources:
        limits:
          cpu: 200m
          memory: 256Mi
        requests:
          cpu: 100m
          memory: 128Mi
    
    # Installer 鏡像配置
    installer:
      image:
        repository: localhost:5000/gtp5g-installer
        tag: "latest"
        pullPolicy: Always
    
    # gtp5g 模塊配置
    module:
      # gtp5g 版本
      version: "v0.8.3"
      
      # 目標節點選擇器
      nodeSelector:
        gtp5g.gthulhu.io/enabled: "true"
      
      # 內核版本（留空則自動檢測）
      kernelVersion: ""
  
    # RBAC 配置
    rbac:
      create: true
    
    # ServiceAccount 配置
    serviceAccount:
      create: true
      name: "gtp5g-operator"
      annotations: {}
  ```

**完成日期**: ____________  

---

### 5.2 創建 Operator Deployment 模板

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建 `chart/gthulhu/templates/gtp5g-operator-deployment.yaml`
  ```yaml
  {{- if .Values.gtp5gOperator.enabled }}
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: {{ include "gthulhu.fullname" . }}-gtp5g-operator
    namespace: {{ .Release.Namespace }}
    labels:
      {{- include "gthulhu.labels" . | nindent 4 }}
      app.kubernetes.io/component: gtp5g-operator
  spec:
    replicas: 1
    selector:
      matchLabels:
        {{- include "gthulhu.selectorLabels" . | nindent 8 }}
        app.kubernetes.io/component: gtp5g-operator
    template:
      metadata:
        labels:
          {{- include "gthulhu.selectorLabels" . | nindent 10 }}
          app.kubernetes.io/component: gtp5g-operator
      spec:
        serviceAccountName: {{ .Values.gtp5gOperator.serviceAccount.name }}
        containers:
        - name: manager
          image: "{{ .Values.gtp5gOperator.operator.image.repository }}:{{ .Values.gtp5gOperator.operator.image.tag }}"
          imagePullPolicy: {{ .Values.gtp5gOperator.operator.image.pullPolicy }}
          command:
          - /manager
          args:
          - --leader-elect
          env:
          - name: INSTALLER_IMAGE
            value: "{{ .Values.gtp5gOperator.installer.image.repository }}:{{ .Values.gtp5gOperator.installer.image.tag }}"
          - name: GTP5G_VERSION
            value: {{ .Values.gtp5gOperator.module.version }}
          - name: KERNEL_VERSION
            value: {{ .Values.gtp5gOperator.module.kernelVersion }}
          resources:
            {{- toYaml .Values.gtp5gOperator.operator.resources | nindent 12 }}
  {{- end }}
  ```

**完成日期**: ____________  

---

### 5.3 創建 RBAC 模板

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建 `chart/gthulhu/templates/gtp5g-operator-rbac.yaml`
  ```yaml
  {{- if and .Values.gtp5gOperator.enabled .Values.gtp5gOperator.rbac.create }}
  ---
  apiVersion: v1
  kind: ServiceAccount
  metadata:
    name: {{ .Values.gtp5gOperator.serviceAccount.name }}
    namespace: {{ .Release.Namespace }}
    {{- with .Values.gtp5gOperator.serviceAccount.annotations }}
    annotations:
      {{- toYaml . | nindent 4 }}
    {{- end }}
  ---
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: {{ include "gthulhu.fullname" . }}-gtp5g-operator
  rules:
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["daemonsets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["operator.gthulhu.io"]
    resources: ["gtp5gmodules"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["operator.gthulhu.io"]
    resources: ["gtp5gmodules/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["operator.gthulhu.io"]
    resources: ["gtp5gmodules/finalizers"]
    verbs: ["update"]
  ---
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  metadata:
    name: {{ include "gthulhu.fullname" . }}-gtp5g-operator
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: {{ include "gthulhu.fullname" . }}-gtp5g-operator
  subjects:
  - kind: ServiceAccount
    name: {{ .Values.gtp5gOperator.serviceAccount.name }}
    namespace: {{ .Release.Namespace }}
  {{- end }}
  ```

**完成日期**: ____________  

---

### 5.4 創建 CRD 模板

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 複製生成的 CRD
  ```bash
  cp operators/gtp5g-operator/config/crd/bases/operator.gthulhu.io_gtp5gmodules.yaml \
     chart/gthulhu/templates/gtp5g-crd.yaml
  ```

- [ ] 添加 Helm 條件
  ```yaml
  {{- if .Values.gtp5gOperator.enabled }}
  # CRD 內容
  {{- end }}
  ```

**完成日期**: ____________  

---

### 5.5 Helm Chart 測試

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] Lint chart
  ```bash
  cd chart/gthulhu
  helm lint .
  ```

- [ ] 模板渲染測試
  ```bash
  helm template gthulhu . \
    --set gtp5gOperator.enabled=true \
    --debug
  ```

- [ ] 安裝到測試集群
  ```bash
  helm install gthulhu . \
    --set gtp5gOperator.enabled=true \
    --namespace gthulhu-system \
    --create-namespace
  ```

- [ ] 驗證部署
  ```bash
  kubectl get pods -n gthulhu-system
  kubectl get crd
  kubectl logs -n gthulhu-system -l app.kubernetes.io/component=gtp5g-operator
  ```

- [ ] 創建測試 GTP5GModule
  ```bash
  # 標記節點
  kubectl label node gtp5g-test-worker gtp5g.gthulhu.io/enabled=true
  
  # 創建 CR
  cat <<EOF | kubectl apply -f -
  apiVersion: operator.gthulhu.io/v1alpha1
  kind: GTP5GModule
  metadata:
    name: test-module
  spec:
    version: v0.8.3
  EOF
  ```

- [ ] 觀察安裝過程
  ```bash
  kubectl get gtp5gmodule test-module -w
  kubectl get daemonset
  kubectl get pods -l app=gtp5g-installer
  ```

- [ ] 驗證安裝成功
  ```bash
  kubectl describe gtp5gmodule test-module
  # 期望 Status.Phase = Installed
  ```

- [ ] 清理
  ```bash
  helm uninstall gthulhu -n gthulhu-system
  ```

**完成日期**: ____________  
**測試結果**: 
```


```

**Commit**: 
```bash
git add chart/gthulhu/
git commit -m "feat(chart): integrate gtp5g-operator into Helm chart

- Add gtp5g-operator configuration to values.yaml
- Create operator deployment template
- Create RBAC templates
- Add CRD installation

Operator is disabled by default, can be enabled via values.

Part of #11"
```

**Commit Hash**: ____________

---

### ✅ 階段 5 完成檢查清單

- [ ] Helm Chart 更新完成
- [ ] Lint 無錯誤
- [ ] 測試安裝成功
- [ ] 所有更改已 commit

**階段完成日期**: ____________  
**總耗時**: ____________  

---

## 階段 6: E2E 測試與文檔

**目標**: 編寫端到端測試和完整文檔  
**預計時間**: 3-4 天  
**狀態**: ⬜ 未開始

### 6.1 E2E 測試套件

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建 E2E 測試目錄
  ```bash
  mkdir -p test/e2e
  cd test/e2e
  ```

- [ ] 編寫測試腳本 `test/e2e/gtp5g_operator_e2e_test.sh`
  ```bash
  #!/bin/bash
  set -e
  
  echo "========================================="
  echo "GTP5G Operator E2E Test"
  echo "========================================="
  
  # 清理函數
  cleanup() {
      echo "Cleaning up..."
      helm uninstall gthulhu -n gthulhu-system --ignore-not-found
      kubectl delete namespace gthulhu-system --ignore-not-found
      kind delete cluster --name gtp5g-e2e || true
  }
  trap cleanup EXIT
  
  # 1. 創建測試集群
  echo "Step 1: Creating kind cluster..."
  kind create cluster --name gtp5g-e2e --config ../../kind-config.yaml
  
  # 2. 安裝 Helm chart
  echo "Step 2: Installing Gthulhu with gtp5g-operator..."
  helm install gthulhu ../../chart/gthulhu \
    --set gtp5gOperator.enabled=true \
    --namespace gthulhu-system \
    --create-namespace \
    --wait
  
  # 3. 驗證 operator 運行
  echo "Step 3: Verifying operator is running..."
  kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/component=gtp5g-operator \
    -n gthulhu-system \
    --timeout=120s
  
  # 4. 標記節點
  echo "Step 4: Labeling nodes..."
  kubectl label node gtp5g-e2e-worker gtp5g.gthulhu.io/enabled=true
  
  # 5. 創建 GTP5GModule
  echo "Step 5: Creating GTP5GModule..."
  cat <<EOF | kubectl apply -f -
  apiVersion: operator.gthulhu.io/v1alpha1
  kind: GTP5GModule
  metadata:
    name: e2e-test
  spec:
    version: v0.8.3
  EOF
  
  # 6. 等待安裝完成
  echo "Step 6: Waiting for installation..."
  sleep 30
  kubectl wait --for=condition=ready pod \
    -l app=gtp5g-installer \
    --timeout=300s
  
  # 7. 驗證狀態
  echo "Step 7: Verifying GTP5GModule status..."
  kubectl get gtp5gmodule e2e-test -o yaml
  
  # 8. 驗證 DaemonSet
  echo "Step 8: Verifying DaemonSet..."
  kubectl get daemonset
  
  echo "========================================="
  echo "E2E Test PASSED!"
  echo "========================================="
  ```

- [ ] 使測試腳本可執行
  ```bash
  chmod +x test/e2e/gtp5g_operator_e2e_test.sh
  ```

- [ ] 運行 E2E 測試
  ```bash
  ./test/e2e/gtp5g_operator_e2e_test.sh
  ```

**完成日期**: ____________  
**測試結果**: ✅ PASS / ❌ FAIL  
**測試日誌**: 
```


```

**Commit**: 
```bash
git add test/e2e/
git commit -m "test(e2e): add end-to-end test for gtp5g-operator

- Create automated E2E test script
- Test complete installation flow
- Verify operator and installer functionality

Part of #11"
```

**Commit Hash**: ____________

---

### 6.2 用戶文檔

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 創建用戶指南 `docs/gtp5g-operator.md`
  ```markdown
  # GTP5G Operator 用戶指南
  
  ## 概述
  
  GTP5G Operator 自動化管理 gtp5g 內核模塊的安裝和生命週期...
  
  ## 前置要求
  
  - Kubernetes 1.19+
  - Linux 內核 5.0+
  - 節點需要有編譯工具鏈
  
  ## 快速開始
  
  ### 1. 安裝 Gthulhu (啟用 gtp5g-operator)
  
  \`\`\`bash
  helm install gthulhu gthulhu/gthulhu \\
    --set gtp5gOperator.enabled=true \\
    --namespace gthulhu-system \\
    --create-namespace
  \`\`\`
  
  ### 2. 標記需要安裝 gtp5g 的節點
  
  \`\`\`bash
  kubectl label node <node-name> gtp5g.gthulhu.io/enabled=true
  \`\`\`
  
  ### 3. 創建 GTP5GModule
  
  \`\`\`yaml
  apiVersion: operator.gthulhu.io/v1alpha1
  kind: GTP5GModule
  metadata:
    name: my-gtp5g
  spec:
    version: v0.8.3
  \`\`\`
  
  ## 配置選項
  
  ### Helm Values
  
  ...
  
  ### GTP5GModule Spec
  
  ...
  
  ## 故障排除
  
  ### 模塊編譯失敗
  
  ...
  
  ### 節點未安裝
  
  ...
  
  ## 與 free5gc 整合
  
  ...
  ```

- [ ] 創建 API 參考 `docs/gtp5g-operator-api.md`
- [ ] 創建架構文檔 `docs/gtp5g-operator-architecture.md`

**完成日期**: ____________  

---

### 6.3 更新主 README

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 在 `README.md` 添加 GTP5G Operator 章節
  ```markdown
  ## GTP5G Operator (New!)
  
  Gthulhu now includes an optional Kubernetes operator for managing
  gtp5g kernel modules, enabling seamless deployment of 5G UPF
  workloads.
  
  ### Quick Start
  
  \`\`\`bash
  # Install with gtp5g-operator enabled
  helm install gthulhu gthulhu/gthulhu \\
    --set gtp5gOperator.enabled=true
  
  # Label nodes
  kubectl label node worker1 gtp5g.gthulhu.io/enabled=true
  
  # Create GTP5GModule
  kubectl apply -f - <<EOF
  apiVersion: operator.gthulhu.io/v1alpha1
  kind: GTP5GModule
  metadata:
    name: upf-gtp5g
  spec:
    version: v0.8.3
  EOF
  \`\`\`
  
  See [GTP5G Operator Guide](docs/gtp5g-operator.md) for details.
  ```

**完成日期**: ____________  

**Commit**: 
```bash
git add docs/ README.md
git commit -m "docs(operator): add comprehensive documentation for gtp5g-operator

- Add user guide with quick start
- Add API reference documentation
- Add architecture documentation
- Update main README

Part of #11"
```

**Commit Hash**: ____________

---

### ✅ 階段 6 完成檢查清單

- [ ] E2E 測試通過
- [ ] 所有文檔已完成
- [ ] README 已更新
- [ ] 所有更改已 commit

**階段完成日期**: ____________  
**總耗時**: ____________  

---

## 階段 7: PR 提交與 Review

**目標**: 提交 Pull Request 並完成代碼審查  
**預計時間**: 2-3 天  
**狀態**: ⬜ 未開始

### 7.1 最終代碼審查

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 運行完整測試套件
  ```bash
  # 單元測試
  cd operators/gtp5g-operator
  make test
  
  # E2E 測試
  cd ../../test/e2e
  ./gtp5g_operator_e2e_test.sh
  ```

- [ ] 檢查測試覆蓋率
  ```bash
  cd operators/gtp5g-operator
  make test-coverage
  # 目標: >70%
  ```

- [ ] 運行 linter
  ```bash
  make lint
  # 無錯誤
  ```

- [ ] 檢查 Helm chart
  ```bash
  cd chart/gthulhu
  helm lint .
  ```

- [ ] 自我 code review
  - [ ] 所有代碼符合 Go 規範
  - [ ] 無冗餘註釋
  - [ ] 錯誤處理完整
  - [ ] 日誌記錄適當

**完成日期**: ____________  

---

### 7.2 準備 PR

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 合併最新的 upstream main
  ```bash
  git fetch upstream
  git rebase upstream/main
  ```

- [ ] 解決衝突（如有）

- [ ] Squash commits（可選）
  ```bash
  git rebase -i upstream/main
  # 合併相關的小 commits
  ```

- [ ] 推送到 fork
  ```bash
  git push origin feature/gtp5g-operator --force-with-lease
  ```

**完成日期**: ____________  

---

### 7.3 創建 Pull Request

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 在 GitHub 上創建 PR
  - Base: `Gthulhu/Gthulhu:main`
  - Compare: `YOUR_USERNAME/Gthulhu:feature/gtp5g-operator`

- [ ] 填寫 PR 描述
  ```markdown
  ## Description
  Implements a Kubernetes operator to automate gtp5g kernel module 
  management for 5G UPF workloads on Gthulhu clusters.
  
  ## Type of Change
  - [x] New feature
  - [ ] Bug fix
  - [ ] Performance improvement
  - [x] Documentation update
  - [ ] Code refactoring
  
  ## Motivation
  Simplifies deployment of free5GC on Kubernetes by automating gtp5g
  kernel module installation, making Gthulhu suitable for 5G edge
  computing scenarios.
  
  ## Implementation Details
  
  ### Architecture
  - **CRD**: `GTP5GModule` for declarative module management
  - **Controller**: Reconciles desired state via DaemonSet deployment
  - **Installer**: Container that compiles and loads gtp5g for target kernel
  - **Helm Integration**: Optional operator deployment via values.yaml
  
  ### Key Components
  1. **API (CRD)**: `operators/gtp5g-operator/api/v1alpha1/`
  2. **Controller**: `operators/gtp5g-operator/controllers/`
  3. **Installer**: `operators/gtp5g-operator/installer/`
  4. **Helm Chart**: `chart/gthulhu/templates/gtp5g-*`
  
  ## Testing
  
  ### Test Coverage
  - **Unit Tests**: 75% coverage
  - **Integration Tests**: Controller reconciliation
  - **E2E Tests**: Full installation flow in kind cluster
  
  ### Test Results
  \`\`\`
  === Unit Tests ===
  PASS: 45/45 tests
  Coverage: 75.3%
  
  === E2E Tests ===
  PASS: Complete installation flow
  Time: 5m23s
  \`\`\`
  
  ### Manual Testing
  - ✅ Tested on Ubuntu 20.04 with kernel 5.4
  - ✅ Tested on Ubuntu 22.04 with kernel 5.15
  - ✅ Tested with free5gc v3.3.0
  - ✅ Verified module loading and unloading
  
  ## Performance Impact
  - **Memory**: ~50MB per node for installer DaemonSet
  - **CPU**: Minimal (<5%) during compilation
  - **Installation Time**: <60s on typical nodes
  - **Scheduler Impact**: None (operator runs independently)
  
  ## Documentation
  - [x] User guide: `docs/gtp5g-operator.md`
  - [x] API reference: `docs/gtp5g-operator-api.md`
  - [x] Architecture: `docs/gtp5g-operator-architecture.md`
  - [x] README updated
  - [x] Inline code comments
  
  ## Backward Compatibility
  - ✅ Operator is **disabled by default**
  - ✅ No changes to existing Gthulhu scheduler
  - ✅ No changes to existing API server
  - ✅ Fully backward compatible
  
  ## Checklist
  - [x] Code follows gofmt style guidelines
  - [x] Commit messages follow semantic format
  - [x] Updated relevant documentation
  - [x] Changes are backward compatible
  - [x] All tests pass (`make test`)
  - [x] Linting passes (`make lint`)
  - [x] E2E tests pass
  - [x] Helm chart lints successfully
  
  ## Related Issues
  Closes #11
  
  ## Deployment Guide
  
  ### Enable in existing installation
  \`\`\`bash
  helm upgrade gthulhu gthulhu/gthulhu \\
    --set gtp5gOperator.enabled=true \\
    --reuse-values
  \`\`\`
  
  ### Fresh installation
  \`\`\`bash
  helm install gthulhu gthulhu/gthulhu \\
    --set gtp5gOperator.enabled=true \\
    --namespace gthulhu-system \\
    --create-namespace
  \`\`\`
  
  ## Future Enhancements
  - [ ] Multi-version support (different gtp5g versions per node)
  - [ ] Automatic upgrade mechanism
  - [ ] Prometheus metrics integration
  - [ ] Dashboard for module status
  
  ## Screenshots
  
  ### GTP5GModule Status
  \`\`\`bash
  $ kubectl get gtp5gmodule
  NAME        VERSION   PHASE       INSTALLED NODES   AGE
  upf-gtp5g   v0.8.3    Installed   2                 5m
  \`\`\`
  
  ### DaemonSet Status
  \`\`\`bash
  $ kubectl get ds
  NAME                     DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE
  gtp5g-installer-upf      2         2         2       2            2
  \`\`\`
  
  ## Additional Notes
  - Development followed TDD (Test-Driven Development)
  - All phases documented in `GTP5G_OPERATOR_DEVELOPMENT_PLAN.md`
  - Tested with Docker + kind for local development
  - Reviewed against contributing guidelines
  
  ## Reviewers
  @ianchen0119 @Gthulhu/maintainers
  ```

- [ ] 添加相關標籤
  - `enhancement`
  - `operator`
  - `5G`

**完成日期**: ____________  
**PR URL**: ____________

---

### 7.4 響應 Review 意見

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 監控 PR 狀態
- [ ] 及時響應 reviewer 意見
  - [ ] 回答問題
  - [ ] 修改代碼
  - [ ] 更新文檔
  - [ ] 添加測試

- [ ] 每次修改後更新 PR
  ```bash
  # 修改代碼
  git add .
  git commit -m "address review comments: <具體改動>"
  git push origin feature/gtp5g-operator
  ```

- [ ] 通過 CI/CD 檢查
  - [ ] 構建成功
  - [ ] 測試通過
  - [ ] Linting 通過

**Review 迭代記錄**: 
```
Iteration 1 (日期):
- 意見: 
- 修改: 
- Commit: 

Iteration 2 (日期):
- 意見: 
- 修改: 
- Commit: 
```

---

### 7.5 PR 合併

**狀態**: ⬜ 未開始

#### 任務清單
- [ ] 獲得 maintainer 批准
- [ ] 所有 CI 檢查通過
- [ ] 無衝突
- [ ] PR 被合併

**合併日期**: ____________  
**合併 Commit**: ____________

---

### ✅ 階段 7 完成檢查清單

- [ ] PR 已創建
- [ ] Review 意見已解決
- [ ] PR 已合併

**階段完成日期**: ____________  
**總耗時**: ____________  

---

## 📊 項目總結

### 整體進度

| 階段 | 計劃時間 | 實際時間 | 狀態 |
|------|----------|----------|------|
| 階段 0 | 1-2 天 | ______ | ____ |
| 階段 1 | 2-3 天 | ______ | ____ |
| 階段 2 | 2-3 天 | ______ | ____ |
| 階段 3 | 3-4 天 | ______ | ____ |
| 階段 4 | 3-4 天 | ______ | ____ |
| 階段 5 | 2-3 天 | ______ | ____ |
| 階段 6 | 3-4 天 | ______ | ____ |
| 階段 7 | 2-3 天 | ______ | ____ |
| **總計** | **6 週** | **______** | **____** |

### 交付成果

- [ ] GTP5GModule CRD
- [ ] GTP5G Operator Controller
- [ ] GTP5G Installer 容器
- [ ] Helm Chart 整合
- [ ] 完整測試套件（單元 + 集成 + E2E）
- [ ] 完整文檔（用戶指南 + API 參考 + 架構）
- [ ] 已合併的 Pull Request

### 測試覆蓋率

- **單元測試**: ______%
- **集成測試**: ______%
- **E2E 測試**: PASS / FAIL

### 學到的經驗

```
記錄開發過程中的重要經驗：

1. TDD 的實踐:


2. Kubernetes Operator 開發:


3. 內核模塊管理的挑戰:


4. Docker 測試的技巧:


```

### 未來改進方向

```
1. 


2. 


3. 


```

---

## 🔗 相關資源

- **Issue**: https://github.com/Gthulhu/Gthulhu/issues/11
- **PR**: ____________
- **文檔**: 
  - 用戶指南: `docs/gtp5g-operator.md`
  - API 參考: `docs/gtp5g-operator-api.md`
  - 架構文檔: `docs/gtp5g-operator-architecture.md`
- **測試**: 
  - 單元測試: `operators/gtp5g-operator/test/`
  - E2E 測試: `test/e2e/gtp5g_operator_e2e_test.sh`

---

**最後更新**: ____________  
**維護者**: ianchen0119  
**版本**: 1.0
