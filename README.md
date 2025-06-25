# BSS Metrics API Server

This is an API Server implemented in Golang for receiving and processing BSS (BPF Scheduler Subsystem) metrics data and providing system information.

## Features

- Receive BSS metrics data sent by clients
- Query pod-to-PID mappings from the system
- Provide RESTful API in JSON format
- Include health check endpoint
- Support CORS
- Request logging capability
- Error handling and validation

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

### 3. Health Check
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

### 4. API Information
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

### 3. Test API

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

## License
This project is open source.
