package main

import (
	"log"

	"github.com/veryCrunchy/aimmod-hub/api/run"
)

func main() {
	if err := run.Main(); err != nil {
		log.Fatal(err)
	}
}
