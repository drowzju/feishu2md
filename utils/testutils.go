package utils

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/joho/godotenv"
)

const projectDirName = "feishu2md-main"

// LoadEnv loads env vars from .env
func LoadEnv() {
	re := regexp.MustCompile(`^(.*` + projectDirName + `)`)
	cwd, _ := os.Getwd()
	rootPath := re.Find([]byte(cwd))

	err := godotenv.Load(string(rootPath) + `/.env`)
	if err != nil {
		log.Fatal("Can not load .env file" + string(rootPath) + err.Error())
		os.Exit(-1)
	}
}

func RootDir() string {
	_, b, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(b), "..")
	return root
}
