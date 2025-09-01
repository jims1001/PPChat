package model

import "time"

// Object 表示一条对象存储元数据记录（文件/媒体/附件等）。
// 建议将 Hash 作为“内容指纹”（推荐 sha256），Key 作为“后端存储路径”。
type Object struct {
	TenantID string `bson:"tenant_id"` // PK
	// —— 基础标识 ——
	Name   string `bson:"name"`    // 原始文件名（上传时的文件名，含扩展名）
	UserID string `bson:"user_id"` // 拥有者/归属用户ID（可用于权限判断与分片）
	Group  string `bson:"group"`   // 业务分组：avatar/msg/cover/temp 等（便于策略控制）
	Engine string `bson:"engine"`  // 存储引擎：s3/minio/oss/cos/gcs/localfs...
	Bucket string `bson:"bucket"`  // 目标桶/容器名（S3/OSS等）
	Key    string `bson:"key"`     // 对象Key/路径（后端定位，唯一约束建议：bucket+key）

	// —— 完整性与大小 ——
	Hash        string `bson:"hash"`           // 内容指纹（推荐 sha256 hex；用于去重/秒传）
	ETag        string `bson:"etag,omitempty"` // 云厂商返回的 ETag（分片上传不一定等于MD5）
	Size        int64  `bson:"size"`           // 字节大小
	ContentType string `bson:"content_type"`   // MIME 类型（服务端嗅探校准，别完全信客户端）

	// —— 存储与访问控制 ——
	StorageClass string `bson:"storage_class,omitempty"` // standard/ia/archive...
	Region       string `bson:"region,omitempty"`        // 区域/机房标识（如 cn-north-1）
	ACL          int32  `bson:"acl"`                     // 访问级别: 0私有,1公开读,2签名链接
	Status       int32  `bson:"status"`                  // 0=上传中,1=可用,2=失败,3=归档,4=已删,5=隔离(审核/病毒)

	// —— 生命周期 ——
	CreateTime time.Time  `bson:"create_time"`                // 创建时间
	UpdateTime time.Time  `bson:"update_time"`                // 最后一次元数据更新
	LastAccess time.Time  `bson:"last_access_time,omitempty"` // 最近访问（用于冷热分层/清理）
	ExpireTime *time.Time `bson:"expire_time,omitempty"`      // 到期时间（TTL/一次性下载）
	DeleteTime *time.Time `bson:"delete_time,omitempty"`      // 软删除时间（与 Status=4 搭配）

	// —— 上传/分片信息（大文件/断点续传）——
	UploadSessionID string `bson:"upload_session_id,omitempty"` // 分片上传会话ID
	PartCount       int32  `bson:"part_count,omitempty"`        // 分片数量（便于校验合并）

	// —— 链接/派生资源 ——
	CDNURL     string `bson:"cdn_url,omitempty"`     // CDN 可公开访问的 URL（如 ACL=1）
	OriginURL  string `bson:"origin_url,omitempty"`  // 源站/直链（内部自检/回源）
	ThumbKey   string `bson:"thumb_key,omitempty"`   // 缩略图对象 Key（图片/视频封面）
	PreviewKey string `bson:"preview_key,omitempty"` // 预览版对象 Key（转码低清版、PDF预览等）

	// —— 媒体元信息（可选，根据类型填充）——
	Meta     ObjectMeta     `bson:"meta,omitempty"`     // 文件扩展信息（图片/音频/视频）
	Security ObjectSecurity `bson:"security,omitempty"` // 安全审计（病毒/内容审核）

	// —— 标签与引用计数 ——
	Tags     []string `bson:"tags,omitempty"` // 业务标签（消息附件/头像/客服工单附件等）
	RefCount int64    `bson:"ref_count"`      // 引用次数（被多少消息/实体引用，做去重回收）

	// —— 扩展 ——
	Ex string `bson:"ex"` // 预留扩展(JSON)
}

// ObjectMeta：多媒体文件的可选元信息（避免主表字段爆炸）
type ObjectMeta struct {
	FileExt string     `bson:"file_ext,omitempty"` // 扩展名（.jpg/.mp4），服务端根据Name/ContentType判定
	Image   *ImageMeta `bson:"image,omitempty"`
	Video   *VideoMeta `bson:"video,omitempty"`
	Audio   *AudioMeta `bson:"audio,omitempty"`
	Doc     *DocMeta   `bson:"doc,omitempty"` // 文档类（页数/可预览页）
}

type ImageMeta struct {
	Width  int32 `bson:"width"`
	Height int32 `bson:"height"`
	Orient int32 `bson:"orient,omitempty"` // EXIF 方向
}

type VideoMeta struct {
	Width      int32   `bson:"width"`
	Height     int32   `bson:"height"`
	DurationMs int64   `bson:"duration_ms"`
	Bitrate    int32   `bson:"bitrate,omitempty"`
	Fps        float32 `bson:"fps,omitempty"`
	Codec      string  `bson:"codec,omitempty"`
}

type AudioMeta struct {
	DurationMs int64  `bson:"duration_ms"`
	Bitrate    int32  `bson:"bitrate,omitempty"`
	SampleRate int32  `bson:"sample_rate,omitempty"`
	Channels   int32  `bson:"channels,omitempty"`
	Codec      string `bson:"codec,omitempty"`
}

type DocMeta struct {
	Pages int32 `bson:"pages,omitempty"` // 文档页数（PDF/Docx转预览）
}

// ObjectSecurity：安全与合规元信息
type ObjectSecurity struct {
	VirusScanStatus  int32  `bson:"virus_scan_status"` // 0待扫,1安全,2发现病毒(隔离)
	ModerationStatus int32  `bson:"moderation_status"` // 0待审,1通过,2拒绝,3需人工复核
	ChecksumCRC32    uint32 `bson:"checksum_crc32,omitempty"`
	SHA1             string `bson:"sha1,omitempty"`   // 如果 Hash 用 sha256，这里可存 sha1 兼容
	SHA256           string `bson:"sha256,omitempty"` // 与 Hash 一致时可省略，或用于双写迁移
}
