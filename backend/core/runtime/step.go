package runtime

import "context"

// BashCmdStep 是 R2 阶段的 Step 逃生舱:
// 把整段 bash 命令裹成一个 Step,Do/Undo 暂未接通 stepengine,
// 由 usecase/deploy 直接取出 Cmd() 字符串通过 pkg/runner 执行。
//
// R6 改造:
//   - 替换为多个细粒度 Step(PullImageStep / RunContainerStep / ...)
//   - Do/Undo 真正接通 stepengine,并删除本类型的 panic 路径
type BashCmdStep struct {
	StepName string
	Command  string
}

// Name 返回 Step 的展示名(日志 / UI 用)。
func (s *BashCmdStep) Name() string { return s.StepName }

// Cmd 返回底层 bash 命令字符串(R2 仅供 usecase/deploy 使用)。
func (s *BashCmdStep) Cmd() string { return s.Command }

// Do 在 R2 不被任何调用方触发(usecase 走 Cmd() 直执路径)。
// R6 接通 stepengine 后将真正执行。
func (s *BashCmdStep) Do(_ context.Context) error {
	panic("runtime.BashCmdStep.Do: not wired in R2 — call Cmd() and execute via runner; stepengine arrives in R6")
}

// Undo 同 Do,R2 占位。
func (s *BashCmdStep) Undo(_ context.Context) error {
	panic("runtime.BashCmdStep.Undo: not wired in R2 — see R6")
}
