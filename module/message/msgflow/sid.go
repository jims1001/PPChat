package msgflow

import "fmt"

// 辅助：构造会话ID
func P2PConvID(u1, u2 string) string {
	if u1 < u2 {
		return fmt.Sprintf("p2p:%s:%s", u1, u2)
	}
	return fmt.Sprintf("p2p:%s:%s", u2, u1)
}
func GroupConvID(gid string) string  { return "grp:" + gid }
func ThreadConvID(tid string) string { return "thread:" + tid }
