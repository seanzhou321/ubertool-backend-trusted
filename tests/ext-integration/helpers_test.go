package extintegration

import (
	"flag"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "../../config/mail_config.test.yaml", "path to mail test config file")
}
