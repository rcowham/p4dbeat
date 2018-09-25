package beater

import (
	"fmt"
	"psla/p4dlogparse"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/hpcloud/tail"
	"github.com/rcowham/p4dbeat/config"
)

// P4dbeat configuration.
type P4dbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

// New creates an instance of p4dbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &P4dbeat{
		done:   make(chan struct{}),
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

type parseResult struct {
	bt       *P4dbeat
	callback p4dlogparse.P4dOutputCallback
}

func newResult(bt *P4dbeat) *parseResult {
	var pr parseResult
	pr.bt = bt
	pr.callback = func(output string) {
		event := beat.Event{
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"type": bt.Info.Name,
				"cmd":  output,
			},
		}
		bt.client.Publish(event)
		logp.Info("Event sent")
	}
	return &pr
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

	presult := newResult(bt)

	for line := range t.Lines {
		select {
		case <-stop:
			t.Stop()
			return
		default:
		}

		// event := make(common.MapStr)
		// if err = json.Unmarshal([]byte(line.Text), &event); err != nil {
		// 	logp.Err("Unmarshal json log failed, err: %v", err)
		// 	continue
		// }
		// if logTime, err := time.Parse("2017-03-13T07:13:30.172Z", event["@timestamp"].(string)); err != nil {
		// 	event["@timestamp"] = common.Time(logTime)
		// } else {
		// 	logp.Err("Unmarshal json log @timestamp failed, time string: %v", event["@timestamp"].(string))
		// 	event["@timestamp"] = common.Time(time.Now())
		// }
		// bt.client.PublishEvent(event)
		// logp.Info("Event sent")
	}

	if err = t.Wait(); err != nil {
		logp.Err("Tail file blocking goroutine stopped, err: %v", err)
	}
}

// 		// P4LogParseFile - interface for parsing a specified file
// func (fp *P4dFileParser) P4LogParseFile(opts P4dParseOptions) {
// 	var scanner *bufio.Scanner
// 	if len(opts.testInput) > 0 {
// 		scanner = bufio.NewScanner(strings.NewReader(opts.testInput))
// 	} else if opts.File == "-" {
// 		scanner = bufio.NewScanner(os.Stdin)
// 	} else {
// 		file, err := os.Open(opts.File)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer file.Close()
// 		reader := bufio.NewReaderSize(file, 1024*1024) // Read in chunks
// 		scanner = bufio.NewScanner(reader)
// 	}
// 	fp.lineNo = 0
// 	for scanner.Scan() {
// 		line := scanner.Bytes()
// 		fp.P4LogParseLine(line)
// 	}
// 	fp.P4LogParseFinish()
// 	if err := scanner.Err(); err != nil {
// 		fmt.Fprintf(os.Stderr, "reading file %s:%s\n", opts.File, err)
// 	}

// }

// Stop stops p4dbeat.
func (bt *P4dbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
