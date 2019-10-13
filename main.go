package main

import (
	"strconv"
	"fmt"
	"strings"
	"encoding/json"
	"time"
	"net/http"
	"github.com/rs/cors"
	"log"
	"os"
	"io/ioutil"
)

const (
	channelSize = 16
	channelPeriod = 1000000; // 1ms
)

var (
	channelsTarget []uint16;
	channelsCurrent []uint16;
	channelMapping map[uint16]uint16;
	fadeSpeed uint16;
)

func channelPath(channel uint16) string {
	return fmt.Sprintf("/sys/class/pwm/pwmchip0/pwm%d", channel)
}

func periodPath(channel uint16) string {
	return fmt.Sprintf("%s/period", channelPath(channel))
}

func enablePath(channel uint16) string {
	return fmt.Sprintf("%s/enable", channelPath(channel))
}

func dutyCyclePath(channel uint16) string {
	return fmt.Sprintf("%s/duty_cycle", channelPath(channel))
}

func setupPWM() {
	for _, channel := range channelMapping {
		_, err := os.Stat(channelPath(channel));
		if err != nil {
			if os.IsNotExist(err) {
				ioutil.WriteFile("/sys/class/pwm/pwmchip0/export", []byte(strconv.Itoa(int(channel))), 0644);
				log.Println("Writing ", channel, "/sys/class/pwm/pwmchip0/export");
			}
		}
		for {
			_, err = os.Stat(channelPath(channel));
			if err == nil || !os.IsNotExist(err) {
				break
			}
		}
		ioutil.WriteFile(periodPath(channel), []byte(strconv.Itoa(channelPeriod)), 0644);
		ioutil.WriteFile(enablePath(channel), []byte("1"), 0644);
	}
}

func setPWMChannels() {
	for channel, value := range channelsCurrent {
		duty_cycle := uint32(value/8)*channelPeriod/255;
		ioutil.WriteFile(dutyCyclePath(uint16(channel)), []byte(strconv.Itoa(int(duty_cycle))), 0644);
	}
}

func getPWMChannels() []uint16 {
	sysChannels := make([]uint16, len(channelsCurrent))
	for i :=0 ; i < len(channelsCurrent); i++ {
		periodContent, err := ioutil.ReadFile(periodPath(uint16(i)))
		if err != nil {
			log.Println("Could not read period of channel", i)
			continue;
		}
		period, err := strconv.Atoi(strings.TrimSpace(string(periodContent)))
		if err != nil {
			log.Println("Could not parse period of channel", i)
			continue;
		}
		dutyCycleContent, err := ioutil.ReadFile(dutyCyclePath(uint16(i)))
		if err != nil {
			log.Println("Could not read dutyCycle of channel", i)
			continue;
		}
		dutyCycle, err := strconv.Atoi(strings.TrimSpace(string(dutyCycleContent)))
		if err != nil {
			log.Println("Could not parse dutyCycle of channel", i)
			continue;
		}
		sysChannels[i] = uint16(4096 * dutyCycle / period);
	}
	return sysChannels;
}

func setChannel(channel uint16, value uint8) {
	actualChannel := channelMapping[channel];
	channelsTarget[actualChannel] = uint16(value)*8;
}

func getChannelsString() map[string][]string {
	response := make(map[string][]string);
	response["target"] = make([]string, channelSize);
	response["current"] = make([]string, channelSize);
	for idx, value := range channelsTarget {
		response["target"][idx] = strconv.Itoa(int(value/8))
	}
	for idx, value := range channelsCurrent {
		response["current"][idx] = strconv.Itoa(int(value/8))
	}
	return response;
}

func abs(x int32) uint16 {
	if x < 0 {
		return uint16(-1*x);
	}
	return uint16(x);
}

func min(x uint16, y uint16) uint16 {
	if x > y {
		return y;
	}
	return x;
}

func init() {
	channelsTarget = make([]uint16, channelSize);
	channelsCurrent = make([]uint16, channelSize);
	channelMapping = make(map[uint16]uint16);
	channelMapping[9] = 10;
	channelMapping[10] = 11;
	channelMapping[11] = 12;
	channelMapping[12] = 13;
	fadeSpeed = 100;

	for i := 0; i < channelSize; i++ {
		channelMapping[uint16(i)] = uint16(i);
	}
	for _, idx := range channelsCurrent {
		channelsCurrent[idx] = 0;
	}
	for _, idx := range channelsTarget {
		channelsTarget[idx] = 0;
	}
	setupPWM()
	for idx, value := range getPWMChannels() {
		channelsCurrent[idx] = value;
		channelsTarget[idx] = value;
	}
	log.Println("current", channelsCurrent)
	log.Println("target", channelsTarget)
}

func main() {
	timerDuration := time.Millisecond*30;
	timer := time.NewTimer(timerDuration);
	go func() {
		for {
			<-timer.C
			channelsDirty := false;
			for idx, value := range channelsTarget {
				if channelsCurrent[idx] != value {
					diff := abs(int32(value)-int32(channelsCurrent[idx]));
					if fadeSpeed == 0 {
						channelsCurrent[idx] = value;
					} else if channelsCurrent[idx] < value {
						channelsCurrent[idx] += min(fadeSpeed, diff);
					} else {
						channelsCurrent[idx] -= min(fadeSpeed, diff);
					}
					channelsDirty = true;
				}
			}
			if (channelsDirty) {
				setPWMChannels();
				channelsDirty = false;
			}
			timer.Reset(timerDuration);
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		for ks,vs := range values {
			channel, err := strconv.ParseUint(ks, 10, 16)
			if err != nil {
				log.Println("Key NaN");
				continue
			}
			value, err := strconv.ParseUint(vs[0], 10, 16)
			if err != nil {
				log.Println("Value NaN");
				continue
			}
			if channel > channelSize  {
				log.Println("Key out of Range");
				continue
			}
			if value > 255 {
				log.Println("Value out of Range");
				continue
			}
			log.Println(channel, value);
			setChannel(uint16(channel), uint8(value));
		}
	});
	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		jsonData, err := json.Marshal(getChannelsString());
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	});
	mux.HandleFunc("/fade", func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query();
		if len(values["speed"][0]) > 0 {
			value, err := strconv.ParseUint(values["speed"][0], 10, 16)
			if err != nil {
				log.Println("Value NaN");
				w.WriteHeader(400);
			}
			if value > 65536 {
				log.Println("Value out of range");
				w.WriteHeader(400);
			}
			log.Println("Setting value to", value);
			fadeSpeed = uint16(value);
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf("{ \"speed\": %d }", fadeSpeed)))
	});
	mux.Handle("/", http.FileServer(http.Dir("./frontend")));
	handler := cors.AllowAll().Handler(mux);
	http.ListenAndServe(":7777", handler);
}
