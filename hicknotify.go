package main

import "fmt"
import "net/http"
import "bufio"
import "sync"
import "bytes"
import "regexp"
import "strconv"
import "time"
import "encoding/json"
import "os"
import "gopkg.in/gomail.v2"
import "os/exec"

type Event struct {
  etype string
  estate string
  ecount int
  camera *Camera
}

func (e *Event) Complete() bool {
  if len(e.etype) > 0 && len(e.estate) > 0 && e.ecount > 0 {
    return true
  }
  return false
}
func (e *Event) Reset() {
  *e = Event{}
}

func GenerateTimeout(config Config, camera *Camera, lc chan bool, ec chan Event) {
  count := 1
  for {
    select {
      case <-lc:
        break;

      case <-time.After(time.Second * config.WatchdogTime):
        ec <- Event{"watchdog", "lost-signal", count, camera}
        count += 1
        break

    }
  }
}
func GeneratePresence(wg *sync.WaitGroup, config Config, host string, hc chan bool) {
  defer wg.Done()

  for {
    cmd := exec.Command("ping", "-c", "1", host)
    err := cmd.Run()
    if err == nil {
      hc <- true
    } else {
      fmt.Println("PING FAILED", host, err)
    }

    time.Sleep(config.PingInterval * time.Second)
  }
}

func GenerateEvents(wg *sync.WaitGroup, config Config, camera Camera, ec chan Event) {
  var last_attempt time.Time

  lc := make(chan bool)
  event := Event{camera: &camera}
  defer wg.Done()

  go GenerateTimeout(config, &camera, lc, ec)

  for {
    if time.Now().Before(last_attempt.Add(time.Second * config.ErrorRetryTime)) {
      fmt.Println("SLEEPING FOR SECONDS", config.ErrorRetryTime)
      time.Sleep(config.ErrorRetryTime * time.Second)
    }
    last_attempt = time.Now()

    client := &http.Client{}
    // This will break the stream of responses, the response has
    // to complete within 10 seconds.
    // client.Timeout = time.Second * 10

    etyper := regexp.MustCompile("eventType>(.*)</eventType")
    estater := regexp.MustCompile("eventState>(.*)</eventState")
    ecountr := regexp.MustCompile("activePostCount>(.*)</activePostCount")

    request, err := http.NewRequest("GET", camera.Url, nil)
    if err != nil {
      fmt.Println("REQUEST ERROR", camera, err)
      continue
    }

    request.SetBasicAuth(config.Username, config.Password)

    resp, err := client.Do(request)
    if err != nil {
      fmt.Println("DO ERROR", camera, err)
      continue
    }

    reader := bufio.NewReader(resp.Body)
    for {
      line, err := reader.ReadBytes('\n')
      if err != nil {
        fmt.Println("READER ERROR", camera, err)
        break
      }
      // Send keepalive
      lc <- true
      //Uncomment this line to dump all notifications
      //fmt.Println(string(line))

      line = bytes.TrimRight(line, "\n\r")
      etypes := etyper.FindSubmatch(line)
      if len(etypes) > 0 {
        event.etype = string(etypes[1])
      }
      estates := estater.FindSubmatch(line)
      if len(estates) > 0 {
        event.estate = string(estates[1])
      }
      ecounts := ecountr.FindSubmatch(line)
      if len(ecounts) > 0 {
        count, _ := strconv.Atoi(string(ecounts[1]))
        event.ecount = count + 1
      }
      if event.Complete() {
        if event.etype != "videoloss" {
          ec <- event
        }
        event.Reset()
        event.camera = &camera
      }
    }
  }
}

type Camera struct {
  Url string
  Name string
}

type Config struct {
  Cameras []Camera
  Hosts []string

  Username string
  Password string

  DampeningTime time.Duration
  ErrorRetryTime time.Duration
  WatchdogTime time.Duration

  MailFrom string
  MailTo []string

  MailServer string
  MailPort int
  MailUser string
  MailPassword string

  PingInterval time.Duration
  PingDisable time.Duration
}

func LoadConfig() (Config, error) {
  file, err := os.Open("config.json")
  if err != nil {
    return Config{}, err
  }
  decoder := json.NewDecoder(file)
  configuration := Config{}
  err = decoder.Decode(&configuration)
  if err != nil {
    return Config{}, err
  }

  if configuration.DampeningTime <= 0 {
    configuration.DampeningTime = 10
  }
  if configuration.ErrorRetryTime <= 0 {
    configuration.ErrorRetryTime = 5
  }
  if configuration.WatchdogTime <= 0 {
    configuration.WatchdogTime = 5
  }
  if configuration.PingInterval <= 0 {
    configuration.PingInterval = 1
  }
  if configuration.PingDisable <= 0 {
    configuration.PingDisable = 600
  }
  return configuration, nil
}

func SendMail(config Config, event Event, now time.Time) {
  fmt.Println("SENDING NOTIFICATION FOR", event, event.camera.Name, now)
  m := gomail.NewMessage()
  m.SetHeader("From", config.MailFrom)
  m.SetHeader("To", config.MailTo...)
  m.SetHeader("Subject", fmt.Sprintf("[NVR] Event in '%s'", event.camera.Name))
  m.SetBody("text/html", fmt.Sprintf("Event: %s<br>State: %s<br>Count: %d<br>At: %s<br>In: %s<br>Url: %s",
                                     event.etype, event.estate, event.ecount, now, event.camera.Name, event.camera.Url))

  d := gomail.NewDialer(config.MailServer, config.MailPort, config.MailUser, config.MailPassword)
  err := d.DialAndSend(m)
  if err != nil {
    fmt.Println("ERROR SENDING NOTIFICATION", err)
  }
}

func main() {
  config, err := LoadConfig()
  if err != nil {
    fmt.Println("COULD NOT READ CONFIG", err)
    return
  }

  hc := make(chan bool)
  ec := make(chan Event)
  wg := &sync.WaitGroup{}
  for _, camera := range config.Cameras {
    fmt.Println("MONITORING CAMERA", camera)

    wg.Add(1)
    go GenerateEvents(wg, config, camera, ec)
  }

  for _, host := range config.Hosts {
    fmt.Println("MONITORING HOST", host)
    wg.Add(1)
    go GeneratePresence(wg, config, host, hc)
  }

  type DampKey struct {
    event string
    camera *Camera
  }

  var hostsilence time.Time
  dampener := make(map[DampKey]time.Time)
  for {
    select {
      case <-hc:
        //fmt.Println("HOST DETECTED", hostsilence)
        hostsilence = time.Now()
        break;

      case event := <-ec:
        key := DampKey{event.etype, event.camera}
        last, ok := dampener[key]
        now := time.Now()
        if now.After(hostsilence.Add(config.PingDisable * time.Second)) && (!ok || now.After(last.Add(config.DampeningTime * time.Second))) {
          SendMail(config, event, now)
          dampener[key] = now
        } else {
          // fmt.Println("TIME DAMPENED ", event, key)
          dampener[key] = time.Now()
        }
        break;
    }
  }

  wg.Wait()
}
