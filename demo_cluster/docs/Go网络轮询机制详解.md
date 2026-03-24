# Go 网络轮询（Netpoll）机制详解

## 1. `//go:linkname` 指令原理

### 1.1 什么是 `//go:linkname`

`//go:linkname` 是 Go 编译器的特殊指令，用于将当前包中的函数链接到另一个包中的私有函数。这是 Go 内部实现跨包调用私有函数的核心机制。

**语法格式：**
```go
//go:linkname localname importpath.name
```

- `localname`: 当前包中的函数名
- `importpath.name`: 目标包的完整路径和函数名

### 1.2 使用要求

1. 必须导入 `unsafe` 包（即使不直接使用）
2. 只能在 Go 标准库内部使用（用户代码使用会有风险）
3. 需要声明函数签名，但不需要实现

## 2. Netpoll 调用链路详解

### 2.1 完整调用链

```
用户代码 (net.Dial)
    ↓
net 包 (fd_unix.go)
    ↓
internal/poll 包 (fd_poll_runtime.go)
    ↓  [通过 go:linkname]
runtime 包 (netpoll.go)
    ↓
runtime 包 (netpoll_kqueue.go / netpoll_epoll.go)
    ↓
系统调用 (kqueue/epoll)
```

### 2.2 详细调用过程

#### 步骤 1: internal/poll 包声明函数

**文件：** `/src/internal/poll/fd_poll_runtime.go`

```go
package poll

import (
    _ "unsafe" // for go:linkname - 必须导入
)

// 声明函数签名，但不实现
// 这些函数实际在 runtime 包中实现
func runtime_pollServerInit()
func runtime_pollOpen(fd uintptr) (uintptr, int)
func runtime_pollClose(ctx uintptr)
func runtime_pollWait(ctx uintptr, mode int) int
func runtime_pollWaitCanceled(ctx uintptr, mode int)
func runtime_pollReset(ctx uintptr, mode int) int
func runtime_pollSetDeadline(ctx uintptr, d int64, mode int)
func runtime_pollUnblock(ctx uintptr)
func runtime_isPollServerDescriptor(fd uintptr) bool

// 使用示例
type pollDesc struct {
    runtimeCtx uintptr
}

func (pd *pollDesc) init(fd *FD) error {
    serverInit.Do(runtime_pollServerInit)  // 调用 runtime 函数
    ctx, errno := runtime_pollOpen(uintptr(fd.Sysfd))  // 调用 runtime 函数
    if errno != 0 {
        return errnoErr(syscall.Errno(errno))
    }
    pd.runtimeCtx = ctx
    return nil
}
```

#### 步骤 2: runtime 包实现函数并链接

**文件：** `/src/runtime/netpoll.go`

```go
package runtime

// 使用 go:linkname 将此函数链接到 internal/poll.runtime_pollOpen
//go:linkname poll_runtime_pollOpen internal/poll.runtime_pollOpen
func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
    pd := pollcache.alloc()
    lock(&pd.lock)
    
    // 初始化 pollDesc
    pd.fd = fd
    pd.closing = false
    pd.rg.Store(pdNil)
    pd.wg.Store(pdNil)
    pd.self = pd
    
    unlock(&pd.lock)
    
    // 调用平台特定的 netpollopen
    errno := netpollopen(fd, pd)
    if errno != 0 {
        pollcache.free(pd)
        return nil, int(errno)
    }
    return pd, 0
}

//go:linkname poll_runtime_pollClose internal/poll.runtime_pollClose
func poll_runtime_pollClose(ctx uintptr) {
    pd := (*pollDesc)(unsafe.Pointer(ctx))
    if !pd.closing {
        pd.closing = true
        netpollclose(pd.fd)
        pollcache.free(pd)
    }
}

//go:linkname poll_runtime_pollWait internal/poll.runtime_pollWait
func poll_runtime_pollWait(ctx uintptr, mode int) int {
    pd := (*pollDesc)(unsafe.Pointer(ctx))
    // 等待 I/O 就绪
    return netpollblock(pd, int32(mode), false)
}
```

#### 步骤 3: 平台特定实现

**文件：** `/src/runtime/netpoll_kqueue.go` (macOS)

```go
package runtime

import "unsafe"

func netpollopen(fd uintptr, pd *pollDesc) int32 {
    // 使用 kqueue 注册文件描述符
    var ev [2]keventt
    
    // 注册读事件
    ev[0] = keventt{
        ident:  uint64(fd),
        filter: _EVFILT_READ,
        flags:  _EV_ADD | _EV_CLEAR,
        udata:  (*byte)(unsafe.Pointer(pd)),
    }
    
    // 注册写事件
    ev[1] = keventt{
        ident:  uint64(fd),
        filter: _EVFILT_WRITE,
        flags:  _EV_ADD | _EV_CLEAR,
        udata:  (*byte)(unsafe.Pointer(pd)),
    }
    
    n := kevent(kq, &ev[0], 2, nil, 0, nil)
    if n < 0 {
        return -n
    }
    return 0
}
```

**文件：** `/src/runtime/netpoll_epoll.go` (Linux)

```go
package runtime

func netpollopen(fd uintptr, pd *pollDesc) int32 {
    var ev epollevent
    ev.events = _EPOLLIN | _EPOLLOUT | _EPOLLRDHUP | _EPOLLET
    ev.data = uint64(uintptr(unsafe.Pointer(pd)))
    
    return -epollctl(epfd, _EPOLL_CTL_ADD, int32(fd), &ev)
}
```

## 3. 实际使用示例

### 3.1 TCP 连接建立过程

```go
// 用户代码
conn, err := net.Dial("tcp", "example.com:80")

// ↓ net/dial.go
func Dial(network, address string) (Conn, error) {
    // ...
    return DialContext(context.Background(), network, address)
}

// ↓ net/fd_unix.go
func (fd *netFD) init() error {
    return fd.pfd.Init(fd.net, true)
}

// ↓ internal/poll/fd_unix.go
func (fd *FD) Init(net string, pollable bool) error {
    if pollable {
        // 初始化 pollDesc
        if err := fd.pd.init(fd); err != nil {
            return err
        }
    }
    return nil
}

// ↓ internal/poll/fd_poll_runtime.go
func (pd *pollDesc) init(fd *FD) error {
    serverInit.Do(runtime_pollServerInit)  // 第一次调用时初始化 poll 服务器
    ctx, errno := runtime_pollOpen(uintptr(fd.Sysfd))  // 注册 fd 到 epoll/kqueue
    if errno != 0 {
        return errnoErr(syscall.Errno(errno))
    }
    pd.runtimeCtx = ctx
    return nil
}

// ↓ runtime/netpoll.go (通过 go:linkname 链接)
func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
    pd := pollcache.alloc()
    // ... 初始化 pollDesc ...
    errno := netpollopen(fd, pd)  // 调用平台特定实现
    return pd, int(errno)
}

// ↓ runtime/netpoll_kqueue.go (macOS) 或 runtime/netpoll_epoll.go (Linux)
func netpollopen(fd uintptr, pd *pollDesc) int32 {
    // 调用 kqueue/epoll 系统调用注册 fd
    return kevent(...) // 或 epollctl(...)
}
```

### 3.2 等待 I/O 就绪

```go
// 用户代码
n, err := conn.Read(buf)

// ↓ net/net.go
func (c *conn) Read(b []byte) (int, error) {
    return c.fd.Read(b)
}

// ↓ net/fd_posix.go
func (fd *netFD) Read(p []byte) (n int, err error) {
    n, err = fd.pfd.Read(p)
    return n, wrapSyscallError(readSyscallName, err)
}

// ↓ internal/poll/fd_unix.go
func (fd *FD) Read(p []byte) (int, error) {
    for {
        n, err := ignoringEINTRIO(syscall.Read, fd.Sysfd, p)
        if err == nil {
            return n, nil
        }
        if err == syscall.EAGAIN {
            // 数据未就绪，等待
            if err = fd.pd.waitRead(fd.isFile); err == nil {
                continue  // 重试读取
            }
        }
        return n, err
    }
}

// ↓ internal/poll/fd_poll_runtime.go
func (pd *pollDesc) waitRead(isFile bool) error {
    return pd.wait('r', isFile)
}

func (pd *pollDesc) wait(mode int, isFile bool) error {
    res := runtime_pollWait(pd.runtimeCtx, mode)  // 调用 runtime
    return convertErr(res, isFile)
}

// ↓ runtime/netpoll.go (通过 go:linkname 链接)
func poll_runtime_pollWait(ctx uintptr, mode int) int {
    pd := (*pollDesc)(unsafe.Pointer(ctx))
    return netpollblock(pd, int32(mode), false)  // 阻塞等待 I/O 就绪
}

// ↓ runtime/netpoll.go
func netpollblock(pd *pollDesc, mode int32, waitio bool) int {
    gpp := &pd.rg
    if mode == 'w' {
        gpp = &pd.wg
    }
    
    // 将当前 goroutine 挂起，等待 I/O 就绪
    gopark(netpollblockcommit, unsafe.Pointer(gpp), waitReasonIOWait, traceBlockNet, 5)
    
    // 被唤醒后返回
    return pollNoError
}
```

## 4. 关键数据结构

### 4.1 pollDesc (runtime 包)

```go
type pollDesc struct {
    _     sys.NotInHeap
    link  *pollDesc      // 链表指针
    fd    uintptr        // 文件描述符
    
    // 读写等待的 goroutine
    rg    atomic.Uintptr // pdReady, pdWait, G waiting for read or pdNil
    wg    atomic.Uintptr // pdReady, pdWait, G waiting for write or pdNil
    
    lock    mutex
    closing bool
    
    // 用户设置的截止时间
    rd      int64 // read deadline
    wd      int64 // write deadline
    
    rseq    uintptr // 读序列号
    wseq    uintptr // 写序列号
    
    self    *pollDesc // 指向自己
}
```

### 4.2 pollCache (runtime 包)

```go
type pollCache struct {
    lock  mutex
    first *pollDesc  // 空闲 pollDesc 链表
}

// 分配 pollDesc
func (c *pollCache) alloc() *pollDesc {
    lock(&c.lock)
    if c.first == nil {
        // 批量分配
        const pdSize = unsafe.Sizeof(pollDesc{})
        n := pollBlockSize / pdSize
        mem := persistentalloc(n*pdSize, 0, &memstats.other_sys)
        for i := uintptr(0); i < n; i++ {
            pd := (*pollDesc)(add(mem, i*pdSize))
            pd.link = c.first
            c.first = pd
        }
    }
    pd := c.first
    c.first = pd.link
    unlock(&c.lock)
    return pd
}
```

## 5. 平台差异

### 5.1 macOS/BSD - kqueue

```go
// netpoll_kqueue.go
func netpollinit() {
    kq = kqueue()  // 创建 kqueue
}

func netpollopen(fd uintptr, pd *pollDesc) int32 {
    // 使用 EV_ADD 添加事件
    // 使用 EV_CLEAR 边缘触发模式
    return kevent(kq, &ev[0], 2, nil, 0, nil)
}

func netpoll(delay int64) (gList, int32) {
    var events [64]keventt
    n := kevent(kq, nil, 0, &events[0], int32(len(events)), tp)
    // 处理就绪事件
}
```

### 5.2 Linux - epoll

```go
// netpoll_epoll.go
func netpollinit() {
    epfd = epollcreate1(_EPOLL_CLOEXEC)  // 创建 epoll
}

func netpollopen(fd uintptr, pd *pollDesc) int32 {
    // 使用 EPOLLET 边缘触发模式
    // 使用 EPOLLIN | EPOLLOUT 监听读写
    return epollctl(epfd, _EPOLL_CTL_ADD, int32(fd), &ev)
}

func netpoll(delay int64) (gList, int32) {
    var events [128]epollevent
    n := epollwait(epfd, &events[0], int32(len(events)), waitms)
    // 处理就绪事件
}
```

## 6. 为什么使用 `//go:linkname`

### 6.1 设计原因

1. **包隔离**: `internal/poll` 是内部包，不能直接访问 `runtime` 包的私有函数
2. **避免循环依赖**: `runtime` 包是最底层的包，不能导入其他包
3. **性能优化**: 直接链接避免了接口调用的开销
4. **实现隐藏**: 用户代码不需要知道底层实现细节

### 6.2 调用流程图

```
┌─────────────────┐
│   用户代码       │
│  net.Dial()     │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│   net 包        │
│  fd_unix.go     │
└────────┬────────┘
         │
         ↓
┌─────────────────────────────┐
│   internal/poll 包          │
│  fd_poll_runtime.go         │
│                             │
│  func runtime_pollOpen(...) │  ← 声明但不实现
└────────┬────────────────────┘
         │
         │ go:linkname 链接
         ↓
┌─────────────────────────────┐
│   runtime 包                │
│  netpoll.go                 │
│                             │
│  func poll_runtime_pollOpen │  ← 实际实现
│  //go:linkname ...          │
└────────┬────────────────────┘
         │
         ↓
┌─────────────────────────────┐
│   runtime 包 (平台特定)      │
│  netpoll_kqueue.go (macOS)  │
│  netpoll_epoll.go (Linux)   │
│                             │
│  func netpollopen(...)      │
└────────┬────────────────────┘
         │
         ↓
┌─────────────────┐
│   系统调用       │
│  kqueue/epoll   │
└─────────────────┘
```

## 7. 注意事项

### 7.1 使用限制

1. `//go:linkname` 只应在 Go 标准库内部使用
2. 用户代码使用可能导致：
   - 编译器版本兼容性问题
   - 运行时崩溃
   - 无法保证 API 稳定性

### 7.2 调试技巧

```bash
# 查看链接关系
go build -gcflags="-m -m" your_package

# 查看汇编代码
go tool compile -S your_file.go

# 查看符号表
go tool nm your_binary | grep poll
```

## 8. 总结

1. `//go:linkname` 是 Go 内部跨包调用私有函数的机制
2. Netpoll 使用此机制连接 `internal/poll` 和 `runtime` 包
3. 调用链路：用户代码 → net → internal/poll → runtime → 系统调用
4. 不同平台使用不同的 I/O 多路复用机制（kqueue/epoll）
5. 这种设计实现了高性能的网络 I/O 和良好的包隔离

## 9. 参考资料

- Go 源码：`/src/runtime/netpoll*.go`
- Go 源码：`/src/internal/poll/fd_poll_runtime.go`
- Go 源码：`/src/net/fd_unix.go`
