# Concurrency Control Design

## 问题描述

同一个Issue/PR上的多个`/pilot`命令会并发执行，导致：

1. **文件系统冲突**：同时clone同一仓库到不同目录
2. **Git冲突**：同时创建分支、commit、push
3. **资源浪费**：重复执行相同或类似的任务
4. **用户体验差**：多个tracking comment，难以追踪进度

## 设计原则 (Linus's Good Taste)

### 1. 消除特殊情况 (Eliminate Special Cases)

**Bad Approach（有特殊情况）：**
```go
if taskRunning[key] {
    if shouldQueue {
        queue.Add(task)  // 特殊情况1：排队
    } else {
        return ErrBusy   // 特殊情况2：拒绝
    }
} else {
    taskRunning[key] = true  // 特殊情况3：需要手动cleanup
    execute()
    taskRunning[key] = false // 容易忘记！
}
```

**Good Taste Approach（无特殊情况）：**
```go
unlock := locker.Acquire(key)
defer unlock()
// do work...
```

### 2. 返回Cleanup函数模式

像`github.Clone()`一样，返回cleanup函数：

```go
// Clone returns workdir, cleanup, err
workdir, cleanup, err := github.Clone(repo, branch)
defer cleanup()

// Locker returns unlock function
unlock := locker.Acquire(key)
defer unlock()
```

优点：
- 统一的API风格
- defer保证cleanup执行
- 即使panic也会unlock（Go的defer保证）

### 3. 简单的数据结构

```go
type Locker struct {
    locks map[string]*sync.Mutex  // key -> mutex
    mu    sync.Mutex               // protects the map
}
```

不使用：
- Redis/分布式锁（overengineering for single-instance）
- Database（unnecessary complexity）
- Channels（harder to reason about）

### 4. 锁粒度

**Key格式：** `{repo}:{type}-{number}`

示例：
- `owner/repo:issue-123`
- `torvalds/linux:pr-456`

**为什么？**
- Too coarse (repo-level): issue-1和issue-2不能并行
- Too fine (comment-level): 同一issue的多个命令应该串行
- Just right: 同一issue串行，不同issue并行

### 5. 超时机制

**不需要超时！**

理由：
1. Go的defer保证即使panic也会unlock
2. goroutine泄露会被Go runtime检测
3. 添加超时会增加复杂度（需要处理超时后的状态）
4. 如果任务真的hang住，应该修复任务本身而不是依赖超时

## 实现

### 核心接口

```go
// Acquire acquires a lock for the given key
// Returns unlock function for defer
func (l *Locker) Acquire(key string) func()

// BuildKey builds a concurrency key from task metadata
func BuildKey(repo string, number int, isPR bool) string
```

### 集成到Handler

```go
go func() {
    // Acquire lock (blocks if another task is running)
    key := executor.BuildKey(task.Repo, task.Number, task.IsPR)
    unlock := h.locker.Acquire(key)
    defer unlock()

    // Execute task (exclusive access guaranteed)
    h.executor.Execute(ctx, task)
}()
```

### 零破坏 (Zero Breakage)

- 现有代码无需修改（除了Handler）
- 对用户透明（第二个请求会等待，不会报错）
- 向后兼容（如果不使用locker，行为不变）

## 测试验证

### 1. 基本功能测试
- Sequential acquisition
- Concurrent acquisition (same key)
- Multiple keys (parallel execution)

### 2. 正确性测试
- Counter increment without race
- Execution order guarantee

### 3. 健壮性测试
- Panic safety (unlock still runs)
- Long-running tasks (no deadlock)

### 4. 性能测试
- Different keys should run in parallel
- Same key should serialize

## 使用示例

```go
// In webhook handler
func (h *Handler) HandleIssueComment(w http.ResponseWriter, r *http.Request) {
    // ... parse request ...

    go func() {
        // Build unique key for this issue/PR
        key := executor.BuildKey(task.Repo, task.Number, task.IsPR)
        
        // Acquire lock (blocks if another task is running)
        unlock := h.locker.Acquire(key)
        defer unlock()
        
        // Now we have exclusive access
        log.Printf("Executing task for %s (lock held)", key)
        h.executor.Execute(ctx, task)
    }()
}
```

## 可观测性 (Observability)

日志输出：
```
[Concurrency] Attempting to acquire lock: owner/repo:issue-123
[Concurrency] Lock acquired: owner/repo:issue-123
Executing task for owner/repo:issue-123 (lock held)
... task execution logs ...
[Concurrency] Lock released: owner/repo:issue-123
```

如果第二个请求到达时第一个还在执行：
```
[Concurrency] Attempting to acquire lock: owner/repo:issue-123
[Concurrency] Lock acquired: owner/repo:issue-123  # (after waiting)
```

## 未来改进空间

如果需要支持多实例部署：

1. **Redis分布式锁**：使用Redlock算法
2. **数据库锁**：使用PostgreSQL的advisory locks
3. **Kubernetes Lease**：使用K8s coordination API

但现在不需要！（YAGNI - You Aren't Gonna Need It）

## 代码审查要点

When reviewing this code, check:

1. ✅ **No special cases**: Single code path for all scenarios
2. ✅ **defer unlock()**: Always paired with Acquire()
3. ✅ **Shallow indentation**: Max 2 levels in Acquire()
4. ✅ **Clear naming**: Locker, Acquire, BuildKey (not Manager, Lock, MakeKey)
5. ✅ **Zero configuration**: Works out-of-box, no tuning needed

## Reference

This design follows Linus Torvalds's "Good Taste" principle:

> "Sometimes you can look at a problem from a different angle and rewrite it 
> so that the special case disappears and becomes the normal case."

The cleanup function pattern eliminates the special case of "what if cleanup 
fails" by letting Go's defer mechanism handle it uniformly.