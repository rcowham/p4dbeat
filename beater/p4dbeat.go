package beater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/paths"
	"github.com/elastic/beats/v7/libbeat/statestore"
	"github.com/elastic/beats/v7/libbeat/statestore/backend/memlog"
	"github.com/sirupsen/logrus"

	"github.com/hpcloud/tail"
	p4dlog "github.com/rcowham/go-libp4dlog"
	"github.com/rcowham/p4dbeat/config"
)

const offsetKeyName = "offset"

// P4dbeat configuration.
type P4dbeat struct {
	done     chan struct{}
	name     string
	config   config.Config
	client   beat.Client
	lines    chan string
	events   chan string
	log      *logrus.Logger
	registry *statestore.Registry
}

// New creates an instance of p4dbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	log := logrus.New()

	memlog, err := memlog.New(
		logp.NewLogger("p4dbeat"),
		memlog.Settings{
			Root: paths.Resolve(paths.Data, c.StatePath),
			Checkpoint: func(filesize uint64) bool {
				const limit = 1024 * 256 // 256kb of transactions
				return filesize >= limit
			},
		})
	if err != nil {
		return nil, err
	}

	bt := &P4dbeat{
		done:     make(chan struct{}),
		lines:    make(chan string, 100),
		events:   make(chan string, 100),
		name:     b.Info.Name,
		config:   c,
		log:      log,
		registry: statestore.NewRegistry(memlog),
	}

	return bt, nil
}

// Run starts p4dbeat.
func (bt *P4dbeat) Run(b *beat.Beat) error {
	logp.Info("p4dbeat is running! Hit CTRL-C to stop it.")

	store, err := bt.registry.Get("p4dbeat")
	if err != nil {
		return err
	}
	defer store.Close()

	// Load the stored offset from the state registry so
	// we resume parsing the file where we left off.
	// Note: if a rotation happens in between we will resume at
	// the wrong place, we need to store the inode value to detect that
	offsetData := common.MapStr{}
	var offset int64
	err = store.Get(offsetKeyName, &offsetData)
	if err != nil {
		bt.log.Warnf("No file offset state found, resuming from the start: %v", err)
	} else if offsetData["offset"] != "" {
		offsetVal, ok := offsetData["offset"].(float64) // all json values are float64
		if !ok {
			bt.log.Warnf("Invalid file offset: '%s': %v", offsetData["offset"], err)
		} else {
			offset = int64(offsetVal)
			bt.log.Infof("Starting at offset %d bytes", offset)
		}
	}

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
		Location: &tail.SeekInfo{
			Offset: offset,
			Whence: os.SEEK_SET,
		},
	}

	go bt.tailFile(bt.config.Path, tailFileconfig, tailFileDone, bt.done, store)

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

func setIfNonZero(event *beat.Event, fieldName string, value int64) {
	if value > 0 {
		event.Fields[fmt.Sprintf("p4.%s", fieldName)] = value
	}
}

func setIfNonZeroSec(event *beat.Event, fieldName string, value float32) {
	if value > 0 {
		event.Fields[fmt.Sprintf("p4.%s", fieldName)] = value
	}
}

func setTblIfNonZero(event *beat.Event, tableName string, fieldName string, value int64) {
	if value > 0 {
		event.Fields[fmt.Sprintf("p4.tbl.%s.%s", strings.ToLower(tableName), fieldName)] = value
	}
}

// setIfNonZeroMs records the value in the event if it's non zero, converting from integer ms values to float seconds
func setTblIfNonZeroMs(event *beat.Event, tableName string, fieldName string, valueMS int64) {
	if valueMS > 0 {
		event.Fields[fmt.Sprintf("p4.tbl.%s.%s", strings.ToLower(tableName), fieldName)] = float64(valueMS) / 1000.0
	}
}

func (bt *P4dbeat) publishCommand(command p4dlog.Command) {
	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"type":                bt.name,
			"p4.process_key":      command.ProcessKey,
			"p4.cmd":              command.Cmd,
			"p4.pid":              command.Pid,
			"p4.line_no":          command.LineNo,
			"p4.user":             command.User,
			"p4.workspace":        command.Workspace,
			"p4.start_time":       command.StartTime,
			"p4.end_time":         command.EndTime,
			"p4.compute_sec":      command.ComputeLapse,
			"p4.completed_sec":    command.CompletedLapse,
			"p4.app":              command.App,
			"p4.args":             command.Args,
			"p4.running":          command.Running,
			"p4.cpu.user":         command.UCpu,
			"p4.cpu.system":       command.SCpu,
			"p4.disk.read_bytes":  command.DiskIn * 512,
			"p4.disk.write_bytes": command.DiskOut * 512,
			"p4.max_rss":          command.MaxRss,
			"p4.page_faults":      command.PageFaults,
			"p4.cmd_error":        command.CmdError,
		},
	}

	// Only include the IPC/RPC info if it's non zero
	setIfNonZero(&event, "ipc.in", command.IpcIn)
	setIfNonZero(&event, "ipc.out", command.IpcOut)
	setIfNonZero(&event, "rpc.msgs.in", command.RPCMsgsIn)
	setIfNonZero(&event, "rpc.msgs.out", command.RPCMsgsOut)
	setIfNonZero(&event, "rpc.size.in", command.RPCSizeIn)
	setIfNonZero(&event, "rpc.size.out", command.RPCSizeOut)
	setIfNonZero(&event, "rpc.himark.fwd", command.RPCHimarkFwd)
	setIfNonZero(&event, "rpc.himark.rev", command.RPCHimarkRev)
	setIfNonZeroSec(&event, "rpc.snd_sec", command.RPCSnd)
	setIfNonZeroSec(&event, "rpc.rcv_sec", command.RPCRcv)

	ips := strings.Split(command.IP, "/")
	if len(ips) == 1 {
		if ips[0] != "background" && ips[0] != "" {
			event.Fields["p4.ip"] = ips[0]
		}
	} else if len(ips) > 1 {
		if ips[0] != "" {
			event.Fields["p4.proxy_ip"] = ips[0]
		}
		if ips[1] != "" {
			event.Fields["p4.ip"] = ips[1]
		}
	}

	for _, values := range command.Tables {
		// note: these do not exist in fields.yml but will be auto-discovered as numbers
		setTblIfNonZero(&event, values.TableName, "pages.in", values.PagesIn)
		setTblIfNonZero(&event, values.TableName, "pages.out", values.PagesOut)
		setTblIfNonZero(&event, values.TableName, "pages.cached", values.PagesCached)
		setTblIfNonZero(&event, values.TableName, "pages.split_internal", values.PagesSplitInternal)
		setTblIfNonZero(&event, values.TableName, "pages.split_leaf", values.PagesSplitLeaf)
		setTblIfNonZeroMs(&event, values.TableName, "locks.read.total_sec", values.ReadLocks)
		setTblIfNonZeroMs(&event, values.TableName, "locks.read.wait.total_sec", values.TotalReadWait)
		setTblIfNonZeroMs(&event, values.TableName, "locks.read.wait.max_sec", values.MaxReadWait)
		setTblIfNonZeroMs(&event, values.TableName, "locks.read.held.total_sec", values.TotalReadHeld)
		setTblIfNonZeroMs(&event, values.TableName, "locks.read.held.max_sec", values.MaxReadHeld)
		setTblIfNonZeroMs(&event, values.TableName, "locks.write.total_sec", values.WriteLocks)
		setTblIfNonZeroMs(&event, values.TableName, "locks.write.wait.total_sec", values.TotalWriteWait)
		setTblIfNonZeroMs(&event, values.TableName, "locks.write.wait.max_sec", values.MaxWriteWait)
		setTblIfNonZeroMs(&event, values.TableName, "locks.write.held.total_sec", values.TotalWriteHeld)
		setTblIfNonZeroMs(&event, values.TableName, "locks.write.held.max_sec", values.MaxWriteHeld)
		setTblIfNonZero(&event, values.TableName, "rows.get", values.GetRows)
		setTblIfNonZero(&event, values.TableName, "rows.pos", values.PosRows)
		setTblIfNonZero(&event, values.TableName, "rows.scan", values.ScanRows)
		setTblIfNonZero(&event, values.TableName, "rows.put", values.PutRows)
		setTblIfNonZero(&event, values.TableName, "rows.del", values.DelRows)
		setTblIfNonZero(&event, values.TableName, "peek.count", values.PeekCount)
		setTblIfNonZeroMs(&event, values.TableName, "peek.wait.total_sec", values.TotalPeekWait)
		setTblIfNonZeroMs(&event, values.TableName, "peek.wait.max_sec", values.MaxPeekWait)
		setTblIfNonZeroMs(&event, values.TableName, "peek.held.total_sec", values.TotalPeekHeld)
		setTblIfNonZeroMs(&event, values.TableName, "peek.held.max_sec", values.MaxPeekHeld)
	}

	bt.client.Publish(event)
}

func (bt *P4dbeat) publishEvent(str string) {
	var f interface{}
	err := json.Unmarshal([]byte(str), &f)
	if err != nil {
		bt.log.Warnf("Error %v to unmarshal %s", err, str)
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

func (bt *P4dbeat) tailFile(filename string, config tail.Config, done chan struct{}, stop chan struct{}, store *statestore.Store) {
	ctx := context.Background()

	defer func() {
		done <- struct{}{}
	}()

	t, err := tail.TailFile(filename, config)
	if err != nil {
		logp.Err("Start tail file failed, err: %v", err)
		return
	}

	fp := p4dlog.NewP4dFileParser(bt.log)
	commands := fp.LogParser(ctx, bt.lines, nil)

	bt.log.Infof("Log parser is now tailing '%s'", filename)
	for {
		select {
		case <-stop:
			bt.log.Debug("Stopping\n", "")
			close(bt.lines)
			bt.processEvents()
			t.Stop()
			return
		case line := <-t.Lines:
			bt.log.Debugf("Parsing line:\n%s", line.Text)
			bt.lines <- line.Text

		case command := <-commands:
			bt.log.Debugf("Publishing '%s' command", command.Cmd)
			bt.publishCommand(command)
			// update the offset for every parsed command
			offset, err := t.Tell()
			if err != nil {
				bt.log.Errorf("Failed to get current log offset")
			} else {
				store.Set(offsetKeyName, common.MapStr{"offset": offset})
			}
		case json := <-bt.events:
			bt.publishEvent(json)
		}
	}

	// if err = t.Wait(); err != nil {
	// 	logp.Err("Tail file blocking goroutine stopped, err: %v", err)
	// }
}

// Stop stops p4dbeat.
func (bt *P4dbeat) Stop() {
	bt.client.Close()
	bt.registry.Close()
	close(bt.done)
}
