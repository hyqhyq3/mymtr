# MyMTR API 设计文档

## 模块接口定义

### 1. Prober 接口

探测器负责发送网络探测包并收集响应。

```go
package mtr

import (
    "context"
    "net"
    "time"
)

// Prober 网络探测器接口
type Prober interface {
    // Probe 发送一次探测
    // ttl: 生存时间
    // seq: 序列号（用于匹配响应）
    Probe(ctx context.Context, ttl int, seq int) (*ProbeResult, error)

    // SetTarget 设置探测目标
    SetTarget(ip net.IP) error

    // Close 关闭探测器
    Close() error
}

// ProbeResult 单次探测结果
type ProbeResult struct {
    TTL       int           `json:"ttl"`
    Seq       int           `json:"seq"`
    IP        net.IP        `json:"ip,omitempty"`
    RTT       time.Duration `json:"rtt,omitempty"`
    Type      ResponseType  `json:"type"`
    Timestamp time.Time     `json:"timestamp"`
    Error     error         `json:"error,omitempty"`
}

// ResponseType 响应类型
type ResponseType int

const (
    ResponseTypeTimeout      ResponseType = iota // 超时
    ResponseTypeEchoReply                        // Echo Reply（到达目标）
    ResponseTypeTimeExceeded                     // TTL 超时（中间节点）
    ResponseTypeDestUnreach                      // 目标不可达
)

func (t ResponseType) String() string {
    switch t {
    case ResponseTypeTimeout:
        return "timeout"
    case ResponseTypeEchoReply:
        return "echo_reply"
    case ResponseTypeTimeExceeded:
        return "time_exceeded"
    case ResponseTypeDestUnreach:
        return "dest_unreachable"
    default:
        return "unknown"
    }
}
```

### 2. GeoResolver 接口

IP 地理位置解析器接口。

```go
package geoip

import "net"

// GeoResolver IP 地理位置解析器接口
type GeoResolver interface {
    // Resolve 解析 IP 地址的地理位置
    // 如果无法解析，返回空的 GeoLocation（不返回错误）
    Resolve(ip net.IP) *GeoLocation

    // Source 返回数据源名称
    Source() string

    // Close 关闭解析器，释放资源
    Close() error
}

// GeoLocation 地理位置信息
type GeoLocation struct {
    Country  string `json:"country,omitempty"`  // 国家
    Province string `json:"province,omitempty"` // 省份/州
    City     string `json:"city,omitempty"`     // 城市
    ISP      string `json:"isp,omitempty"`      // 运营商
    Source   string `json:"source,omitempty"`   // 数据来源
}

// String 返回格式化的位置字符串
func (g *GeoLocation) String() string {
    if g == nil {
        return ""
    }

    parts := make([]string, 0, 4)
    if g.Country != "" && g.Country != "0" {
        parts = append(parts, g.Country)
    }
    if g.Province != "" && g.Province != "0" {
        parts = append(parts, g.Province)
    }
    if g.City != "" && g.City != "0" {
        parts = append(parts, g.City)
    }
    if g.ISP != "" && g.ISP != "0" {
        parts = append(parts, g.ISP)
    }

    return strings.Join(parts, " ")
}

// IsEmpty 检查是否为空
func (g *GeoLocation) IsEmpty() bool {
    return g == nil || (g.Country == "" && g.Province == "" && g.City == "" && g.ISP == "")
}
```

### 3. Controller 接口

MTR 控制器，协调探测和统计。

```go
package mtr

import (
    "context"
)

// Controller MTR 控制器
type Controller struct {
    config   *Config
    prober   Prober
    resolver geoip.GeoResolver
    hops     []*Hop
    events   chan Event
}

// Config MTR 配置
type Config struct {
    Target      string        `json:"target"`       // 目标主机名或 IP
    TargetIP    net.IP        `json:"target_ip"`    // 解析后的目标 IP
    MaxHops     int           `json:"max_hops"`     // 最大跳数
    Count       int           `json:"count"`        // 探测次数（0=无限）
    Interval    time.Duration `json:"interval"`     // 探测间隔
    Timeout     time.Duration `json:"timeout"`      // 单次超时
    Protocol    Protocol      `json:"protocol"`     // 探测协议
    PacketSize  int           `json:"packet_size"`  // 包大小
    IPVersion   int           `json:"ip_version"`   // IP 版本（4/6）
}

// Protocol 探测协议
type Protocol string

const (
    ProtocolICMP Protocol = "icmp"
    ProtocolUDP  Protocol = "udp"
)

// NewController 创建控制器
func NewController(cfg *Config, resolver geoip.GeoResolver) (*Controller, error)

// Run 开始探测（阻塞直到完成或取消）
func (c *Controller) Run(ctx context.Context) error

// Events 返回事件通道，用于实时获取更新
func (c *Controller) Events() <-chan Event

// Hops 返回当前所有跳数信息（快照）
func (c *Controller) Hops() []*Hop

// Stop 停止探测
func (c *Controller) Stop()
```

### 4. Event 事件系统

用于实时通知 UI 更新。

```go
package mtr

// Event 事件接口
type Event interface {
    Type() EventType
}

// EventType 事件类型
type EventType int

const (
    EventTypeHopUpdate   EventType = iota // 跳数更新
    EventTypeProbeStart                   // 探测开始
    EventTypeProbeDone                    // 探测完成
    EventTypeError                        // 错误
)

// HopUpdateEvent 跳数更新事件
type HopUpdateEvent struct {
    TTL int
    Hop *Hop
}

func (e HopUpdateEvent) Type() EventType { return EventTypeHopUpdate }

// ProbeStartEvent 探测开始事件
type ProbeStartEvent struct {
    Round int
}

func (e ProbeStartEvent) Type() EventType { return EventTypeProbeStart }

// ProbeDoneEvent 探测完成事件
type ProbeDoneEvent struct {
    Round int
}

func (e ProbeDoneEvent) Type() EventType { return EventTypeProbeDone }

// ErrorEvent 错误事件
type ErrorEvent struct {
    Err error
}

func (e ErrorEvent) Type() EventType { return EventTypeError }
```

### 5. Hop 数据结构

单跳信息及统计。

```go
package mtr

import (
    "net"
    "sync"
    "time"
)

// Hop 路由跳数信息
type Hop struct {
    mu       sync.RWMutex
    TTL      int              `json:"ttl"`
    IP       net.IP           `json:"ip,omitempty"`
    Hostname string           `json:"hostname,omitempty"`
    Location *geoip.GeoLocation `json:"location,omitempty"`
    Stats    HopStats         `json:"stats"`
    Final    bool             `json:"final"` // 是否是最终目标
}

// HopStats 统计数据
type HopStats struct {
    Sent    int           `json:"sent"`              // 发送包数
    Recv    int           `json:"recv"`              // 接收包数
    Loss    float64       `json:"loss"`              // 丢包率 (0-100)
    Last    time.Duration `json:"last,omitempty"`    // 最近 RTT
    Best    time.Duration `json:"best,omitempty"`    // 最小 RTT
    Worst   time.Duration `json:"worst,omitempty"`   // 最大 RTT
    Avg     time.Duration `json:"avg,omitempty"`     // 平均 RTT
    StdDev  time.Duration `json:"std_dev,omitempty"` // 标准差
    History []time.Duration `json:"-"`               // RTT 历史（不序列化）
}

// Update 更新统计数据
func (h *Hop) Update(result *ProbeResult) {
    h.mu.Lock()
    defer h.mu.Unlock()

    h.Stats.Sent++

    if result.Type == ResponseTypeTimeout {
        h.recalcLoss()
        return
    }

    // 更新 IP（可能会变化）
    if result.IP != nil {
        h.IP = result.IP
    }

    h.Stats.Recv++
    h.Stats.Last = result.RTT

    // 更新最值
    if h.Stats.Best == 0 || result.RTT < h.Stats.Best {
        h.Stats.Best = result.RTT
    }
    if result.RTT > h.Stats.Worst {
        h.Stats.Worst = result.RTT
    }

    // 更新历史并计算统计
    h.Stats.History = append(h.Stats.History, result.RTT)
    h.recalcStats()
}

func (h *Hop) recalcLoss() {
    if h.Stats.Sent > 0 {
        h.Stats.Loss = float64(h.Stats.Sent-h.Stats.Recv) / float64(h.Stats.Sent) * 100
    }
}

func (h *Hop) recalcStats() {
    h.recalcLoss()

    if len(h.Stats.History) == 0 {
        return
    }

    // 计算平均值
    var sum time.Duration
    for _, rtt := range h.Stats.History {
        sum += rtt
    }
    h.Stats.Avg = sum / time.Duration(len(h.Stats.History))

    // 计算标准差
    var variance float64
    for _, rtt := range h.Stats.History {
        diff := float64(rtt - h.Stats.Avg)
        variance += diff * diff
    }
    variance /= float64(len(h.Stats.History))
    h.Stats.StdDev = time.Duration(math.Sqrt(variance))
}

// Snapshot 返回当前状态的快照
func (h *Hop) Snapshot() Hop {
    h.mu.RLock()
    defer h.mu.RUnlock()

    return Hop{
        TTL:      h.TTL,
        IP:       h.IP,
        Hostname: h.Hostname,
        Location: h.Location,
        Stats:    h.Stats,
        Final:    h.Final,
    }
}
```

## JSON 输出格式

### 完整输出

```json
{
  "target": "google.com",
  "target_ip": "142.250.190.78",
  "timestamp": "2024-01-15T10:30:00Z",
  "config": {
    "max_hops": 30,
    "count": 10,
    "interval": "1s",
    "timeout": "1s",
    "protocol": "icmp"
  },
  "hops": [
    {
      "ttl": 1,
      "ip": "192.168.1.1",
      "hostname": "router.local",
      "location": {
        "country": "局域网",
        "source": "ip2region"
      },
      "stats": {
        "sent": 10,
        "recv": 10,
        "loss": 0,
        "last": 1200000,
        "best": 1000000,
        "worst": 2100000,
        "avg": 1500000,
        "std_dev": 300000
      }
    },
    {
      "ttl": 2,
      "ip": null,
      "stats": {
        "sent": 10,
        "recv": 0,
        "loss": 100
      }
    },
    {
      "ttl": 3,
      "ip": "183.232.56.1",
      "location": {
        "country": "中国",
        "province": "广东",
        "city": "深圳",
        "isp": "电信",
        "source": "ip2region"
      },
      "stats": {
        "sent": 10,
        "recv": 10,
        "loss": 0,
        "last": 12400000,
        "best": 11800000,
        "worst": 15300000,
        "avg": 13200000,
        "std_dev": 1100000
      }
    }
  ]
}
```

## CLI 使用示例

### 基本用法

```bash
# 默认 TUI 模式
mymtr google.com

# 指定探测次数后退出
mymtr -c 10 google.com

# 使用 UDP 模式（无需 root）
mymtr -p udp google.com

# JSON 输出
mymtr --json -c 5 google.com

# 禁用 TUI，简单输出
mymtr --no-tui google.com

# 指定 IP 数据库
mymtr --geoip geoip2 --geoip-db /path/to/GeoLite2-City.mmdb google.com

# IPv6
mymtr -6 ipv6.google.com
```

### 输出示例（--no-tui）

```
MyMTR - google.com (142.250.190.78)
=====================================================

 #  Host                     Loss%  Snt  Last   Avg  Best  Wrst StDev  Location
 1  192.168.1.1               0.0%   10   1.2   1.5   1.0   2.1   0.3  局域网
 2  10.0.0.1                  0.0%   10   5.3   5.8   4.2   8.1   1.2  内网
 3  183.232.56.1              0.0%   10  12.4  13.2  11.8  15.3   1.1  中国 广东 深圳 电信
 4  202.97.94.150            10.0%   10  25.6  28.4  24.1  35.2   3.5  中国 广东 广州 电信
 5  ???                     100.0%   10     -     -     -     -     -
 6  72.14.222.136             0.0%   10  45.2  46.8  44.1  52.3   2.8  美国 Google
 7  142.250.190.78            0.0%   10  48.5  49.2  47.8  53.1   1.5  美国 Google
```

## 错误码

```go
package mtr

import "errors"

var (
    // ErrPermissionDenied 权限不足
    ErrPermissionDenied = errors.New("permission denied: need root or CAP_NET_RAW")

    // ErrInvalidTarget 无效目标
    ErrInvalidTarget = errors.New("invalid target: cannot resolve hostname")

    // ErrTimeout 全局超时
    ErrTimeout = errors.New("operation timeout")

    // ErrNetworkUnreachable 网络不可达
    ErrNetworkUnreachable = errors.New("network unreachable")

    // ErrGeoIPNotFound IP 数据库未找到
    ErrGeoIPNotFound = errors.New("geoip database not found")
)
```

## 使用示例（作为库）

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/yourname/mymtr/internal/geoip"
    "github.com/yourname/mymtr/internal/mtr"
)

func main() {
    // 初始化 GeoIP 解析器
    resolver, err := geoip.NewIP2RegionResolver("data/ip2region.xdb")
    if err != nil {
        panic(err)
    }
    defer resolver.Close()

    // 创建配置
    cfg := &mtr.Config{
        Target:   "google.com",
        MaxHops:  30,
        Count:    10,
        Interval: time.Second,
        Timeout:  time.Second,
        Protocol: mtr.ProtocolICMP,
    }

    // 创建控制器
    ctrl, err := mtr.NewController(cfg, resolver)
    if err != nil {
        panic(err)
    }

    // 监听事件
    go func() {
        for event := range ctrl.Events() {
            switch e := event.(type) {
            case mtr.HopUpdateEvent:
                hop := e.Hop
                fmt.Printf("TTL %d: %s (%s) - %.2fms\n",
                    hop.TTL, hop.IP, hop.Location,
                    float64(hop.Stats.Avg)/float64(time.Millisecond))
            }
        }
    }()

    // 运行
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := ctrl.Run(ctx); err != nil {
        panic(err)
    }

    // 输出最终结果
    for _, hop := range ctrl.Hops() {
        fmt.Printf("%d. %s - Loss: %.1f%%, Avg: %v, Location: %s\n",
            hop.TTL, hop.IP, hop.Stats.Loss, hop.Stats.Avg, hop.Location)
    }
}
```
