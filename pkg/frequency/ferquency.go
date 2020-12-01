package frequency

import (
	"fmt"
	"io/ioutil"
	"strconv"
)

const (
	BASE_PATH     = "/sys/devices/system/cpu/cpu"
	MAX_FREQ_PATH = "/cpufreq/scaling_max_freq"
	MIN_FREQ_PATH = "/cpufreq/scaling_min_freq"
)

func AdjustCpuMaxFrequency(coreID string, freq int) error {
	maxFreqPath := fmt.Sprintf("%s%s%s", BASE_PATH, coreID, MAX_FREQ_PATH)
	realFreqValue := freq * 1000
	realFreqValueStr := strconv.Itoa(realFreqValue)

	err := ioutil.WriteFile(maxFreqPath, []byte(realFreqValueStr), 0064)
	if err != nil {
		return err
	}

	return nil
}
