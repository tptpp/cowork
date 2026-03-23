# Cowork Testing Guide

## Prerequisites

- Go 1.21+
- Node.js 18+ (for frontend)
- SQLite3

## Quick Start

### 1. Create Configuration File

```bash
mkdir -p ~/.cowork
cat > ~/.cowork/setting.json << 'EOF'
{
  "worker": {
    "test-worker": {
      "ai_base_url": "https://coding.dashscope.aliyuncs.com/v1",
      "ai_model": "glm-5",
      "ai_api_key": "your-api-key",
      "tags": ["general", "test"],
      "description": "测试工作节点"
    }
  },
  "coordinator": {
    "ai_base_url": "https://coding.dashscope.aliyuncs.com/v1",
    "ai_model": "glm-5",
    "ai_api_key": "your-api-key",
    "scheduler": {
      "poll_interval": "2s",
      "worker_timeout": "30s",
      "max_retry_attempts": 3,
      "task_timeout": "30m"
    }
  }
}
EOF
```

### 2. Start Coordinator

```bash
./bin/coordinator
```

The coordinator will:
- Start HTTP server on port 8080 (or `COWORK_ADDR` env var)
- Create SQLite database at `~/.cowork/coordinator/cowork.db`
- Initialize WebSocket hub for real-time updates

### 3. Start Worker (New Terminal)

```bash
./bin/worker --name test-worker
```

The worker will:
- Register with the coordinator
- Start heartbeat loop
- Create workspace at `~/.cowork/workers/test-worker/workspace`

### 4. Verify System Status

Check workers:
```bash
curl http://localhost:8080/api/workers
```

Expected response:
```json
{
  "success": true,
  "data": [
    {
      "id": "...",
      "name": "test-worker",
      "status": "idle",
      "tags": ["general", "test"]
    }
  ]
}
```

## Testing Agent Sessions

### 1. Create Agent Session

```bash
curl -X POST http://localhost:8080/api/agent/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai",
    "system_prompt": "You are a helpful assistant."
  }'
```

Response includes `session_id`.

### 2. Send Message with Function Calling

```bash
curl -X POST http://localhost:8080/api/agent/sessions/{session_id}/messages/tools \
  -H "Content-Type: application/json" \
  -d '{
    "content": "List files in /tmp directory"
  }'
```

### 3. Check Task Status

```bash
curl http://localhost:8080/api/tasks
```

## Testing Task Dependencies

### 1. Create Task with Dependencies

First, create the parent task:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "shell",
    "description": "Create a file",
    "input": {"command": "echo hello > /tmp/test.txt"},
    "priority": "high"
  }'
```

Then create a dependent task with the parent task ID:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "shell",
    "description": "Read the file",
    "input": {"command": "cat /tmp/test.txt"},
    "depends_on": ["{parent_task_id}"]
  }'
```

## Testing Task Retry

Create a task that will fail:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "shell",
    "description": "This will fail",
    "input": {"command": "exit 1"},
    "max_retries": 3,
    "retry_on_failure": true
  }'
```

The scheduler will automatically retry the task up to 3 times.

## Testing Task Timeout

Create a long-running task with timeout:
```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "shell",
    "description": "Sleep task",
    "input": {"command": "sleep 60"},
    "timeout": 5
  }'
```

The task will be marked as failed after 5 seconds.

## Frontend Testing

### 1. Install Dependencies

```bash
cd web
npm install
```

### 2. Start Development Server

```bash
npm run dev
```

### 3. Access Dashboard

Open http://localhost:5173 in browser.

### 4. Add Agent Chat Widget

1. Click "Add Widget" button
2. Select "Agent Chat"
3. Drag to position
4. Start chatting with the AI

## End-to-End Test Scenarios

### Scenario 1: Basic Task Execution

1. Start coordinator and worker
2. Create a shell task via API
3. Verify task is assigned to worker
4. Verify task completes with output

### Scenario 2: Multi-Worker Load Balancing

1. Start coordinator
2. Start multiple workers with same tags
3. Create multiple tasks
4. Verify tasks are distributed across workers

### Scenario 3: Worker Offline Rescheduling

1. Start coordinator
2. Start worker, create a task
3. Kill worker process
4. Verify task is rescheduled when worker goes offline
5. Restart worker, verify task is reassigned

### Scenario 4: Task Dependencies (DAG)

1. Create task A (creates file)
2. Create task B (reads file) with dependency on A
3. Verify B waits for A to complete
4. Verify B starts only after A succeeds

### Scenario 5: Task Retry

1. Create task with `max_retries: 3`
2. Task fails
3. Verify retry count increments
4. Verify task is retried up to 3 times

### Scenario 6: Task Timeout

1. Create task with `timeout: 5`
2. Task runs longer than 5 seconds
3. Verify task is marked as failed with "timeout" event

## Monitoring

### Check System Stats

```bash
curl http://localhost:8080/api/system/stats
```

### Check WebSocket

Using `wscat`:
```bash
wscat -c ws://localhost:8080/ws
```

Subscribe to channels:
```json
{"action": "subscribe", "channel": "tasks"}
{"action": "subscribe", "channel": "workers"}
```

## Debugging

### Enable Debug Logging

```bash
COWORK_LOG_LEVEL=debug ./bin/coordinator
```

### Check Database

```bash
sqlite3 ~/.cowork/coordinator/cowork.db
```

Useful queries:
```sql
SELECT * FROM tasks ORDER BY created_at DESC LIMIT 10;
SELECT * FROM workers;
SELECT * FROM task_dependencies;
```

## Common Issues

### Worker Registration Failed

- Check coordinator is running
- Check network connectivity
- Check worker name is unique

### Task Stuck in Pending

- No workers available with required tags
- Check worker tags match task `required_tags`

### Function Calling Not Working

- Check AI API key is configured
- Check model supports function calling
- Check tool definitions are registered