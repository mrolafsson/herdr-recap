package main

import (
	"bytes"
	"os"
	"strconv"
)

// readProcessEnv reads /proc/<pid>/environ: readable for your own processes.
func readProcessEnv(pid int) (map[string]string, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, kv := range bytes.Split(data, []byte{0}) {
		if k, v, ok := bytes.Cut(kv, []byte("=")); ok {
			env[string(k)] = string(v)
		}
	}
	return env, nil
}
