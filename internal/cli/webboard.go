package cli

import (
	"fmt"
	"strconv"
)

func boardOptions(args []string) (int, map[string]bool, error) {
	pos, flags, set, err := parse(args, map[string]bool{"json": true, "web": true, "no-open": true}, map[string]bool{"port": true})
	if err != nil {
		return 0, nil, err
	}
	if len(pos) != 0 {
		return 0, nil, fmt.Errorf("unexpected board argument")
	}
	_, hasPort := flags["port"]
	if set["web"] && set["json"] {
		return 0, nil, fmt.Errorf("--web and --json are mutually exclusive")
	}
	if !set["web"] && (set["no-open"] || hasPort) {
		return 0, nil, fmt.Errorf("--no-open and --port require --web")
	}
	port := 0
	if hasPort {
		port, err = strconv.Atoi(flags["port"])
		if err != nil || port < 0 || port > 65535 {
			return 0, nil, fmt.Errorf("--port must be an integer from 0 through 65535")
		}
	}
	return port, set, nil
}
