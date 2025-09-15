package global

import (
	"hash/crc32"
	"strconv"
)

// GetTopicKey 根据目标网关ID / 租户ID / 区域 等生成 Kafka 的 key
// 你可以只用 gatewayID，也可以加前缀防止跨域冲突。
func GetTopicKey(tenantID, gatewayID string) string {

	// 推荐形式: "tenant:gateway"
	return tenantID + ":" + gatewayID
}

// HashPartition 如果你只想要数值型分区，也可以直接算 hash
func HashPartition(key string, numPartitions int) int32 {
	checksum := crc32.ChecksumIEEE([]byte(key))
	return int32(checksum % uint32(numPartitions))
}

func ExampleKey(tenantID, gatewayID string, numPartitions int) (string, int32) {
	key := GetTopicKey(tenantID, gatewayID)
	part := HashPartition(key, numPartitions)
	return key, part
}

// TopicKeyGateway 如果只要 topic key 字符串
func TopicKeyGateway(gatewayID string) string {
	return "gw:" + gatewayID
}

// TopicKeyUser 如果需要更细粒度（按用户）
func TopicKeyUser(tenantID, userID string) string {
	return "user:" + tenantID + ":" + userID
}

// TopicKeySeq 如果要顺便返回 seq 对应的分片
func TopicKeySeq(convID string, seq int64, numPartitions int) (string, int32) {
	key := convID + ":" + strconv.FormatInt(seq, 10)
	return key, HashPartition(key, numPartitions)
}
