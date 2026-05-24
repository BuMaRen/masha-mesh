package breaker

import "github.com/spf13/cobra"

type Options struct {
	windowCapacity       int     // 窗口大小，单位为秒
	halfOpenAllowed      int     // 半开状态允许的请求数，超过后根据失败率决定是否切换到开状态
	minRequestCount      int     // 最小请求数，未达到该数值时不考虑切换到开状态
	failureRateThreshold float64 // 失败率阈值，超过该值时切换到开状态
	halfOpenMaxDuration  int64   // 半开状态的最大持续时间，超过该时间后切换到开状态，单位为秒
	openDuration         int64   // 开状态的持续时间，超过该时间后切换到半开状态，单位为秒
}

func NewOptions() *Options {
	return &Options{
		windowCapacity:       20,
		halfOpenAllowed:      10,
		minRequestCount:      20,
		failureRateThreshold: 0.5,
		halfOpenMaxDuration:  60,
		openDuration:         60,
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&o.windowCapacity, "window-size", o.windowCapacity, "窗口大小，单位为秒")
	cmd.Flags().IntVar(&o.halfOpenAllowed, "half-open-allowed", o.halfOpenAllowed, "半开状态允许的请求数，超过后根据失败率决定是否切换到开状态")
	cmd.Flags().IntVar(&o.minRequestCount, "min-request-count", o.minRequestCount, "最小请求数，未达到该数值时不考虑切换到开状态")
	cmd.Flags().Float64Var(&o.failureRateThreshold, "failure-rate-threshold", o.failureRateThreshold, "失败率阈值，超过该值时切换到开状态")
	cmd.Flags().Int64Var(&o.halfOpenMaxDuration, "half-open-max-duration", o.halfOpenMaxDuration, "半开状态的最大持续时间，超过该时间后切换到开状态，单位为秒")
	cmd.Flags().Int64Var(&o.openDuration, "open-duration", o.openDuration, "开状态的持续时间，超过该时间后切换到半开状态，单位为秒")
}
