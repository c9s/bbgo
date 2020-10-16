package slackstyle

const Green = "#228B22"
const Red = "#800000"

func TrendIcon(trend int) string {
	if trend < 0 {
		return ":chart_with_downwards_trend:"
	} else if trend > 0 {
		return ":chart_with_upwards_trend:"
	}
	return ""
}
