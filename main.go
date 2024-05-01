package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
  "encoding/csv"
)

type Config struct {
	IP         string
	Port       int
	BedCount   int
	SendAlarms bool
}

func (c *Config) Print() {
	fmt.Printf("Using the following settings: %s:%d, %d beds, SendAlarms = %t \n", c.IP, c.Port, c.BedCount, c.SendAlarms)
}

func LoadAlarms() [][]string {
  file, err := os.Open("Alarms/AlarmDict.csv")
  if err != nil {
    log.Fatalf("Error openning alarm CSV file: %s", err)
  }
  defer file.Close()

  reader := csv.NewReader(file)
  rows, err := reader.ReadAll()
  if err != nil {
    log.Fatalf("Error reading the alarm CSV file: %s", err)
  }

  var alarmPairs [][]string
  for _, row := range rows {
    if len(row) >= 2 {
      alarmPairs = append(alarmPairs, []string{row[0],row[1]})
    }
  }

  if len(alarmPairs) == 0 {
    log.Fatal("NO ALARMS FOUND IN CSV FILE!!!")
  }
  
  return alarmPairs 
}

type Bed struct {
	Unit          string
	Bed           string
	VitalWaveConn net.Conn
	AlarmConn     net.Conn
  ReconnectVitalWaveChan chan struct{}
  ReconnectAlarmChan chan struct{}
}

func (b *Bed) ReadVitalWaveAcks() {
  reader := make([]byte, 1024) 
  for {
    _, err :=  b.VitalWaveConn.Read(reader)
    if err != nil {
      // fmt.Println("Error reading ack, connection may have been lost.")
    }

    reader = make([]byte, 1024)
  }
}

func (b *Bed) ReadAlarmAcks() {
  for {
    reader := make([]byte, 1024)
    _, err := b.AlarmConn.Read(reader)
    if err != nil {
      // fmt.Println("Error reading ack, connection may have been lost.")
    }
  }
}

func (b *Bed) CloseConns() {
  if b.VitalWaveConn != nil {
    b.VitalWaveConn.Close()
  }

  if b.AlarmConn != nil {
    b.AlarmConn.Close()
  }
}

// Makes connection for Vitals and Wave sending then calls SendWaves and SendVitals
func (b *Bed) StartVitalWave(config *Config) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		log.Printf("Failed to make connection: %s", err)
    b.ReconnectVitalWaveChan <- struct{}{}
	}

	b.VitalWaveConn = conn

  go b.ReadVitalWaveAcks()
	go b.SendVitals()
  go b.SendWaves()

  go b.ReconnectVitalWave(config)
}

// Makes connection for Alarms and sends alarms
func (b *Bed) StartAlarm(config *Config) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		log.Printf("Failed to make connection: %s", err)
    b.ReconnectAlarmChan <- struct{}{}
	}

	b.AlarmConn = conn

  go b.ReadAlarmAcks()
	go b.SendAlarms()

  go b.ReconnectAlarm(config)
}

func (b *Bed) ReconnectVitalWave(config *Config) {
  for {
    time.Sleep(time.Minute)
    <- b.ReconnectVitalWaveChan
    for {
      attempts :=  1
      fmt.Println("Attempting to reconnect VitalWave feed...")
      conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
      if err != nil {
        if attempts > 5 {
          time.Sleep(time.Minute)
        } else {
          time.Sleep(time.Second * 30)
        }

        attempts ++
        continue
      }
      b.VitalWaveConn = conn
      fmt.Println("Successfully reconnected VitalWave feed!")
      break
    }
  }
}

func (b *Bed) ReconnectAlarm(config *Config) {
  for {
    <- b.ReconnectAlarmChan
    for {
      attempts := 1
      fmt.Println("Attempting to reconnect Alarm feed...")
      conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
      if err != nil {
        if attempts > 5 {
          time.Sleep(time.Minute)
        } else {
          time.Sleep(time.Second * 30)
        }

        attempts ++
        continue
      }
      b.AlarmConn = conn
      fmt.Println("Successfully reconnected Alarm feed!")
      break
    }
  }
}

func (b *Bed) SendWaves() {
	waveTemplateFile, err := os.ReadFile("Templates/WaveFormTemplate.txt")
	if err != nil {
		log.Fatalf("Error reading the WaveFormTemplate: %s", err)
	}

	waveTemplateString := string(waveTemplateFile)

	waveTemplate, err := template.New("vital").Parse(waveTemplateString)
	if err != nil {
		log.Fatalf("Error making wave template: %s", err)
	}

	data := struct {
		Unit         string
		Bed          string
		PatientID    string
		PatientLast  string
		PatientFirst string
    Datetime string
    DatetimeSub1 string
	}{
		PatientID:    b.Bed,
		PatientLast:  "L" + b.Bed,
		PatientFirst: "F" + b.Bed,
		Unit:         b.Unit,
		Bed:          b.Bed,
	}


  var filledTemplate strings.Builder

	for {
    now := time.Now()
    sub1 := now.Add(-1 * time.Second)
    data.Datetime = now.Format("20060102150405.0000-0700")
    data.DatetimeSub1 = sub1.Format("20060102150405.0000-0700")


		err = waveTemplate.Execute(&filledTemplate, data)
		if err != nil {
			log.Fatalf("Error filling in the wave template: %s", err)
		}

		mess := "\x0b" + filledTemplate.String() + "\x1C\x0D"
    _,err := b.VitalWaveConn.Write([]byte(mess))
    if err != nil {
      log.Println("Error writing wave message, connection may have been lost.")
      b.ReconnectVitalWaveChan <- struct{}{}
    }

    filledTemplate.Reset()
		time.Sleep(time.Second)
	}
}

func (b *Bed) SendVitals() {
	vitalTemplateFile, err := os.ReadFile("Templates/VitalTemplate.txt")
	if err != nil {
		log.Fatalf("Error reading the VitalTemplate: %s", err)
	}

	vitalTemplateString := string(vitalTemplateFile)

	vitalTemplate, err := template.New("vital").Parse(vitalTemplateString)
	if err != nil {
		log.Fatalf("Error making vital template: %s", err)
	}

	data := struct {
		Unit         string
		Bed          string
		PatientID    string
		PatientLast  string
		PatientFirst string
    Datetime string
    DatetimeSub1 string
	}{
		PatientID:    b.Bed,
		PatientLast:  "L" + b.Bed,
		PatientFirst: "F" + b.Bed,
		Unit:         b.Unit,
		Bed:          b.Bed,
	}


  var filledTemplate strings.Builder
	for {
    now := time.Now()
    sub1 := now.Add(-1 * time.Second)
    data.Datetime = now.Format("20060102150405.0000-0700")
    data.DatetimeSub1 = sub1.Format("20060102150405.0000-0700")

		err = vitalTemplate.Execute(&filledTemplate, data)
		if err != nil {
			log.Fatalf("Error filling in the vital template: %s", err)
		}

		mess := "\x0b" + filledTemplate.String() + "\x1C\x0D"
    _, err := b.VitalWaveConn.Write([]byte(mess))
    if err != nil {
      log.Println("Error writing the vital message, connection may have been lost.")
      b.ReconnectVitalWaveChan <- struct{}{}
    }
    filledTemplate.Reset()
		time.Sleep(time.Second)
	}
}

func (b *Bed) SendAlarms() {
	alarmTemplateFile, err := os.ReadFile("Templates/AlarmTemplate.txt")
	if err != nil {
		log.Fatalf("Error reading the AlarmStartTemplate: %s", err)
	}

	alarmTemplateString := string(alarmTemplateFile)

	alarmTemplate, err := template.New("start").Parse(alarmTemplateString)
	if err != nil {
		log.Fatalf("Error making alarm start template: %s", err)
	}

	data := struct {
		Unit         string
		Bed          string
		PatientID    string
		PatientLast  string
		PatientFirst string
		AlarmCode    string
		AlarmText    string
		AlarmLevel   string
		Start        string
		Active       string
	}{
		PatientID:    b.Bed,
		PatientLast:  "L" + b.Bed,
		PatientFirst: "F" + b.Bed,
		Unit:         b.Unit,
		Bed:          b.Bed,
	}

  //Wait a random number of min before sending the next start
  t := rand.Intn(10) + 1
  time.Sleep(time.Duration(t) * time.Minute)

	for {
		var filledTemplate strings.Builder

		//Func to get random alarm stuff
    alm := getRandomAlarm()
		data.AlarmCode = alm[1] 
		data.AlarmText = alm[0] 
		data.AlarmLevel = alm[2] 
		data.Start = "start"
		data.Active = "active"

		err = alarmTemplate.Execute(&filledTemplate, data)
		if err != nil {
			log.Fatalf("Error filling in the alarm start template: %s", err)
		}

		mess := "\x0b" + filledTemplate.String() + "\x1C\x0D"
    _, err := b.AlarmConn.Write([]byte(mess))
    if err != nil {
      log.Println("Error writing alarm message, connection may have been lost.")
      b.ReconnectAlarmChan <- struct{}{}
    }

    fmt.Printf("Sent alarm %s for %s _ %s \n", alm[0], b.Unit, b.Bed)
    filledTemplate.Reset()

		//Wait a random number of seconds before sending the end
		t := rand.Intn(60) + 1
		time.Sleep(time.Duration(t) * time.Second)

		data.Start = "end"
		data.Active = "inactive"

		err = alarmTemplate.Execute(&filledTemplate, data)
		if err != nil {
			log.Fatalf("Error filling in the alarm end template: %s", err)
		}

		mess = "\x0b" + filledTemplate.String() + "\x1C\x0D"
    _, err = b.AlarmConn.Write([]byte(mess))
    if err != nil {
      log.Printf("Error writing alarm message, connection may have been lost.")
      b.ReconnectAlarmChan <- struct{}{}
    }
    
    filledTemplate.Reset()
		//Wait a random number of min before sending the next start
		t = rand.Intn(30) + 1
		time.Sleep(time.Duration(t) * time.Minute)
	}
}

func getRandomAlarm() []string {
  var returnVal []string
  a := rand.Intn(len(alarms))
  returnVal = append(returnVal, alarms[a][0])
  returnVal = append(returnVal, alarms[a][1])

  randomAlarmLevel := rand.Intn(4) + 1

  crisis := "H~PH~SP"
  warning := "H~PM~SP"
  advisory := "H~PL~SP"
  system := "H~PM~ST"

  switch randomAlarmLevel {
  case 4:
    returnVal = append(returnVal, crisis)
  case 3:
    returnVal = append(returnVal, warning)
  case 2:
    returnVal = append(returnVal, advisory)
  default:
    returnVal = append(returnVal, system)
  }

  return returnVal
}

var alarms [][]string

func main() {
	var config Config

  baseConfig :=Config {
		IP : "127.0.0.1",
		Port : 9899,
		BedCount : 20,
		SendAlarms : true,
  }

  if ip := os.Getenv("IP"); ip != "" {
    config.IP = ip 
  } else {
    config.IP = baseConfig.IP
  }

  if port := os.Getenv("PORT"); port != "" {
    port, err := strconv.Atoi(port)
    if err != nil {
      log.Fatalf("Something broke trying to set the port %s", err)
    } else {
      config.Port = port
    }
  } else {
    config.Port = baseConfig.Port
  }

  if bedCountStr := os.Getenv("BED_COUNT"); bedCountStr != "" {
    bedCount, err := strconv.Atoi(bedCountStr)
    if err != nil {
      log.Fatalf("Something broke trying to set the bed count: %s", err)
    } else {
      config.BedCount = bedCount
    }
  } else {
    config.BedCount = baseConfig.BedCount 
  }
  
  if alarms := os.Getenv("SEND_ALARMS"); alarms != "" {
    sendAlarms, err := strconv.ParseBool(alarms)
    if err != nil {
      log.Fatalf("Something broke trying to set the sendAlarms flag: %s", err)
    } else {
      config.SendAlarms = sendAlarms
    }
  } else {
    config.SendAlarms = baseConfig.SendAlarms 
  }
		config.Print()

  if config.SendAlarms {
    alarms = LoadAlarms()
  }

  bedList := make([]Bed, 0, config.BedCount)

  for i := 0; i < config.BedCount; i++ {
    bed := Bed{
      Unit: "LABMR",
      Bed:  strconv.Itoa(i),
      ReconnectVitalWaveChan: make(chan struct{}),
    }

    bed.StartVitalWave(&config)
    if config.SendAlarms {
      bed.StartAlarm(&config)
      bed.ReconnectAlarmChan = make(chan struct{})
    }

    bedList = append(bedList, bed)

    if i % 5 == 0 && i != 0 {
      time.Sleep(time.Second)
    }

    if i % 25 == 0 && i != 0 {
      time.Sleep(time.Minute * 5)
    }

    time.Sleep(time.Millisecond * 50)
  }

  fmt.Println("All configured beds are now sending!")
  
  <-make(chan struct{})
  for _, bed := range bedList{
    bed.CloseConns()
  }
}
