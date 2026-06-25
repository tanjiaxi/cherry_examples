/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-15 22:06:17
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-25 14:13:22
 * @FilePath: /examples/vendor/github.com/cherry-game/cherry/net/cluster/nats_cluster/const.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cherryNatsCluster

import (
	"fmt"
)

const (
	localSubjectFormat      = "cherry-%s.local.%s.%s"   // cherry.{prefix}.local.{nodeType}.{nodeID}
	remoteSubjectFormat     = "cherry-%s.remote.%s.%s"  // cherry.{prefix}.remote.{nodeType}.{nodeID}
	remoteTypeSubjectFormat = "cherry-%s.remoteType.%s" // cherry.{prefix}.remoteType.{nodeType}
	replySubjectFormat      = "cherry-%s.reply.%s.%s"   // cherry.{prefix}.reply.{nodeType}.{nodeID}

)

// GetLocalSubject local message nats chan
func GetLocalSubject(prefix, nodeType, nodeID string) string {
	return fmt.Sprintf(localSubjectFormat, prefix, nodeType, nodeID)
}

// GetRemoteSubject remote message nats chan
func GetRemoteSubject(prefix, nodeType, nodeID string) string {
	return fmt.Sprintf(remoteSubjectFormat, prefix, nodeType, nodeID)
}

func GetRemoteTypeSubject(prefix, nodeType string) string {
	return fmt.Sprintf(remoteTypeSubjectFormat, prefix, nodeType)
}

func GetReplySubject(prefix, nodeType, nodeID string) string {
	return fmt.Sprintf(replySubjectFormat, prefix, nodeType, nodeID)
}
