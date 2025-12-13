# MyMTR 技术设计文档

## 技术选型

### 核心依赖

| 组件 | 库 | 版本 | 用途 |
|------|------|------|------|
| CLI | `github.com/spf13/cobra` | v1.8+ | 命令行框架 |
| TUI | `github.com/charmbracelet/bubbletea` | v0.25+ | 终端 UI 框架 |
| TUI 样式 | `github.com/charmbracelet/lipgloss` | v0.9+ | TUI 样式渲染 |
| ICMP | `golang.org/x/net/icmp` | latest | ICMP 协议处理 |
| IP 网络 | `golang.org/x/net/ipv4` | latest | IPv4 原始套接字 |
| IP 网络 | `golang.org/x/net/ipv6` | latest | IPv6 原始套接字 |
| IP 解析 | `github.com/lionsoul2014/ip2region/binding/golang` | v2.11+ | IP 地理位置 |
| DNS | 标准库 `net` | - | DNS 解析 |

### 可选依赖

| 组件 | 库 | 用途 |
|------|------|------|
| GeoIP2 | `github.com/oschwald/geoip2-golang` | MaxMind 数据库支持 |
| QQWry | `github.com/freshcn/qqwry` | 纯真数据库支持 |

## 核心数据结构

### Hop (跳数信息)

```go
// Hop 表示路由中的一跳
type Hop struct {
    TTL      int           // TTL 值 (1-30)
    IP       net.IP        // 响应的 IP 地址
    Hostname string        // 反向 DNS 解析结果
    Location *GeoLocation  // 地理位置信息
    Stats    *HopStats     // 统计数据
    Lost     bool          // 是否无响应 (*)
}

// HopStats 统计数据
type HopStats struct {
    Sent     int           // 发送包数
    Received int           // 接收包数
    Loss     float64       // 丢包率 (0.0 - 100.0)
    Last     time.Duration // 最近一次 RTT
    Best     time.Duration // 最小 RTT
    Worst    time.Duration // 最大 RTT
    Avg      time.Duration // 平均 RTT
    StdDev   time.Duration // 标准差
    History  []time.Duration // 最近 N 次 RTT 记录
}

// GeoLocation 地理位置
type GeoLocation struct {
    Country  string // 国家
    Province string // 省份
    City     string // 城市
    ISP      string // 运营商
    Source   string // 数据来源 (ip2region/geoip2/qqwry)
}
```

### ProbeResult (探测结果)

```go
// ProbeResult 单次探测结果
type ProbeResult struct {
    TTL       int           // 目标 TTL
    IP        net.IP        // 响应 IP
    RTT       time.Duration // 往返时间
    Type      ICMPType      // ICMP 类型
    Code      int           // ICMP 代码
    Timestamp time.Time     // 探测时间
    Error     error         // 错误信息
}

// ICMPType ICMP 响应类型
type ICMPType int

const (
    ICMPTypeEchoReply      ICMPType = iota // Echo Reply (到达目标)
    ICMPTypeTimeExceeded                   // Time Exceeded (中间节点)
    ICMPTypeDestUnreach                    // Destination Unreachable
    ICMPTypeTimeout                        // 超时无响应
)
```

### MTRConfig (配置)

```go
// MTRConfig MTR 配置
type MTRConfig struct {
    Target      string        // 目标主机
    MaxHops     int           // 最大跳数
    Count       int           // 探测次数 (0=无限)
    Interval    time.Duration // 探测间隔
    Timeout     time.Duration // 单次超时
    Protocol    Protocol      // 探测协议
    IPVersion   int           // IP 版本 (4/6)
    GeoIPSource string        // IP 数据库
    PacketSize  int           // 包大小
}

type Protocol string

const (
    ProtocolICMP Protocol = "icmp"
    ProtocolUDP  Protocol = "udp"
)
```

## 核心流程

### 1. 初始化流程

```
┌─────────────────────────────────────────────────────────────┐
│                        初始化流程                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │   解析命令行参数   │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  DNS 解析目标地址  │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  初始化 GeoIP DB  │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  检查网络权限     │
                    │  (ICMP/UDP 选择)  │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  创建原始套接字   │
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  启动 TUI/CLI    │
                    └──────────────────┘
```

### 2. 探测循环

```go
// 伪代码：探测循环
func (c *Controller) Run(ctx context.Context) error {
    // 创建接收器 goroutine
    go c.receiveLoop(ctx)

    ticker := time.NewTicker(c.config.Interval)
    defer ticker.Stop()

    for round := 0; ; round++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // 并发探测所有 TTL
            for ttl := 1; ttl <= c.maxTTL; ttl++ {
                go c.probe(ttl, round)
            }
        }

        // 检查是否达到指定次数
        if c.config.Count > 0 && round >= c.config.Count {
            return nil
        }
    }
}
```

### 3. ICMP 探测实现

```go
// ICMP Echo Request 探测
func (p *ICMPProber) Probe(ttl int, seq int) (*ProbeResult, error) {
    // 1. 创建 ICMP Echo Request
    msg := icmp.Message{
        Type: ipv4.ICMPTypeEcho,
        Code: 0,
        Body: &icmp.Echo{
            ID:   p.id,
            Seq:  seq,
            Data: p.payload,
        },
    }

    // 2. 设置 TTL
    p.conn.IPv4PacketConn().SetTTL(ttl)

    // 3. 记录发送时间
    sendTime := time.Now()

    // 4. 发送
    p.conn.WriteTo(msg.Marshal(), p.target)

    // 5. 等待响应 (在 receiveLoop 中处理)
    // ...
}
```

### 4. 响应处理

```go
func (c *Controller) receiveLoop(ctx context.Context) {
    buf := make([]byte, 1500)

    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
        n, peer, err := c.conn.ReadFrom(buf)
        if err != nil {
            continue
        }

        // 解析 ICMP 消息
        msg, err := icmp.ParseMessage(ProtocolICMP, buf[:n])
        if err != nil {
            continue
        }

        // 根据类型处理
        switch msg.Type {
        case ipv4.ICMPTypeEchoReply:
            // 到达目标
            c.handleEchoReply(msg, peer, time.Now())
        case ipv4.ICMPTypeTimeExceeded:
            // 中间节点响应
            c.handleTimeExceeded(msg, peer, time.Now())
        case ipv4.ICMPTypeDestinationUnreachable:
            // 目标不可达
            c.handleDestUnreach(msg, peer, time.Now())
        }
    }
}
```

## GeoIP 解析器设计

### 接口定义

```go
// GeoResolver IP 地理位置解析器接口
type GeoResolver interface {
    // Resolve 解析 IP 地址的地理位置
    Resolve(ip net.IP) (*GeoLocation, error)

    // Source 返回数据源名称
    Source() string

    // Close 关闭解析器
    Close() error
}
```

### ip2region 实现

```go
type IP2RegionResolver struct {
    searcher *xdb.Searcher
}

func NewIP2RegionResolver(dbPath string) (*IP2RegionResolver, error) {
    // 使用内存缓存模式，查询性能最优
    cBuff, err := xdb.LoadContentFromFile(dbPath)
    if err != nil {
        return nil, err
    }

    searcher, err := xdb.NewWithBuffer(cBuff)
    if err != nil {
        return nil, err
    }

    return &IP2RegionResolver{searcher: searcher}, nil
}

func (r *IP2RegionResolver) Resolve(ip net.IP) (*GeoLocation, error) {
    result, err := r.searcher.SearchByStr(ip.String())
    if err != nil {
        return nil, err
    }

    // 解析格式: "国家|区域|省份|城市|ISP"
    parts := strings.Split(result, "|")
    return &GeoLocation{
        Country:  parts[0],
        Province: parts[2],
        City:     parts[3],
        ISP:      parts[4],
        Source:   "ip2region",
    }, nil
}
```

## TUI 设计

### 界面布局

```
┌─────────────────────────────────────────────────────────────────────┐
│ MyMTR - google.com (142.250.190.78)                    [q]退出      │
├─────────────────────────────────────────────────────────────────────┤
│ #  │ Host                    │ Loss% │ Snt │ Last │ Avg  │ Best │ Wrst │ StDev │ Location            │
├────┼─────────────────────────┼───────┼─────┼──────┼──────┼──────┼──────┼───────┼─────────────────────┤
│  1 │ 192.168.1.1             │  0.0% │  10 │  1.2 │  1.5 │  1.0 │  2.1 │   0.3 │ 局域网              │
│  2 │ 10.0.0.1                │  0.0% │  10 │  5.3 │  5.8 │  4.2 │  8.1 │   1.2 │ 内网                │
│  3 │ 183.232.56.1            │  0.0% │  10 │ 12.4 │ 13.2 │ 11.8 │ 15.3 │   1.1 │ 广东深圳 电信       │
│  4 │ 202.97.94.150           │ 10.0% │  10 │ 25.6 │ 28.4 │ 24.1 │ 35.2 │   3.5 │ 广东广州 电信骨干   │
│  5 │ ???                     │ 100%  │  10 │    - │    - │    - │    - │     - │                     │
│  6 │ 72.14.222.136           │  0.0% │  10 │ 45.2 │ 46.8 │ 44.1 │ 52.3 │   2.8 │ 美国 Google         │
│  7 │ 142.250.190.78          │  0.0% │  10 │ 48.5 │ 49.2 │ 47.8 │ 53.1 │   1.5 │ 美国 Google         │
└────┴─────────────────────────┴───────┴─────┴──────┴──────┴──────┴──────┴───────┴─────────────────────┘
│ 探测中... 间隔: 1s  协议: ICMP  IP库: ip2region                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Bubbletea Model

```go
type Model struct {
    // 数据
    target    string
    targetIP  net.IP
    hops      []*Hop
    maxHops   int

    // 状态
    probing   bool
    err       error

    // UI
    width     int
    height    int
    styles    Styles
}

// Update 处理消息
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
    case HopUpdateMsg:
        m.hops[msg.TTL-1] = msg.Hop
    }
    return m, nil
}
```

## 错误处理

### 权限错误

```go
func (c *Controller) checkPermission() error {
    // 尝试创建原始套接字
    conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
    if err != nil {
        if os.IsPermission(err) {
            return fmt.Errorf("需要 root 权限或 CAP_NET_RAW 能力: %w\n"+
                "解决方案:\n"+
                "  1. sudo %s\n"+
                "  2. sudo setcap cap_net_raw+ep %s\n"+
                "  3. 使用 -p udp 切换到 UDP 模式",
                err, os.Args[0], os.Args[0])
        }
        return err
    }
    conn.Close()
    return nil
}
```

### 网络错误

```go
// 探测超时不视为错误，标记为丢包
// 网络不可达等错误需要记录并显示
```

## 性能优化

### 1. GeoIP 查询优化

```go
// 使用内存缓存模式加载整个数据库
// ip2region 完全加载后查询在 10 微秒级别
cBuff, _ := xdb.LoadContentFromFile(dbPath)
searcher, _ := xdb.NewWithBuffer(cBuff)
```

### 2. 并发控制

```go
// 使用 worker pool 限制并发探测数量
// 避免瞬时发送过多 ICMP 包被防火墙拦截
sem := make(chan struct{}, 10) // 最多 10 个并发探测
```

### 3. DNS 缓存

```go
// 缓存反向 DNS 解析结果
type DNSCache struct {
    cache sync.Map // map[string]string
}

func (c *DNSCache) Lookup(ip net.IP) string {
    key := ip.String()
    if v, ok := c.cache.Load(key); ok {
        return v.(string)
    }
    names, _ := net.LookupAddr(key)
    if len(names) > 0 {
        c.cache.Store(key, names[0])
        return names[0]
    }
    return ""
}
```

## 测试策略

### 单元测试

- Stats 计算正确性
- GeoIP 解析正确性
- ICMP 消息编解码

### 集成测试

- 本地回环探测 (127.0.0.1)
- 局域网网关探测
- 公网目标探测

### Mock 测试

```go
// 模拟 Prober 用于测试 Controller
type MockProber struct {
    results map[int]*ProbeResult
}

func (m *MockProber) Probe(ttl int, seq int) (*ProbeResult, error) {
    return m.results[ttl], nil
}
```
