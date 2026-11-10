package printing

import "fmt"

type cupsSize struct {
	Media                    [128]byte
	Width, Length            int32
	Bottom, Left, Right, Top int32
}

func cupsDeviceMetrics(media cupsSize) (DeviceMetrics, error) {
	if media.Width <= 0 || media.Length <= 0 || media.Left < 0 || media.Top < 0 || media.Right < 0 || media.Bottom < 0 ||
		int64(media.Left)+int64(media.Right) >= int64(media.Width) || int64(media.Top)+int64(media.Bottom) >= int64(media.Length) {
		return DeviceMetrics{}, fmt.Errorf("printing: CUPS printer has no printable area")
	}
	const pointsPerHundredthMM = 72.0 / 2540
	return DeviceMetrics{
		PaperWidth: float64(media.Width) * pointsPerHundredthMM, PaperHeight: float64(media.Length) * pointsPerHundredthMM,
		PhysicalMargins: Insets{
			Left: float64(media.Left) * pointsPerHundredthMM, Top: float64(media.Top) * pointsPerHundredthMM,
			Right: float64(media.Right) * pointsPerHundredthMM, Bottom: float64(media.Bottom) * pointsPerHundredthMM,
		},
	}, nil
}
