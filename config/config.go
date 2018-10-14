// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

// Config - P4dbeat config
type Config struct {
	Period    time.Duration `config:"period"`
	Path      string        `config:"path"`
	Statefile string        `config:"statefile"`
}

// DefaultConfig - default values for P4dbeat
var DefaultConfig = Config{
	Period:    1 * time.Second,
	Path:      "/p4/1/logs/log",
	Statefile: "./statefile",
}
