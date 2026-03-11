package m3u8_parser

import (
	"testing"
)

// ============ 场景1: m3u8 中直接有分片 ============

func TestParseFromContent_MediaPlaylist(t *testing.T) {
	content := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
segment1.ts
#EXTINF:10.0,
segment2.ts
#EXTINF:5.0,
segment3.ts
#EXT-X-ENDLIST`

	parser := New()
	result, err := parser.ParseFromContent(content)
	if err != nil {
		t.Fatalf("ParseFromContent failed: %v", err)
	}

	// 验证类型
	if result.Type != PlaylistTypeMedia {
		t.Errorf("Expected type media, got %s", result.Type)
	}

	// 验证分片数量
	if len(result.Segments) != 3 {
		t.Errorf("Expected 3 segments, got %d", len(result.Segments))
	}

	// 验证分片URL
	expectedURLs := []string{"segment1.ts", "segment2.ts", "segment3.ts"}
	for i, seg := range result.Segments {
		if seg.URI != expectedURLs[i] {
			t.Errorf("Segment[%d] expected %s, got %s", i, expectedURLs[i], seg.URI)
		}
	}

	// 验证分片时长
	if result.Segments[0].Duration != 10.0 {
		t.Errorf("Segment[0] duration expected 10.0, got %f", result.Segments[0].Duration)
	}
}

func TestParseFromContent_WithQueryParams(t *testing.T) {
	// 测试场景：分片URL带query参数（校验参数）
	content := `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:10.0,
segment1.ts?token=abc123&expire=1700000000
#EXTINF:10.0,
segment2.ts?token=abc123&expire=1700000000
#EXTINF:5.0,
segment3.ts
#EXT-X-ENDLIST`

	parser := New()
	result, err := parser.ParseFromContent(content)
	if err != nil {
		t.Fatalf("ParseFromContent failed: %v", err)
	}

	// 验证 query 参数完整保留
	if result.Segments[0].URI != "segment1.ts?token=abc123&expire=1700000000" {
		t.Errorf("Query params not preserved: %s", result.Segments[0].URI)
	}

	if result.Segments[1].URI != "segment2.ts?token=abc123&expire=1700000000" {
		t.Errorf("Query params not preserved: %s", result.Segments[1].URI)
	}

	// 没有 query 的分片
	if result.Segments[2].URI != "segment3.ts" {
		t.Errorf("Expected segment3.ts, got %s", result.Segments[2].URI)
	}
}

func TestParseFromContent_WithEncryption(t *testing.T) {
	// 测试场景：加密的 m3u8
	content := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-KEY:METHOD=AES-128,URI="https://example.com/key",IV=0x12345678901234567890123456789012
#EXTINF:10.0,
encrypted_segment1.ts
#EXTINF:10.0,
encrypted_segment2.ts
#EXT-X-ENDLIST`

	parser := New()
	result, err := parser.ParseFromContent(content)
	if err != nil {
		t.Fatalf("ParseFromContent failed: %v", err)
	}

	// 验证加密信息
	if result.Encryption == nil {
		t.Fatal("Encryption info should not be nil")
	}

	if result.Encryption.Method != "AES-128" {
		t.Errorf("Expected method AES-128, got %s", result.Encryption.Method)
	}

	if result.Encryption.Key != "https://example.com/key" {
		t.Errorf("Expected key URL https://example.com/key, got %s", result.Encryption.Key)
	}
}

// ============ 场景2: Master Playlist (嵌套 m3u8) ============

func TestParseFromContent_MasterPlaylist(t *testing.T) {
	// 测试场景：master playlist 包含多个码率
	content := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1280000,RESOLUTION=720x480
playlist_480p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2560000,RESOLUTION=1280x720
playlist_720p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5120000,RESOLUTION=1920x1080
playlist_1080p.m3u8`

	parser := New()
	result, err := parser.ParseFromContent(content)
	if err != nil {
		t.Fatalf("ParseFromContent failed: %v", err)
	}

	// 验证类型
	if result.Type != PlaylistTypeMaster {
		t.Errorf("Expected type master, got %s", result.Type)
	}

	// 验证变体数量
	if len(result.Variants) != 3 {
		t.Errorf("Expected 3 variants, got %d", len(result.Variants))
	}

	// 验证变体信息
	variants := []struct {
		bandwidth  int
		resolution string
	}{
		{1280000, "720x480"},
		{2560000, "1280x720"},
		{5120000, "1920x1080"},
	}

	for i, v := range variants {
		if result.Variants[i].Bandwidth != v.bandwidth {
			t.Errorf("Variant[%d] bandwidth expected %d, got %d", i, v.bandwidth, result.Variants[i].Bandwidth)
		}
		if result.Variants[i].Resolution != v.resolution {
			t.Errorf("Variant[%d] resolution expected %s, got %s", i, v.resolution, result.Variants[i].Resolution)
		}
	}
}

func TestParseFromContent_MasterPlaylistWithCodecs(t *testing.T) {
	// 测试场景：master playlist 包含 CODECS 信息
	content := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2560000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
playlist_720p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5120000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
playlist_1080p.m3u8`

	parser := New()
	result, err := parser.ParseFromContent(content)
	if err != nil {
		t.Fatalf("ParseFromContent failed: %v", err)
	}

	if len(result.Variants) != 2 {
		t.Fatalf("Expected 2 variants, got %d", len(result.Variants))
	}

	if result.Variants[0].Codecs != "avc1.64001f,mp4a.40.2" {
		t.Errorf("Codecs not parsed: %s", result.Variants[0].Codecs)
	}
}

// ============ 工具函数测试 ============

func TestResolveURL(t *testing.T) {
	tests := []struct {
		baseURL  string
		relative string
		expected string
	}{
		{"https://example.com/video/", "segment1.ts", "https://example.com/video/segment1.ts"},
		{"https://example.com/video/", "/segment1.ts", "https://example.com/segment1.ts"},
		{"https://example.com/video/", "https://cdn.example.com/segment1.ts", "https://cdn.example.com/segment1.ts"},
		{"", "segment1.ts", "segment1.ts"},
	}

	for _, tt := range tests {
		result := resolveURL(tt.baseURL, tt.relative)
		if result != tt.expected {
			t.Errorf("resolveURL(%s, %s) = %s; want %s", tt.baseURL, tt.relative, result, tt.expected)
		}
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		line     string
		key      string
		expected string
	}{
		{`#EXT-X-KEY:METHOD=AES-128,URI="https://example.com/key"`, "URI", "https://example.com/key"},
		{`#EXT-X-STREAM-INF:BANDWIDTH=1280000,RESOLUTION=720x480`, "RESOLUTION", "720x480"},
		{`#EXT-X-STREAM-INF:CODECS="avc1.64001f,mp4a.40.2"`, "CODECS", "avc1.64001f,mp4a.40.2"},
	}

	for _, tt := range tests {
		result := extractString(tt.line, tt.key)
		if result != tt.expected {
			t.Errorf("extractString(%s, %s) = %s; want %s", tt.line, tt.key, result, tt.expected)
		}
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		line     string
		key      string
		expected int
	}{
		{`#EXT-X-STREAM-INF:BANDWIDTH=1280000`, "BANDWIDTH", 1280000},
		{`#EXT-X-VERSION:3`, "VERSION", 3},
		{`#EXT-X-TARGETDURATION:10`, "TARGETDURATION", 10},
	}

	for _, tt := range tests {
		result := extractInt(tt.line, tt.key)
		if result != tt.expected {
			t.Errorf("extractInt(%s, %s) = %d; want %d", tt.line, tt.key, result, tt.expected)
		}
	}
}
