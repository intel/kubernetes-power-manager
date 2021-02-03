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

func AdjustCpuFrequency(coreList []string, freq int, freqPath string) error {
	realFreqValue := freq * 1000
	realFreqValueStr := strconv.Itoa(realFreqValue)
	for _, coreID := range coreList {
		maxFreqPath := fmt.Sprintf("%s%s%s", BASE_PATH, coreID, freqPath)

		err := ioutil.WriteFile(maxFreqPath, []byte(realFreqValueStr), 0064)
		if err != nil {
			return err
		}
	}

	return nil
}
