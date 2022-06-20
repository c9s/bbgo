package slackstyle

// Green is the green hex color
const Green = "#228B22"

// Red is the red hex color
const Red = "#800000"

// TrendIcon returns the slack emoji of trends
// 1: uptrend
// -1: downtrend
func TrendIcon(trend int) string {
	if trend < 0 {
		return ":chart_with_downwards_trend:"
	} else if trend > 0 {
		return ":chart_with_upwards_trend:"
	}
	return ""
}
