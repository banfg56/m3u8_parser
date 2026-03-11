# m3u8 - HLS M3U8 解析库

一个 Go 语言编写的 M3U8 解析库，支持解析 M3U8 文件/URL，获取所有推流分片地址。

## 功能特性

- ✅ 支持 Master Playlist（多码率）和 Media Playlist（单一码率）
- ✅ 自动递归获取所有码率变体的分片
- ✅ 完整保留分片 URL 中的 query 参数（如 token、auth_key 等）
- ✅ 支持加密流解析（EXT-X-KEY）
- ✅ 支持从 URL、文件、内容三种方式解析
- ✅ 完善的测试用例

## 安装

```bash
go get github.com/banfg56/m3u8_parser
```

## 快速开始

### 作为库引入

```go
import m3u8 "github.com/banfg56/m3u8_parser"

func main() {
    parser := m3u8.New()

    // 方式1: 从 URL 解析
    result, err := parser.ParseFromURL("https://example.com/video.m3u8")

    // 方式2: 从文件解析
    result, err := parser.ParseFromFile("/path/to/video.m3u8")

    // 方式3: 从内容解析
    result, err := parser.ParseFromContent(m3u8Content)

    // 获取带分片的变体（推荐用于 Master Playlist）
    result, err := parser.GetVariantSegments("https://example.com/video.m3u8")

    // 获取所有分片 URL
    urls, err := parser.GetAllSegmentURLs("https://example.com/video.m3u8")
}
```

### 数据结构

#### ParseResult - 解析结果

```go
type ParseResult struct {
	Type       PlaylistType // master 或 media
	URI        string       // 原始地址
	Version    int          // EXT-X-VERSION
	TargetDuration int      // EXT-X-TARGETDURATION
	Segments   []Segment    // 媒体分片列表
	Variants   []Variant    // 多码率列表（仅master）
	Encryption *Encryption // 加密信息
}
```

#### Variant - 码率变体

```go
// Variant 码率变体（master playlist）
type Variant struct {
	Bandwidth  int        // 带宽 (bps)
	Resolution string     // 分辨率
	URI        string     // 媒体播放列表地址
	Codecs     string     // 编解码器信息
	Segments   []Segment  // 该码率对应的分片列表
}
```

#### Segment - 分片信息

```go
type Segment struct {
    Index     int     // 分片索引
    URI       string  // 分片完整URL（含query参数）
    Title     string  // EXTINF 标题
    Duration  float64 // 时长（秒）
}
```

### CLI 使用

```bash
# 编译
go build -o m3u8_parser ./cmd/main.go

# 查看帮助
./m3u8_parser -h

# 从 URL 解析
./m3u8_parser -u https://example.com/video.m3u8

# 从文件解析
./m3u8_parser -f /path/to/video.m3u8

# 按画质过滤 (480p, 720p, 1080p)
./m3u8_parser -u https://example.com/video.m3u8 -q 720p

# 按码率过滤 (kbps)
./m3u8_parser -u https://example.com/video.m3u8 -b 1200

# 输出分片 URL 列表
./m3u8_parser -u https://example.com/video.m3u8 -l

# 输出到 m3u8 文件
./m3u8_parser -u https://example.com/video.m3u8 -o output.m3u8
```

## 自动化测试

### 运行测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试
go test -v -run TestParseFromContent_MediaPlaylist
```

### 测试用例说明

| 测试用例 | 说明 |
|----------|------|
| `TestParseFromContent_MediaPlaylist` | 解析标准 Media Playlist |
| `TestParseFromContent_WithQueryParams` | 验证 query 参数保留 |
| `TestParseFromContent_WithEncryption` | 解析加密流 |
| `TestParseFromContent_MasterPlaylist` | 解析 Master Playlist |
| `TestParseFromContent_MasterPlaylistWithCodecs` | 解析带 CODECS 信息的 Master Playlist |
| `TestResolveURL` | URL 解析工具函数 |
| `TestExtractString` | 字符串提取工具函数 |
| `TestExtractInt` | 数值提取工具函数 |

## M3U8 协议参考

### 官方文档

- **RFC 8216** - HTTP Live Streaming (HLS): https://datatracker.ietf.org/doc/html/rfc8216
- **Apple HLS Documentation**: https://developer.apple.com/documentation/http-live-streaming

### 协议变更关注

1. **IETF RFC 8216**: https://datatracker.ietf.org/doc/html/rfc8216
   - 关注 HTTP Live Streaming 标准版本

2. **Apple 开发者文档**: https://developer.apple.com/documentation/http-live-streaming
   - 关注 Apple 对 HLS 的实现细节和扩展

3. **GitHub HLS 相关项目**:
   - https://github.com/topics/hls
   - https://github.com/video-dev/hls.js

## 版本兼容性策略

### 当前支持的特性

| 特性 | HLS 版本 | 说明 |
|------|----------|------|
| Basic Playlist | v3+ | 基础播放列表支持 |
| Master Playlist | v3+ | 多码率支持 |
| EXT-X-KEY (AES-128) | v4+ | 加密支持 |
| EXT-X-STREAM-INF | v3+ | 码率变体信息 |
| EXT-X-TARGETDURATION | v3+ | 目标时长 |
| EXT-X-VERSION | v3+ | 协议版本 |

### 兼容性处理

1. **版本检测**: 程序会解析 `#EXT-X-VERSION` 标签，支持版本 3-7

2. **向后兼容**:
   - 新版本解析器会尝试兼容旧版本格式
   - 不支持的标签会被忽略（以 `#` 开头的行会被跳过）

3. **扩展支持**:
   - 库会持续更新以支持新的 HLS 特性
   - 建议关注 Release Notes 获取更新

### 升级注意事项

当 HLS 协议有新版本发布时：

1. 检查新版本协议与当前实现的兼容性
2. 更新 `Version` 字段的解析逻辑（如需要）
3. 添加新的测试用例验证
4. 在 Release Notes 中记录变更

## 项目结构

```
m3u8/
├── m3u8.go           # 核心解析库
├── m3u8_test.go      # 测试用例
├── go.mod            # Go 模块定义
├── cmd/
│   └── main.go      # CLI 入口
└── README.md         # 本文档
```

## License

MIT License
