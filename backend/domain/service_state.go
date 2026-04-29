package domain

import "fmt"

// ServiceSyncState 定义 Service.SyncStatus 的合法状态。
const (
	ServiceSyncStateEmpty   = ""        // 初始,从未参与 reconcile
	ServiceSyncStateSyncing = "syncing" // reconcile 进行中(CAS 守卫)
	ServiceSyncStateSynced  = "synced"  // reconcile 成功
	ServiceSyncStateError   = "error"   // reconcile 失败
)

// validServiceSyncTransitions 定义 Service.SyncStatus 的合法迁移。
var validServiceSyncTransitions = map[string][]string{
	ServiceSyncStateEmpty:   {ServiceSyncStateSyncing, ServiceSyncStateSynced},
	ServiceSyncStateSyncing: {ServiceSyncStateSynced, ServiceSyncStateError},
	ServiceSyncStateSynced:  {ServiceSyncStateSyncing},
	ServiceSyncStateError:   {ServiceSyncStateSyncing},
}

// CanTransitionTo 校验 Service.SyncStatus 从 from 迁移到 to 是否合法。
func CanTransitionTo(from, to string) error {
	if from == to {
		return nil
	}
	allowed, ok := validServiceSyncTransitions[from]
	if !ok {
		return fmt.Errorf("service sync: 未知当前状态 %q", from)
	}
	for _, a := range allowed {
		if a == to {
			return nil
		}
	}
	return fmt.Errorf("service sync: 不允许从 %q 迁移到 %q", from, to)
}
