package nginxops

import "sync"

// edgeLocks 维护按 edge_server_id 区分的互斥锁，确保同一台 edge 上同时
// 至多只有一次 Reconciler.Apply 在写远端文件 / reload nginx。
//
// 实现细节：sync.Map + LoadOrStore 保证 lock 自身的并发安全；返回的 release
// 必须在持锁分支的 defer 中调用。Acquire 阻塞至取得锁。
var edgeLocks sync.Map // map[uint]*sync.Mutex

// Acquire 取得 edge 的互斥锁，返回释放回调。调用方必须 defer release()。
func Acquire(edgeID uint) func() {
	v, _ := edgeLocks.LoadOrStore(edgeID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
