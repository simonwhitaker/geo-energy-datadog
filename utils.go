package main

import "os"

func getEnvOrDefault(key, defaultValue string) string {
	result, ok := os.LookupEnv(key)
	if ok {
		return result
	}
	return defaultValue
}
