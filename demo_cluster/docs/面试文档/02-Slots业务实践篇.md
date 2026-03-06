# 第二篇: Slots游戏业务实践

## 一、Slots游戏核心概念

### 1.1 什么是Slots游戏

Slots(老虎机)是一种经典的博彩游戏,玩家通过下注、旋转(Spin)、获得奖励的循环来进行游戏。

**核心要素**:
- **Reels(卷轴)**: 通常3-5个,每个卷轴有多个符号
- **Paylines(赔付线)**: 中奖的连线方式
- **Symbols(符号)**: 不同符号有不同赔率
- **RTP(Return To Player)**: 玩家回报率,通常92%-98%
- **Volatility(波动率)**: 高波动=大奖少但金额大,低波动=小奖多但金额小

### 1.2 Slots游戏流程

```
玩家登录 → 选择机台 → 设置下注 → 点击Spin → 
计算结果 → 展示动画 → 发放奖励 → 更新余额 → 继续游戏
```

## 二、Slots服务器架构

### 2.1 整体架构

```
客户端(Web/App)
    ↓ HTTP/WebSocket
Web节点(登录/充值)
    ↓ 
Gate节点(长连接网关)
    ↓ RPC
Game节点(游戏逻辑)
    ↓
Center节点(账号管理)
    ↓
PostgreSQL(数据持久化)
```

### 2.2 核心模块设计

#### 2.2.1 玩家Actor (actor_player.go)

**职责**: 管理单个玩家的所有状态和行为

```go
type ActorPlayer struct {
    cherry.ActorBase
    
    // 基础信息
    UserId    int64
    PlayerName string
    Level     int32
    
    // 游戏状态
    Coin      int64      // 金币余额
    Machine   *Machine   // 当前机台
    RoomId    int32      // 所在房间
    
    // 统计数据
    TotalBet  int64      // 累计下注
    TotalWin  int64      // 累计赢得
    SpinCount int64      // Spin次数
}

// 关键方法
func (a *ActorPlayer) OnInit() {
    // 初始化玩家数据
    a.loadFromDB()
}

func (a *ActorPlayer) OnReceive(msg cherry.Message) {
    switch msg.Route {
    case "player.spin":
        a.handleSpin(msg)
    case "player.bet":
        a.handleBet(msg)
    }
}

func (a *ActorPlayer) OnStop() {
    // 玩家下线,保存数据
    a.saveToDB()
}
```

**设计亮点**:
1. 单线程处理,无需加锁
2. 消息驱动,异步处理
3. 状态持久化,数据安全
4. 生命周期管理,资源释放

#### 2.2.2 机台房间 (level_room.go)

**职责**: 管理多个玩家的机台实例

```go
type LevelRoom struct {
    cherry.ActorBase
    
    RoomId    int32
    Level     int32                    // 房间等级
    Players   map[int64]*ActorPlayer   // 房间内玩家
    Machines  map[int32]*MachineConfig // 机台配置
    
    // 房间统计
    TotalPlayers int32
    TotalBet     int64
    TotalWin     int64
}

// 玩家进入房间
func (r *LevelRoom) OnPlayerEnter(player *ActorPlayer) {
    r.Players[player.UserId] = player
    r.TotalPlayers++
    
    // 分配机台
    machine := r.allocateMachine(player)
    player.Machine = machine
}

// 玩家离开房间
func (r *LevelRoom) OnPlayerLeave(userId int64) {
    delete(r.Players, userId)
    r.TotalPlayers--
}
```

**设计亮点**:
1. 房间隔离,互不影响
2. 动态扩容,按需创建
3. 玩家路由,快速定位
4. 统计数据,实时监控

## 三、Slots核心算法

### 3.1 Spin算法流程

```go
func (a *ActorPlayer) handleSpin(req *pb.SpinRequest) error {
    // 1. 参数验证
    if req.BetAmount <= 0 || req.BetAmount > a.Coin {
        return errors.New("invalid bet amount")
    }
    
    // 2. 扣除下注金额
    a.Coin -= req.BetAmount
    a.TotalBet += req.BetAmount
    
    // 3. 执行随机算法
    result := a.calculateSpinResult(req)
    
    // 4. 计算中奖金额
    winAmount := a.calculateWinAmount(result)
    
    // 5. 发放奖励
    a.Coin += winAmount
    a.TotalWin += winAmount
    
    // 6. 更新统计
    a.SpinCount++
    
    // 7. 异步持久化
    go a.saveToDB()
    
    // 8. 返回结果
    return a.sendSpinResult(result, winAmount)
}
```

### 3.2 RNG(随机数生成)算法

**核心要求**: 
- 密码学安全
- 均匀分布
- 不可预测
- 可审计

```go
import "crypto/rand"

// 生成随机数
func generateRandomNumber(max int) int {
    b := make([]byte, 8)
    rand.Read(b)
    n := binary.BigEndian.Uint64(b)
    return int(n % uint64(max))
}

// Spin结果生成
func (a *ActorPlayer) calculateSpinResult(req *pb.SpinRequest) *SpinResult {
    result := &SpinResult{
        Reels: make([][]int, 5), // 5个卷轴
    }
    
    // 每个卷轴独立随机
    for i := 0; i < 5; i++ {
        reelConfig := a.Machine.Reels[i]
        position := generateRandomNumber(len(reelConfig.Symbols))
        
        // 获取3个连续符号
        result.Reels[i] = []int{
            reelConfig.Symbols[position],
            reelConfig.Symbols[(position+1)%len(reelConfig.Symbols)],
            reelConfig.Symbols[(position+2)%len(reelConfig.Symbols)],
        }
    }
    
    return result
}
```

### 3.3 RTP控制算法

**RTP(Return To Player)**: 玩家长期回报率

```go
// RTP配置
type RTPConfig struct {
    TargetRTP    float64  // 目标RTP: 95%
    MinRTP       float64  // 最小RTP: 92%
    MaxRTP       float64  // 最大RTP: 98%
    CheckPeriod  int64    // 检查周期: 1000次Spin
}

// RTP计算
func (a *ActorPlayer) calculateCurrentRTP() float64 {
    if a.TotalBet == 0 {
        return 0
    }
    return float64(a.TotalWin) / float64(a.TotalBet)
}

// RTP调整
func (a *ActorPlayer) adjustRTP(result *SpinResult) {
    currentRTP := a.calculateCurrentRTP()
    targetRTP := a.Machine.Config.TargetRTP
    
    // RTP过低,增加中奖概率
    if currentRTP < targetRTP - 0.02 {
        result.WinMultiplier *= 1.1
    }
    
    // RTP过高,降低中奖概率
    if currentRTP > targetRTP + 0.02 {
        result.WinMultiplier *= 0.9
    }
}
```

### 3.4 大奖触发算法

**Jackpot(累积奖池)**: 多个玩家共同累积的大奖

```go
type JackpotPool struct {
    PoolId      int32
    TotalAmount int64    // 奖池总额
    MinTrigger  int64    // 最小触发金额
    MaxTrigger  int64    // 最大触发金额
    
    // 触发概率随奖池增长
    BaseProbability float64  // 基础概率: 0.001%
}

// 判断是否触发Jackpot
func (j *JackpotPool) shouldTrigger(betAmount int64) bool {
    // 奖池越大,触发概率越高
    probability := j.BaseProbability * (float64(j.TotalAmount) / float64(j.MinTrigger))
    
    // 随机判断
    random := rand.Float64()
    return random < probability
}

// Spin时检查Jackpot
func (a *ActorPlayer) checkJackpot(betAmount int64) int64 {
    jackpot := a.Machine.JackpotPool
    
    // 累积奖池(每次下注的1%进入奖池)
    contribution := betAmount / 100
    jackpot.TotalAmount += contribution
    
    // 判断是否触发
    if jackpot.shouldTrigger(betAmount) {
        winAmount := jackpot.TotalAmount
        jackpot.TotalAmount = 0  // 重置奖池
        
        // 记录大奖
        a.recordJackpotWin(winAmount)
        
        return winAmount
    }
    
    return 0
}
```

## 四、数据流转

### 4.1 完整Spin流程

```
1. 客户端发起Spin请求
   ↓
2. Gate节点接收WebSocket消息
   ↓
3. 查询玩家所在Game节点(通过Center)
   ↓
4. 路由消息到目标Game节点
   ↓
5. Game节点定位玩家Actor
   ↓
6. Actor处理Spin逻辑
   - 验证参数
   - 扣除金币
   - 执行RNG
   - 计算结果
   - 发放奖励
   - 检查Jackpot
   ↓
7. 更新内存状态
   ↓
8. 异步持久化到DB
   ↓
9. 返回结果给客户端
   ↓
10. 客户端展示动画
```

### 4.2 数据一致性保证

**问题**: 如何保证金币扣除和奖励发放的一致性?

**方案1**: Actor单线程 + 内存状态
```go
// Actor内部串行处理,天然保证一致性
func (a *ActorPlayer) handleSpin(req *SpinRequest) {
    // 1. 扣除金币(内存操作)
    a.Coin -= req.BetAmount
    
    // 2. 计算结果
    result := a.calculateResult()
    
    // 3. 发放奖励(内存操作)
    a.Coin += result.WinAmount
    
    // 4. 异步持久化
    go a.saveToDB()
}
```

**方案2**: 数据库事务
```go
func (a *ActorPlayer) saveToDBWithTransaction() error {
    tx := db.Begin()
    defer tx.Rollback()
    
    // 更新金币
    tx.Model(&Player{}).Where("user_id = ?", a.UserId).
        Update("coin", a.Coin)
    
    // 记录Spin历史
    tx.Create(&SpinHistory{
        UserId: a.UserId,
        BetAmount: req.BetAmount,
        WinAmount: result.WinAmount,
    })
    
    return tx.Commit().Error
}
```

## 五、性能优化

### 5.1 对象池优化

```go
// SpinResult对象池
var spinResultPool = sync.Pool{
    New: func() interface{} {
        return &SpinResult{
            Reels: make([][]int, 5),
        }
    },
}

// 使用
result := spinResultPool.Get().(*SpinResult)
// ... 使用result
spinResultPool.Put(result)  // 归还
```

### 5.2 批量持久化

```go
// 批量保存玩家数据
type PlayerSaver struct {
    buffer   []*Player
    mu       sync.Mutex
    ticker   *time.Ticker
}

func (s *PlayerSaver) Add(player *Player) {
    s.mu.Lock()
    s.buffer = append(s.buffer, player)
    s.mu.Unlock()
}

func (s *PlayerSaver) Flush() {
    s.mu.Lock()
    batch := s.buffer
    s.buffer = make([]*Player, 0, 100)
    s.mu.Unlock()
    
    // 批量写入
    db.CreateInBatches(batch, 100)
}

// 定时刷新
func (s *PlayerSaver) Start() {
    s.ticker = time.NewTicker(5 * time.Second)
    go func() {
        for range s.ticker.C {
            s.Flush()
        }
    }()
}
```

### 5.3 缓存策略

```go
// 机台配置缓存
type MachineConfigCache struct {
    cache map[int32]*MachineConfig
    mu    sync.RWMutex
}

func (c *MachineConfigCache) Get(machineId int32) *MachineConfig {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.cache[machineId]
}

// 热更新配置
func (c *MachineConfigCache) Reload() {
    newConfigs := loadConfigFromDB()
    
    c.mu.Lock()
    c.cache = newConfigs
    c.mu.Unlock()
}
```

---

**下一篇**: [Cherry框架特性篇](./03-Cherry框架特性篇.md)
