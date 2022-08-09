package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"time"
	"strings"
	"net/http"
	"github.com/hashicorp/go-version"

	"github.com/imdario/mergo"

	"github.com/liip/sheriff"

	"github.com/sirupsen/logrus"
	"strconv"
)

// Command line flag global variables
var debug bool
var configFile string


func checkFile(filename string) error {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		_, err := os.Create(filename)
		if err != nil {
			return err
		}
	}
	return nil
}




//NewStreamCore do load config file
func NewStreamCore() *StorageST {
	
	var tmp StorageST

	var (
		User     string
		Password string
		ServerID string
		Port     string
		Host     string
		Prefix   string
		Protocol string
		ApiOption bool
	)
	var VmsCamList []VmsCam
	tmp.Streams = make(map[string]StreamST)
	//LOAD DATA FROM CONFIG.JSON
	filename := "config.json"
	err := checkFile(filename)
	if err != nil {
		logrus.Error(err)
	}
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		logrus.Error(err)
	}
	json.Unmarshal(file, &tmp)
	for oldStream := range tmp.Streams {
		delete(tmp.Streams, oldStream)
	}
	
	//READ CONFIG .ENV
	lines, err := readLines(".env")
	if err != nil {
		log.Fatalf("readLines: %s", err)
	}

	for _, line := range lines {
		if strings.Contains(line, "WEBRTC_SERVICE_USER") {
			User = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_PASS") {
			Password = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_SERVER_ID") {
			ServerID = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_VMS_PORT") {
			Port = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_VMS_HOST") {
			Host = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_VMS_PREFIX") {
			Prefix = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_PROTOCOL") {
			Protocol = (strings.Split(line, "="))[1]
		}
		if strings.Contains(line, "WEBRTC_SERVICE_API_OPTION_LAN_NETWORK") {
			ApiOption, _ = strconv.ParseBool((strings.Split(line, "="))[1])
		}
	}

	//LOAD VMS CAMERA FROM API
	apiUrl := Protocol + "://" + Host + ":" + Port + "/" + Prefix + "/api/monitors"
	if (ApiOption) {
		//api cam LAN// 
		apiUrl += apiUrl + "?lan=1"
	}
	// fmt.Println("URL", apiUrl)
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiUrl, nil)
	req.SetBasicAuth(User, Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &VmsCamList)
	//UPDATE LIST CAMERA TO CONFIG.JSON
	for _, item := range VmsCamList {
		if item.ServerID == ServerID {
			var newStream StreamST
			newStream.Channels = make(map[string]ChannelST)
			newName := item.Id
			newStream.Name = newName
			for _, link := range item.Links {
				if link.Type == "rtsp" {
					for _, url := range link.Url {
						if url.Value != "" {
							var newChannels ChannelST
							newChannels.Name = url.Type
							newChannels.URL = url.Value
							newChannels.OnDemand = true
							newChannels.Debug = false
							newChannels.Status = 0
							newChannels.Audio = false
							newStream.Channels[url.Type] = newChannels
						}
					}
				}
			}
			tmp.Streams[newName] = newStream
		}
	}
	dataBytes, err := json.Marshal(tmp)
	if err != nil {
		logrus.Error(err)
	}

	err = ioutil.WriteFile(filename, dataBytes, 0644)
	if err != nil {
		logrus.Error(err)
	}

	flag.BoolVar(&debug, "debug", true, "set debug mode")
	flag.StringVar(&configFile, "config", "config.json", "config patch (/etc/server/config.json or config.json)")
	flag.Parse()
	
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "config",
			"func":   "NewStreamCore",
			"call":   "ReadFile",
		}).Errorln(err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(data, &tmp)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "config",
			"func":   "NewStreamCore",
			"call":   "Unmarshal",
		}).Errorln(err.Error())
		os.Exit(1)
	}
	debug = tmp.Server.Debug
	for i, i2 := range tmp.Streams {
		for i3, i4 := range i2.Channels {
			channel := tmp.ChannelDefaults
			err = mergo.Merge(&channel, i4)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module": "config",
					"func":   "NewStreamCore",
					"call":   "Merge",
				}).Errorln(err.Error())
				os.Exit(1)
			}
			channel.clients = make(map[string]ClientST)
			channel.ack = time.Now().Add(-255 * time.Hour)
			channel.hlsSegmentBuffer = make(map[int]SegmentOld)
			channel.signals = make(chan int, 100)
			i2.Channels[i3] = channel
		}
		tmp.Streams[i] = i2
	}
	return &tmp
}

//ClientDelete Delete Client
func (obj *StorageST) SaveConfig() error {
	log.WithFields(logrus.Fields{
		"module": "config",
		"func":   "NewStreamCore",
	}).Debugln("Saving configuration to", configFile)
	v2, err := version.NewVersion("2.0.0")
	if err != nil {
		return err
	}
	data, err := sheriff.Marshal(&sheriff.Options{
		Groups:     []string{"config"},
		ApiVersion: v2,
	}, obj)
	if err != nil {
		return err
	}
	res, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(configFile, res, 0644)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "config",
			"func":   "SaveConfig",
			"call":   "WriteFile",
		}).Errorln(err.Error())
		return err
	}
	return nil
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}