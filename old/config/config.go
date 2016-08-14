package config

import (
	"os"
	"path/filepath"
)

var (
	GOSUV_HOME           = os.ExpandEnv("$HOME/.gosuv")
	GOSUV_SOCK_PATH      = filepath.Join(GOSUV_HOME, "gosuv.sock")
	GOSUV_CONFIG         = filepath.Join(GOSUV_HOME, "gosuv.json")
	GOSUV_PROGRAM_CONFIG = filepath.Join(GOSUV_HOME, "programs.json")
	CMDPLUGIN_DIR        = filepath.Join(GOSUV_HOME, "cmdplugin")
)
