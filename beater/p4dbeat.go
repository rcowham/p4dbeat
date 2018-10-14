package beater

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/hpcloud/tail"
	p4dlog "github.com/rcowham/go-libp4dlog"
	"github.com/rcowham/p4dbeat/config"
)

// P4dbeat configuration.
type P4dbeat struct {
	done   chan struct{}
	name   string
	config config.Config
	client beat.Client
	lines  chan []byte
	events chan string
}

// New creates an instance of p4dbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &P4dbeat{
		done:   make(chan struct{}),
		lines:  make(chan []byte, 100),
		events: make(chan string, 100),
		name:   b.Info.Name,
		config: c,
	}

	return bt, nil
}

// Run starts p4dbeat.
func (bt *P4dbeat) Run(b *beat.Beat) error {
	logp.Info("p4dbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	logp.Debug("Processing log file: %s\n", bt.config.Path)

	tailFileDone := make(chan struct{})
	tailFileconfig := tail.Config{
		ReOpen:      true,
		MustExist:   false,
		Poll:        false,
		Follow:      true,
		MaxLineSize: 0,
	}

	go bt.tailFile(bt.config.Path, tailFileconfig, tailFileDone, bt.done)

	<-tailFileDone

	return nil
}

// ticker := time.NewTicker(bt.config.Period)
// counter := 1
// for {
// 	select {
// 	case <-bt.done:
// 		return nil
// 	case <-ticker.C:
// 	}

func (bt *P4dbeat) publishEvent(str string) {
	var f interface{}
	err := json.Unmarshal([]byte(str), &f)
	if err != nil {
		logp.Warn("Error %v to unmarshal %s", err, str)
	}
	m := f.(map[string]interface{})
	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"type":             bt.name,
			"p4.cmd":           m["cmd"],
			"p4.user":          m["user"],
			"p4.workspace":     m["workspace"],
			"p4.ip":            m["ip"],
			"p4.args":          m["args"],
			"p4.start_time":    m["startTime"],
			"p4.end_time":      m["endTime"],
			"p4.compute_sec":   m["computeLapse"],
			"p4.completed_sec": m["completedLapse"],
		},
	}
	bt.client.Publish(event)
}

func (bt *P4dbeat) processEvents() {
	for {
		select {
		case json := <-bt.events:
			bt.publishEvent(json)
		default:
			return
		}
	}
}

func (bt *P4dbeat) tailFile(filename string, config tail.Config, done chan struct{}, stop chan struct{}) {
	defer func() {
		done <- struct{}{}
	}()

	t, err := tail.TailFile(filename, config)
	if err != nil {
		logp.Err("Start tail file failed, err: %v", err)
		return
	}

	fp := p4dlog.NewP4dFileParser()
	go fp.LogParser(bt.lines, bt.events)

	for {
		select {
		case <-stop:
			logp.Debug("Stopping\n", "")
			close(bt.lines)
			bt.processEvents()
			t.Stop()
			return
		case line := <-t.Lines:
			logp.Debug("Parsing line:\n%s", line.Text)
			buf := []byte(line.Text)
			bt.lines <- buf
		case json := <-bt.events:
			bt.publishEvent(json)
			// default:
		}
	}

	// if err = t.Wait(); err != nil {
	// 	logp.Err("Tail file blocking goroutine stopped, err: %v", err)
	// }
}

// Stop stops p4dbeat.
func (bt *P4dbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
