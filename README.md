# BSS Metrics API Server

This is an API Server implemented in Golang for receiving and processing BSS (BPF Scheduler Subsystem) metrics data and providing system information.

## DEMO

Click the image below to see our DEMO on YouTube!

[![IMAGE ALT TEXT HERE](./assets/preview.png)](https://www.youtube.com/watch?v=R4EmZ18P954)

## Features

- Receive BSS metrics data sent by clients
- Query pod-to-PID mappings from the system
- Provide RESTful API in JSON format
- Include health check endpoint
- Support CORS
- Request logging capability
- Error handling and validation
- Kubernetes integration for pod label information
- Configurable scheduling strategies based on pod labels

## API Endpoints

### 1. Submit Metrics Data
- **URL**: `/api/v1/metrics`
- **Method**: `POST`
- **Content-Type**: `application/json`

#### Request Format
```json
{
  "usersched_pid": 1234,
  "nr_queued": 10,
  "nr_scheduled": 5,
  "nr_running": 2,
  "nr_online_cpus": 8,
  "nr_user_dispatches": 100,
  "nr_kernel_dispatches": 50,
  "nr_cancel_dispatches": 2,
  "nr_bounce_dispatches": 1,
  "nr_failed_dispatches": 0,
  "nr_sched_congested": 3
}
```

#### Success Response
```json
{
  "success": true,
  "message": "Metrics received successfully",
  "timestamp": "2025-06-19T10:30:00Z"
}
```

#### Error Response
```json
{
  "success": false,
  "error": "Invalid JSON format: ..."
}
```

### 2. Get Pod-PID Mappings
- **URL**: `/api/v1/pods/pids`
- **Method**: `GET`

#### Response
```json
{
  "success": true,
  "message": "Pod-PID mappings retrieved successfully",
  "timestamp": "2025-06-25T13:50:21Z",
  "pods": [
    {
      "pod_name": "",
      "namespace": "",
      "pod_uid": "65979e01-4cb1-4d08-9dba-45530253ff00",
      "container_id": "5148a146ffbbe8672f11494843d54b8769d2eccc677c02027fc09aba192e3c67",
      "processes": [
        {
          "pid": 717720,
          "command": "pause",
          "ppid": 717576
        },
        {
          "pid": 718001,
          "command": "loki",
          "ppid": 717576
        }
      ]
    }
  ]
}
```

#### Error Response
```json
{
  "success": false,
  "error": "Failed to get pod-pid mappings: ..."
}
```

### 3. Get and Set Scheduling Strategies
- **URL**: `/api/v1/scheduling/strategies`
- **Methods**: `GET`, `POST`
- **Content-Type**: `application/json`

#### GET Response
```json
{
  "success": true,
  "message": "Scheduling strategies retrieved successfully",
  "timestamp": "2025-06-19T10:30:00Z",
  "scheduling": [
    {
      "priority": true,
      "execution_time": 20000000,
      "pid": 718001
    },
    {
      "priority": false,
      "execution_time": 10000000,
      "pid": 717720
    }
  ]
}
```

#### POST Request Format
```json
{
  "strategies": [
    {
      "priority": true,
      "execution_time": 20000000,
      "selectors": [
        {
          "key": "nf",
          "value": "upf"
        }
      ]
    },
    {
      "priority": false,
      "execution_time": 10000000,
      "pid": 717720
    }
  ]
}
```

#### POST Success Response
```json
{
  "success": true,
  "message": "Scheduling strategies saved successfully",
  "timestamp": "2025-06-19T10:30:00Z"
}
```

### 4. Health Check
- **URL**: `/health`
- **Method**: `GET`

#### Response
```json
{
  "status": "healthy",
  "timestamp": "2025-06-19T10:30:00Z",
  "service": "BSS Metrics API Server"
}
```

### 5. API Information
- **URL**: `/`
- **Method**: `GET`

## Quick Start

### 1. Install Dependencies
```bash
go mod tidy
```

### 2. Start Service
```bash
go run main.go
```

The service will start on `http://localhost:8080`.

### 3. Start Service with Kubernetes integration
```bash
# Use local Kubernetes config
go run main.go --kubeconfig=$HOME/.kube/config

# Run in-cluster (when deployed in Kubernetes)
go run main.go --in-cluster=true
```

### 4. Test API

#### Submit metrics data
```bash
curl -X POST http://localhost:8080/api/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{
    "usersched_pid": 1234,
    "nr_queued": 10,
    "nr_scheduled": 5,
    "nr_running": 2,
    "nr_online_cpus": 8,
    "nr_user_dispatches": 100,
    "nr_kernel_dispatches": 50,
    "nr_cancel_dispatches": 2,
    "nr_bounce_dispatches": 1,
    "nr_failed_dispatches": 0,
    "nr_sched_congested": 3
  }'
```

#### Check health status
```bash
curl http://localhost:8080/health
```

#### Query Pod-PID Mappings
```bash
# Get all pod-pid mappings
curl -X GET http://localhost:8080/api/v1/pods/pids

# Format output with jq for better readability
curl -s -X GET http://localhost:8080/api/v1/pods/pids | jq '.'

# Get only specific information (example: extract pod UIDs and process counts)
curl -s -X GET http://localhost:8080/api/v1/pods/pids | jq '.pods[] | {pod_uid: .pod_uid, process_count: (.processes | length)}'
```

#### Get scheduling strategies
```bash
curl -X GET http://localhost:8080/api/v1/scheduling/strategies
```

#### Set scheduling strategies
```bash
curl -X POST http://localhost:8080/api/v1/scheduling/strategies \
  -H "Content-Type: application/json" \
  -d '{
    "strategies": [
      {
        "priority": true,
        "execution_time": 20000000,
        "selectors": [
          {
            "key": "nf",
            "value": "upf"
          }
        ]
      },
      {
        "priority": false,
        "execution_time": 10000000,
        "pid": 717720
      }
    ]
  }'
```

## Data Structure Description

### BssData Structure
| Field | Type | Description |
|-------|------|-------------|
| `usersched_pid` | uint32 | PID of the userspace scheduler |
| `nr_queued` | uint64 | Number of tasks queued in the userspace scheduler |
| `nr_scheduled` | uint64 | Number of tasks scheduled by the userspace scheduler |
| `nr_running` | uint64 | Number of tasks currently running in the userspace scheduler |
| `nr_online_cpus` | uint64 | Number of online CPUs in the system |
| `nr_user_dispatches` | uint64 | Number of userspace dispatches |
| `nr_kernel_dispatches` | uint64 | Number of kernel space dispatches |
| `nr_cancel_dispatches` | uint64 | Number of cancelled dispatches |
| `nr_bounce_dispatches` | uint64 | Number of bounce dispatches |
| `nr_failed_dispatches` | uint64 | Number of failed dispatches |
| `nr_sched_congested` | uint64 | Number of scheduler congestion occurrences |

### PodInfo Structure
| Field | Type | Description |
|-------|------|-------------|
| `pod_name` | string | Name of the pod (currently empty, extracted from metadata) |
| `namespace` | string | Namespace of the pod (currently empty, extracted from metadata) |
| `pod_uid` | string | Unique identifier of the pod |
| `container_id` | string | Container ID within the pod |
| `processes` | []PodProcess | List of processes running in the pod |

### PodProcess Structure
| Field | Type | Description |
|-------|------|-------------|
| `pid` | int | Process ID |
| `command` | string | Command name of the process |
| `ppid` | int | Parent Process ID (optional) |

### SchedulingStrategy Structure
| Field | Type | Description |
|-------|------|-------------|
| `priority` | bool | Indicates if this is a high-priority strategy |
| `execution_time` | uint64 | Desired execution time in nanoseconds |
| `pid` | uint32 | Optional specific PID to apply this strategy |
| `selectors` | []LabelSelector | Optional selectors to match pods for this strategy |

### LabelSelector Structure
| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Label key to match |
| `value` | string | Expected value of the label |

## Development and Extension

### Suggested New Features
1. **Data Persistence**: Store received metrics to database (such as PostgreSQL, MongoDB)
2. **Data Analytics**: Add statistical and analytical features
3. **Alert System**: Set up alert rules based on metrics values
4. **Authentication & Authorization**: Add API key or JWT authentication
5. **Batch Processing**: Support batch submission of multiple metrics data
6. **Monitoring Dashboard**: Build web interface to visualize metrics data

### Architecture Description
- Uses Gorilla Mux for routing
- Includes middleware for CORS and logging
- Structured error handling and response format
- Timestamps use RFC3339 format
- Kubernetes client integration with caching
- Support for both in-cluster and out-of-cluster operation

## Kubernetes 整合

此 API 伺服器可以與 Kubernetes 整合，以獲取真實的 Pod 標籤資訊。它支援兩種運行模式：

### 在 Kubernetes 集群內運行

當在 Kubernetes 集群內部署時，API 伺服器會自動使用 ServiceAccount 連接到 Kubernetes API。完整的部署清單可在 `k8s/deployment.yaml` 中找到，其中包含：

- 具備必要權限的 ServiceAccount
- 具有健康檢查的 Deployment
- 用於公開 API 的 Service

部署命令：
```bash
kubectl apply -f k8s/deployment.yaml
```

### 在集群外運行

當在 Kubernetes 集群外運行時，API 伺服器會嘗試使用 kubeconfig 文件連接到 Kubernetes API。預設情況下，它會使用 `~/.kube/config` 路徑，您也可以通過設置 `KUBECONFIG` 環境變數或使用 `--kubeconfig` 參數來指定不同的路徑：

```bash
export KUBECONFIG=/path/to/your/kubeconfig
go run main.go

# 或者
go run main.go --kubeconfig=/path/to/your/kubeconfig
```

### 實時 Pod 標籤更新

API 伺服器會定期刷新其 Pod 標籤緩存，以確保即使在 Pod 標籤變更時，調度策略也能正確應用。

### 降級到模擬數據

如果無法連接到 Kubernetes API，系統會自動降級使用模擬數據。這對於開發和測試環境很有用。

### Docker 映像建立

要建立 Docker 映像：
```bash
docker build -t bss-metrics-api:latest .
```

## License
This project is open source.
