// 提供一个协程池接口
package gpool

// 协程池接口
type Pool interface {
	// 提交任务
	Submit(f func()) error
	// 动态调整池大小
	Tune(size int)
	// 运行中的实例个数
	Running() int
	// 空闲空间大小
	Free() int
	// 池总大小
	Cap() int
}
